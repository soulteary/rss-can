package server

// modify file: https://github.com/toorop/gin-logrus/blob/master/logger.go

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

// 2016-09-27 09:38:21.541541811 +0200 CEST
// 127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700]
// "GET /apache_pb.gif HTTP/1.0" 200 2326
// "http://www.example.com/start.html"
// "Mozilla/4.08 [en] (Win98; I ;Nav)"

var timeFormat = "02/Jan/2006:15:04:05 -0700"

func Logger(logger *zap.SugaredLogger, notLogged ...string) fiber.Handler {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknow"
	}

	var skip map[string]struct{}

	if length := len(notLogged); length > 0 {
		skip = make(map[string]struct{}, length)

		for _, p := range notLogged {
			skip[p] = struct{}{}
		}
	}

	return func(c fiber.Ctx) error {
		// other handler can change c.Path so:
		path := inputSanitized(c.Path())
		start := time.Now()
		err := c.Next()
		stop := time.Since(start)
		latency := int(math.Ceil(float64(stop.Nanoseconds()) / 1000000.0))
		statusCode := c.Response().StatusCode()
		clientIP := inputSanitized(c.IP())
		clientUserAgent := inputSanitized(c.Get(fiber.HeaderUserAgent))
		referer := inputSanitized(c.Get(fiber.HeaderReferer))
		dataLength := len(c.Response().Body())

		if _, ok := skip[path]; ok {
			return err
		}

		entry := logger.With(
			"hostname", hostname,
			"statusCode", statusCode,
			"latency", latency, // time to process
			"clientIP", clientIP,
			"method", c.Method(),
			"path", path,
			"referer", referer,
			"dataLength", dataLength,
			"userAgent", clientUserAgent,
		)

		if err != nil {
			entry.Error(err)
		} else {
			msg := fmt.Sprintf("%s - %s [%s] \"%s %s\" %d %d \"%s\" \"%s\" (%dms)", clientIP, hostname, time.Now().Format(timeFormat), c.Method(), path, statusCode, dataLength, referer, clientUserAgent, latency)
			if statusCode >= http.StatusInternalServerError {
				entry.Error(msg)
			} else if statusCode >= http.StatusBadRequest {
				entry.Warn(msg)
			} else {
				entry.Info(msg)
			}
		}
		return err
	}
}

func inputSanitized(input string) string {
	return strings.Replace(strings.Replace(input, "\n", "", -1), "\r", "", -1)
}
