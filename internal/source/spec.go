// Package source defines the non-executable source format exposed to users.
package source

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/cascadia"
)

var ErrNoMatch = errors.New("selectors matched no items")
var ErrSelector = errors.New("only simple tag, class and ID selectors are supported")
var simpleSelector = regexp.MustCompile(`^(?:[A-Za-z][A-Za-z0-9-]{0,63})?(?:[.#][A-Za-z_][A-Za-z0-9_-]{0,63}){0,4}$`)

// Spec contains selectors and values only. It intentionally has no JavaScript,
// headers, cookies, proxy, browser endpoint or arbitrary hook fields.
type Spec struct {
	Name                string `json:"name"`
	URL                 string `json:"url"`
	ItemSelector        string `json:"item_selector"`
	TitleSelector       string `json:"title_selector"`
	LinkSelector        string `json:"link_selector"`
	DateSelector        string `json:"date_selector,omitempty"`
	DescriptionSelector string `json:"description_selector,omitempty"`
	PagerSelector       string `json:"pager_selector,omitempty"`
	MaxPages            int    `json:"max_pages,omitempty"`
	RefreshSeconds      int    `json:"refresh_seconds,omitempty"`
}

func (s Spec) Normalized() Spec {
	s.Name = strings.TrimSpace(s.Name)
	s.URL = strings.TrimSpace(s.URL)
	s.ItemSelector = strings.TrimSpace(s.ItemSelector)
	s.TitleSelector = strings.TrimSpace(s.TitleSelector)
	s.LinkSelector = strings.TrimSpace(s.LinkSelector)
	s.DateSelector = strings.TrimSpace(s.DateSelector)
	s.DescriptionSelector = strings.TrimSpace(s.DescriptionSelector)
	s.PagerSelector = strings.TrimSpace(s.PagerSelector)
	if s.MaxPages == 0 {
		s.MaxPages = 1
	}
	if s.RefreshSeconds == 0 {
		s.RefreshSeconds = 600
	}
	return s.Clone()
}

func (s Spec) Clone() Spec {
	s.Name = strings.Clone(s.Name)
	s.URL = strings.Clone(s.URL)
	s.ItemSelector = strings.Clone(s.ItemSelector)
	s.TitleSelector = strings.Clone(s.TitleSelector)
	s.LinkSelector = strings.Clone(s.LinkSelector)
	s.DateSelector = strings.Clone(s.DateSelector)
	s.DescriptionSelector = strings.Clone(s.DescriptionSelector)
	s.PagerSelector = strings.Clone(s.PagerSelector)
	return s
}

func (s Spec) Validate() error {
	if strings.TrimSpace(s.Name) == "" || len(s.Name) > 256 {
		return errors.New("name must contain 1–256 bytes")
	}
	u, err := url.Parse(s.URL)
	if err != nil || len(s.URL) > 8192 || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Fragment != "" {
		return errors.New("enter an HTTP(S) URL without credentials or fragment")
	}
	if s.MaxPages < 1 || s.MaxPages > 20 {
		return errors.New("max_pages must be between 1 and 20")
	}
	if s.RefreshSeconds < 60 || s.RefreshSeconds > 86400 {
		return errors.New("refresh_seconds must be between 60 and 86400")
	}
	for _, field := range []struct {
		name, value string
		required    bool
	}{{"item_selector", s.ItemSelector, true}, {"title_selector", s.TitleSelector, true}, {"link_selector", s.LinkSelector, true}, {"date_selector", s.DateSelector, false}, {"description_selector", s.DescriptionSelector, false}, {"pager_selector", s.PagerSelector, false}} {
		if field.value == "" && !field.required {
			continue
		}
		if field.value == "" || !simpleSelector.MatchString(field.value) {
			return fmt.Errorf("%w: %s", ErrSelector, field.name)
		}
		if _, err := cascadia.Compile(field.value); err != nil {
			return errors.New("invalid CSS selector: " + field.name)
		}
	}
	return nil
}

func (s Spec) Config() (define.JavaScriptConfig, error) {
	s = s.Normalized()
	if err := s.Validate(); err != nil {
		return define.JavaScriptConfig{}, err
	}
	c := define.JavaScriptConfig{TextOnly: true, Name: s.Name, URL: s.URL, Mode: define.PARSE_MODE_SSR, ListContainer: s.ItemSelector, Title: "css:" + s.TitleSelector, Link: "css:" + s.LinkSelector, Expire: s.RefreshSeconds, Pager: s.PagerSelector, PagerLimit: s.MaxPages}
	if s.DateSelector != "" {
		c.DateTime = "css:" + s.DateSelector
	}
	if s.DescriptionSelector != "" {
		c.Description = "css:" + s.DescriptionSelector
	}
	return c, nil
}

func (s Spec) Bytes() ([]byte, error) {
	s = s.Normalized()
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(s)
}
func Parse(body []byte) (Spec, error) {
	var s Spec
	d := json.NewDecoder(strings.NewReader(string(body)))
	d.DisallowUnknownFields()
	if err := d.Decode(&s); err != nil {
		return Spec{}, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return Spec{}, errors.New("unexpected trailing JSON")
	}
	s = s.Normalized()
	return s, s.Validate()
}

func ValidateItems(items []define.InfoItem) error {
	if len(items) == 0 {
		return ErrNoMatch
	}
	for _, item := range items {
		u, err := url.Parse(item.Link)
		if strings.TrimSpace(item.Title) == "" || err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return ErrNoMatch
		}
	}
	return nil
}
