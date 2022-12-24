package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const transPixel = "\x47\x49\x46\x38\x39\x61\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\x00\x00\x00\x21\xF9\x04\x01\x00\x00\x00\x00\x2C\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02\x44\x01\x00\x3B"

func PixelImage() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Pragma", "no-cache")
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, post-check=0, pre-check=0")
		c.Header("Expires", "Wed, 11 Nov 1998 11:11:11 GMT")

		c.Header("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		c.Data(http.StatusOK, "image/gif", []byte(transPixel))
	}
}
