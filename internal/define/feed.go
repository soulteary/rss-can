package define

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

// RuleSource is an immutable copy of the script used to evaluate a configuration.
// Private string storage prevents callers from changing a published rule's bytes.
type RuleSource struct {
	body string
	hash [sha256.Size]byte
	set  bool
}

func NewRuleSource(body []byte) RuleSource {
	return RuleSource{body: string(body), hash: sha256.Sum256(body), set: true}
}

func (s RuleSource) Present() bool           { return s.set }
func (s RuleSource) Text() string            { return s.body }
func (s RuleSource) Hash() [sha256.Size]byte { return s.hash }

const (
	FEED_TYPE_RSS  = "rss"
	FEED_TYPE_ATOM = "atom"
	FEED_TYPE_JSON = "json"
)

const (
	FEED_MIME_TYPE_RSS     = "application/rss+xml"
	FEED_MIME_TYPE_ATOM    = "application/atom+xml"
	FEED_MIME_TYPE_JSON    = "application/feed+json"
	FEED_MIME_TYPE_DEFAULT = "text/plain"
)

type InfoItem struct {
	ID          string `json:"id,omitempty"`
	Title       string `json:"title"`
	Link        string `json:"link"`
	Date        string `json:"date"`
	Author      string `json:"author,omitempty"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content,omitempty"`
}

const (
	ConfigHookReadLink = "readlink"
)

type ConfigHook struct {
	Action string `json:"action"`
	Object string `json:"object"`
	URL    string `json:"URL"`
}

type PROP_ID struct {
	Object string `json:"object"`
	Prop   string `json:"prop"`
}

type JavaScriptConfig struct {
	RuleSource   RuleSource `json:"-"`
	Name         string     `json:"name"`
	URL          string     `json:"URL"`
	Mode         string     `json:"Mode"`
	File         string     //private field
	Charset      string     `json:"Charset"`
	Expire       int        `json:"Expire"`
	Headless     string     `json:"Headless"`
	Timeout      int        `json:"Timeout"`
	Proxy        string     `json:"Proxy"`
	DisableCache bool       `json:"DisableCache"`
	IdByRegexp   string     `json:"IdByRegexp"`
	IdByProp     PROP_ID    `json:"IdByProp"`
	Cookies      string     `json:"Cookies"`

	ListContainer string `json:"ListContainer"`
	// ReadySelector optionally separates browser page readiness from data scope.
	ReadySelector string `json:"ReadySelector,omitempty"`
	Title         string `json:"Title"`
	Author        string `json:"Author"`
	Link          string `json:"Link"`

	DateTime     string     `json:"DateTime"`
	DateTimeHook ConfigHook `json:"DateTimeHook"`

	Category     string     `json:"Category"`
	CategoryHook ConfigHook `json:"CategoryHook"`

	Description     string     `json:"Description"`
	DescriptionHook ConfigHook `json:"DescriptionHook"`

	Content     string     `json:"Content"`
	ContentHook ConfigHook `json:"ContentHook"`

	Pager      string `json:"Pager"`
	PagerLimit int    `json:"PagerLimit"`
}

// CompileIDRegexp validates the capture used as the entry ID before parsing.
func (c JavaScriptConfig) CompileIDRegexp() (*regexp.Regexp, error) {
	pattern := strings.TrimSpace(c.IdByRegexp)
	if pattern == "" {
		return nil, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid IdByRegexp: %w", err)
	}
	if re.NumSubexp() == 0 {
		return nil, fmt.Errorf("IdByRegexp must contain a capture group")
	}
	return re, nil
}
