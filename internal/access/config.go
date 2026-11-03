package access

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config uses the verified Stargate X-Auth-User stable subject, never the
// X-Forwarded-User authentication marker or user-supplied role names.
type Config struct {
	StorePath      string
	PublicOrigin   string
	TrustedProxies []netip.Prefix
	WardenURL      string
	WardenAPIKey   string
}

func FromEnv() (*Config, error) {
	mode := os.Getenv("RSS_ACCESS_MODE")
	if mode == "" || mode == "standalone" {
		return nil, nil
	}
	if mode != "team" {
		return nil, errors.New("RSS_ACCESS_MODE must be standalone or team")
	}
	c := &Config{StorePath: os.Getenv("RSS_ACCESS_STORE"), PublicOrigin: os.Getenv("RSS_PUBLIC_ORIGIN"), WardenURL: os.Getenv("RSS_WARDEN_URL"), WardenAPIKey: os.Getenv("RSS_WARDEN_API_KEY")}
	for _, raw := range strings.Split(os.Getenv("RSS_TRUSTED_PROXIES"), ",") {
		p, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return nil, errors.New("RSS_TRUSTED_PROXIES requires explicit CIDR ranges")
		}
		c.TrustedProxies = append(c.TrustedProxies, p)
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	if c.StorePath == "" || len(c.TrustedProxies) == 0 {
		return errors.New("team mode requires access store and trusted proxy CIDRs")
	}
	u, err := url.Parse(c.PublicOrigin)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("RSS_PUBLIC_ORIGIN must be an HTTPS origin without a path")
	}
	for _, p := range c.TrustedProxies {
		if !p.IsValid() || p.Bits() == 0 {
			return errors.New("trusted proxy ranges must not trust every address")
		}
	}
	w, err := url.Parse(c.WardenURL)
	if err != nil || w.Hostname() == "" || w.User != nil || w.RawQuery != "" || w.Fragment != "" || (w.Scheme != "https" && w.Scheme != "http") || c.WardenAPIKey == "" {
		return errors.New("team mode requires Warden URL and API key for account-state checks")
	}
	return nil
}

func (c Config) Trusted(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	for _, p := range c.TrustedProxies {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

type Checker interface {
	Active(context.Context, string) error
}

type WardenChecker struct {
	URL, APIKey string
	Client      *http.Client
}

// Active consults the authoritative directory on every private request. It
// intentionally has no availability fallback and does not follow redirects.
func (w WardenChecker) Active(ctx context.Context, subject string) error {
	if !validID(subject) {
		return ErrDenied
	}
	u, err := url.Parse(strings.TrimRight(w.URL, "/") + "/user")
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("user_id", subject)
	u.RawQuery = q.Encode()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+w.APIKey)
	req.Header.Set("Accept", "application/json")
	client := w.Client
	if client == nil {
		// Identity credentials belong only to the configured directory, not to
		// an ambient HTTP_PROXY inherited from the feed-fetching environment.
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		client = &http.Client{Timeout: 2 * time.Second, Transport: transport}
		defer transport.CloseIdleConnections()
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := copyClient.Do(req)
	if err != nil {
		return fmt.Errorf("directory unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 || resp.StatusCode == 401 || resp.StatusCode == 403 {
		return ErrDenied
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("directory unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil || len(body) > 64<<10 {
		return errors.New("invalid directory response")
	}
	var user struct {
		UserID string `json:"user_id"`
		Status string `json:"status"`
	}
	if json.Unmarshal(body, &user) != nil {
		return errors.New("invalid directory response")
	}
	if user.UserID != subject || user.Status != "active" {
		return ErrDenied
	}
	return nil
}
