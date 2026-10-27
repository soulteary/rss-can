package server

import (
	"crypto/subtle"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// Management endpoints accept direct loopback clients by default. Remote
// access requires an explicitly configured bearer token; forwarded headers do
// not establish trust. A configured token is required even on loopback.
func adminOnly() fiber.Handler {
	token := os.Getenv("RSS_ADMIN_TOKEN")
	return func(c fiber.Ctx) error {
		status := adminAccessStatus(adminRequest{
			token:         token,
			site:          c.Get("Sec-Fetch-Site"),
			origin:        c.Get("Origin"),
			host:          c.Host(),
			authorization: c.Get("Authorization"),
			forwarded:     c.Get("Forwarded"),
			xForwardedFor: c.Get("X-Forwarded-For"),
			xRealIP:       c.Get("X-Real-IP"),
			remoteAddr:    c.RequestCtx().RemoteAddr().String(),
		})
		switch status {
		case 0:
			return c.Next()
		case http.StatusUnauthorized:
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"msg": "admin authorization required"})
		default:
			return c.Status(http.StatusForbidden).JSON(fiber.Map{"msg": "management access is restricted to loopback"})
		}
	}
}

type adminRequest struct {
	token, site, origin, host, authorization string
	forwarded, xForwardedFor, xRealIP        string
	remoteAddr                               string
}

func adminAccessStatus(request adminRequest) int {
	if request.site != "" && request.site != "same-origin" && request.site != "none" {
		return http.StatusForbidden
	}
	if request.origin != "" {
		u, err := url.Parse(request.origin)
		if err != nil || u.Host != request.host || (u.Scheme != "http" && u.Scheme != "https") {
			return http.StatusForbidden
		}
	}
	if request.token != "" {
		supplied, ok := strings.CutPrefix(request.authorization, "Bearer ")
		if ok && subtle.ConstantTimeCompare([]byte(supplied), []byte(request.token)) == 1 {
			return 0
		}
		return http.StatusUnauthorized
	}
	// A loopback reverse proxy does not make its remote clients local admins.
	if request.forwarded != "" || request.xForwardedFor != "" || request.xRealIP != "" {
		return http.StatusForbidden
	}
	// Also reject DNS rebinding via a non-local Host name.
	requestURL, err := url.Parse("http://" + request.host)
	if err != nil || !localHost(requestURL.Hostname()) {
		return http.StatusForbidden
	}
	host, _, err := net.SplitHostPort(request.remoteAddr)
	if err == nil {
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return 0
		}
	}
	return http.StatusForbidden
}

func localHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
