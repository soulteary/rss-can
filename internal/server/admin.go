package server

import (
	"crypto/subtle"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// Management endpoints accept direct loopback clients by default. Remote
// access requires an explicitly configured bearer token; forwarded headers do
// not establish trust. A configured token is required even on loopback.
func adminOnly() gin.HandlerFunc {
	token := os.Getenv("RSS_ADMIN_TOKEN")
	return func(c *gin.Context) {
		if site := c.GetHeader("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if origin := c.GetHeader("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != c.Request.Host || (u.Scheme != "http" && u.Scheme != "https") {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}
		if token != "" {
			supplied, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
			if ok && subtle.ConstantTimeCompare([]byte(supplied), []byte(token)) == 1 {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"msg": "admin authorization required"})
			return
		}
		// A loopback reverse proxy does not make its remote clients local admins.
		if c.GetHeader("Forwarded") != "" || c.GetHeader("X-Forwarded-For") != "" || c.GetHeader("X-Real-IP") != "" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		// Also reject DNS rebinding via a non-local Host name.
		requestURL, err := url.Parse("http://" + c.Request.Host)
		if err != nil || !localHost(requestURL.Hostname()) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err == nil {
			if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"msg": "management access is restricted to loopback"})
	}
}

func localHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
