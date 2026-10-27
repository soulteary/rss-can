package network

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"time"
)

// PublicURL rejects non-web URLs and credentials before a browser is allocated.
func PublicURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("only public HTTP(S) URLs without credentials are allowed")
	}
	if u.Hostname() == "localhost" {
		return fmt.Errorf("local destinations are not allowed")
	}
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil && !isPublicIP(ip) {
		return fmt.Errorf("non-public destinations are not allowed")
	}
	return nil
}

func isPublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	// Additional non-public ranges (including cloud metadata and IPv6 translation).
	for _, cidr := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001::/23", "2001:db8::/32", "64:ff9b::/96", "64:ff9b:1::/48"} {
		if netip.MustParsePrefix(cidr).Contains(ip) {
			return false
		}
	}
	return true
}

// publicDialContext validates DNS results and dials a validated IP directly, so
// a second DNS lookup cannot redirect the inspector to an internal address.
func publicDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("no addresses for %s", host)
	}
	for _, ip := range addresses {
		if !isPublicIP(ip) {
			return nil, fmt.Errorf("non-public destination: %s", host)
		}
	}
	var dialer net.Dialer
	var last error
	for _, ip := range addresses {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		last = err
	}
	return nil, last
}

// PublicHTTPClient is used for every inspector request, including redirects and
// subresources. Redirects are returned to the browser and intercepted again.
func PublicHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		Transport:     &http.Transport{DialContext: publicDialContext, TLSHandshakeTimeout: timeout, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
