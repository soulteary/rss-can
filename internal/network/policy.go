package network

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/soulteary/RSS-Can/internal/limits"
)

// FetchPolicy is captured when the application starts. Public mode permits
// only public web destinations; trusted mode preserves operator-owned rules.
// Public mode deliberately does not enable browser execution: page interception
// alone cannot guarantee the network isolation of every browser target.
type FetchPolicy struct{ Mode string }

const (
	FetchPublic            = "public"
	FetchTrusted           = "trusted"
	MaxTaskRequests        = 64
	MaxTaskBodyBytes int64 = 64 << 20
)

var ErrFetchPolicy = errors.New("fetch destination is blocked by policy")
var ErrFetchBudget = errors.New("fetch task budget exceeded")

func PublicPolicy() FetchPolicy { return FetchPolicy{Mode: FetchPublic} }

func ParseFetchPolicy(mode string) (FetchPolicy, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = FetchTrusted
	}
	if mode != FetchPublic && mode != FetchTrusted {
		return FetchPolicy{}, fmt.Errorf("RSS_FETCH_POLICY must be public or trusted")
	}
	return FetchPolicy{Mode: mode}, nil
}
func (p FetchPolicy) PublicOnly() bool { return p.Mode == FetchPublic }
func (p FetchPolicy) ValidateURL(raw string) error {
	if p.Mode != "" && p.Mode != FetchPublic && p.Mode != FetchTrusted {
		return ErrFetchPolicy
	}
	u, err := url.Parse(raw)
	if len(raw) > limits.MaxDocumentURLBytes || err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ErrFetchPolicy
	}
	if p.PublicOnly() {
		if err := PublicURL(raw); err != nil {
			return ErrFetchPolicy
		}
		if port := u.Port(); port != "" && port != "80" && port != "443" {
			return ErrFetchPolicy
		}
	}
	return nil
}
func (p FetchPolicy) Scope() string {
	return fmt.Sprintf("fetch-policy:v1:%x", sha256.Sum256([]byte(p.Mode)))
}

type policyKey struct{}

func WithFetchPolicy(ctx context.Context, p FetchPolicy) context.Context {
	return context.WithValue(ctx, policyKey{}, p)
}
func PolicyForContext(ctx context.Context) FetchPolicy {
	if p, ok := ctx.Value(policyKey{}).(FetchPolicy); ok {
		return p
	}
	return configForContext(ctx).Policy
}

type taskFetchBudget struct {
	mu       sync.Mutex
	requests int
	bytes    int64
}
type budgetKey struct{}

// WithFetchBudget starts a budget shared by all pages, redirects and details in
// one generation. Rebinding resources must retain this context.
func WithFetchBudget(ctx context.Context) context.Context {
	if _, ok := ctx.Value(budgetKey{}).(*taskFetchBudget); ok {
		return ctx
	}
	return context.WithValue(ctx, budgetKey{}, &taskFetchBudget{})
}
func taskBudget(ctx context.Context) *taskFetchBudget {
	b, _ := ctx.Value(budgetKey{}).(*taskFetchBudget)
	return b
}
func (b *taskFetchBudget) request() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.requests >= MaxTaskRequests {
		return ErrFetchBudget
	}
	b.requests++
	return nil
}
func (b *taskFetchBudget) consume(n int) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if int64(n) > MaxTaskBodyBytes-b.bytes {
		b.bytes = MaxTaskBodyBytes
		return ErrFetchBudget
	}
	b.bytes += int64(n)
	return nil
}

type policyBody struct {
	io.ReadCloser
	budget *taskFetchBudget
}

func (b *policyBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if budgetErr := b.budget.consume(n); budgetErr != nil {
		return 0, budgetErr
	}
	return n, err
}

type policyTransport struct {
	base   http.RoundTripper
	policy FetchPolicy
}

func (t policyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.policy.ValidateURL(req.URL.String()); err != nil {
		return nil, err
	}
	if err := taskBudget(req.Context()).request(); err != nil {
		return nil, err
	}
	res, err := t.base.RoundTrip(req)
	if res != nil && res.Body != nil {
		res.Body = &policyBody{ReadCloser: res.Body, budget: taskBudget(req.Context())}
	}
	return res, err
}

// policyDialContext resolves once and dials the validated addresses directly.
// The injected resolver/dialer are private test seams, never request options.
func policyDialContext(ctx context.Context, network, address string, lookup func(context.Context, string, string) ([]netip.Addr, error), dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := lookup(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, ErrFetchPolicy
	}
	for _, ip := range addresses {
		if !isPublicIP(ip) {
			return nil, ErrFetchPolicy
		}
	}
	for _, ip := range addresses {
		var c net.Conn
		c, err = dial(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return c, nil
		}
	}
	return nil, err
}
func policyHTTPClient(ctx context.Context, timeout time.Duration, proxy string) (*http.Client, *http.Transport, error) {
	p := PolicyForContext(ctx)
	if p.PublicOnly() && proxy != "" {
		return nil, nil, ErrFetchPolicy
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if p.PublicOnly() {
		tr.Proxy = nil
		tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			return policyDialContext(ctx, network, address, net.DefaultResolver.LookupNetIP, (&net.Dialer{}).DialContext)
		}
	} else if proxy != "" {
		u, _ := url.Parse(proxy)
		tr.Proxy = http.ProxyURL(u)
	}
	client := &http.Client{Timeout: timeout, Transport: policyTransport{base: tr, policy: p}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		return p.ValidateURL(req.URL.String())
	}}
	return client, tr, nil
}
func exactOrigin(u *url.URL) string {
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return strings.ToLower(u.Scheme) + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}

type originJar struct {
	jar    http.CookieJar
	origin string
}

func (j originJar) Cookies(u *url.URL) []*http.Cookie {
	if exactOrigin(u) != j.origin {
		return nil
	}
	return j.jar.Cookies(u)
}
func (j originJar) SetCookies(u *url.URL, c []*http.Cookie) {
	if exactOrigin(u) == j.origin {
		j.jar.SetCookies(u, c)
	}
}
func newOriginJar(u *url.URL) http.CookieJar {
	j, _ := cookiejar.New(nil)
	return originJar{jar: j, origin: exactOrigin(u)}
}

func policyDocumentKey(ctx context.Context, key string) string {
	p := PolicyForContext(ctx)
	if !p.PublicOnly() {
		return key
	}
	return p.Scope() + ":" + key
}

// ValidateRule prevents source-provided network configuration from weakening
// public mode, even if the source is already present in a cache.
func (p FetchPolicy) ValidateRule(target, proxy, headless, mode string) error {
	if err := p.ValidateURL(target); err != nil {
		return err
	}
	if p.PublicOnly() && (strings.TrimSpace(proxy) != "" || strings.TrimSpace(headless) != "") {
		return fmt.Errorf("%w: source network overrides are disabled", ErrFetchPolicy)
	}
	if p.PublicOnly() && (strings.EqualFold(mode, "csr") || strings.EqualFold(mode, "mix")) {
		return fmt.Errorf("%w: browser execution requires a separately isolated trusted deployment", ErrFetchPolicy)
	}
	return nil
}
