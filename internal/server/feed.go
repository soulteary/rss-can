package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/feedservice"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/telemetry"
)

func feedMIME(rssType string) (string, bool) {
	switch rssType {
	case define.FEED_TYPE_RSS:
		return define.FEED_MIME_TYPE_RSS, true
	case define.FEED_TYPE_ATOM:
		return define.FEED_MIME_TYPE_ATOM, true
	case define.FEED_TYPE_JSON:
		return define.FEED_MIME_TYPE_JSON, true
	default:
		return "", false
	}
}

type FeedGenerator interface {
	Generate(context.Context, string, string) (string, error)
}

type feedRepresentations interface {
	Representation(context.Context, string, string) (feedservice.Representation, error)
}

func apiRSS() fiber.Handler {
	return apiRSSWithService(defaultFeedGenerator())
}

func defaultFeedGenerator() FeedGenerator {
	return feedservice.NewDefault(fn.ExpireBySecond(define.SERVER_TIMEOUT))
}

func apiRSSWithService(service FeedGenerator) fiber.Handler {
	return func(c fiber.Ctx) (err error) {
		began := time.Now()
		telemetry.Default.Add("feed_requests_total", 1)
		defer func() {
			telemetry.Default.Observe("feed_duration_seconds", time.Since(began))
			if c.Response().StatusCode() >= 400 || err != nil {
				telemetry.Default.Add("feed_failures_total", 1)
			} else {
				telemetry.Default.Add("feed_successes_total", 1)
			}
			if c.Response().StatusCode() == http.StatusGatewayTimeout {
				telemetry.Default.Add("feed_timeouts_total", 1)
			}
		}()
		id := c.Params("id")
		if resolved, ok := c.Locals(sourceLocal).(string); ok {
			id = resolved
		}
		rssType := strings.ToLower(c.Params("type"))
		if id == "" || rssType == "" {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"msg": "missing feed parameter"})
		}
		mimetype, supported := feedMIME(rssType)
		if !supported {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"msg": "unsupported feed type"})
		}
		var response string
		var representation feedservice.Representation
		if snapshots, ok := service.(feedRepresentations); ok {
			representation, err = snapshots.Representation(c.Context(), id, rssType)
			response = representation.Body
		} else {
			response, err = service.Generate(c.Context(), id, rssType)
		}
		if err != nil {
			logger.Instance.Error("Feed generation failed; details withheld to protect source credentials")
			var failure *feedservice.Error
			if !errors.As(err, &failure) {
				return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"msg": "generate feed failed"})
			}
			status, message := http.StatusInternalServerError, "generate feed failed"
			switch failure.Stage {
			case feedservice.InvalidFormat:
				status, message = http.StatusBadRequest, "unsupported feed type"
			case feedservice.MissingRule:
				status, message = http.StatusNotFound, "rule not found"
			case feedservice.Configuration:
				message = "parse config failed"
			case feedservice.Fetch:
				status, message = http.StatusBadGateway, fmt.Sprintf("get website data failed, code:%v", failure.Code)
				if failure.Code == define.ERROR_CODE_OVERLOADED {
					status, message = http.StatusServiceUnavailable, "upstream fetch queue is full"
				}
			case feedservice.Admission:
				status, message = http.StatusServiceUnavailable, "feed generation queue is full"
			case feedservice.RateLimited:
				status, message = http.StatusTooManyRequests, "feed owner queue is full"
			case feedservice.Timeout:
				status, message = http.StatusGatewayTimeout, "feed generation timed out"
			}
			if status == http.StatusServiceUnavailable || status == http.StatusTooManyRequests {
				c.Set("Retry-After", "5")
			}
			return c.Status(status).JSON(fiber.Map{"msg": message})
		}

		c.Set(fiber.HeaderContentType, mimetype)
		// A representation is not an authorization decision. This handler must
		// remain behind the same authorization middleware as ordinary downloads.
		if c.GetRespHeader("Cache-Control") == "" {
			c.Set("Cache-Control", "private, no-cache")
		}
		if representation.ETag != "" {
			c.Set("ETag", representation.ETag)
			c.Set("Last-Modified", representation.ChangedAt.UTC().Format(http.TimeFormat))
			if representation.Stale {
				c.Set("Warning", `110 - "Response is stale"`)
			}
			if feedNotModified(c.Get("If-None-Match"), c.Get("If-Modified-Since"), representation) {
				return c.Status(http.StatusNotModified).Send(nil)
			}
		}
		return c.Status(http.StatusOK).SendString(response)
	}
}

func feedNotModified(etags, modified string, representation feedservice.Representation) bool {
	if etags != "" {
		for _, tag := range strings.Split(etags, ",") {
			tag = strings.TrimSpace(tag)
			if tag == "*" || strings.TrimPrefix(tag, "W/") == strings.TrimPrefix(representation.ETag, "W/") {
				return true
			}
		}
		return false
	}
	if modified != "" && !representation.ChangedAt.IsZero() {
		at, err := http.ParseTime(modified)
		return err == nil && !representation.ChangedAt.Truncate(time.Second).After(at)
	}
	return false
}
