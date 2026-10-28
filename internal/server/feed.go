package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
	"github.com/soulteary/RSS-Can/internal/generator"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/rule"
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

func apiRSS() fiber.Handler {
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
		rssType := strings.ToLower(c.Params("type"))
		if id == "" || rssType == "" {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"msg": "missing feed parameter"})
		}
		mimetype, supported := feedMIME(rssType)
		if !supported {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"msg": "unsupported feed type"})
		}
		ruleFile, exist := rule.Lookup(id)
		if !exist {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"msg": "rule not found"})
		}

		// One budget for JS queueing/execution, cache operations, browser/HTTP
		// requests, pagination, detail hooks and feed serialization.
		ctx, cancel := context.WithTimeout(c.Context(), fn.ExpireBySecond(define.SERVER_TIMEOUT))
		defer cancel()
		timedOut := func() error {
			return c.Status(http.StatusGatewayTimeout).JSON(fiber.Map{"msg": "feed generation timed out"})
		}
		config, err := rule.GenerateConfigByRuleContext(ctx, ruleFile)
		if ctx.Err() != nil {
			return timedOut()
		}
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"msg": "parse config failed"})
		}

		data := rule.GetWebsiteDataWithConfigContext(ctx, config)
		if ctx.Err() != nil {
			return timedOut()
		}
		if data.Code != define.ERROR_CODE_NULL {
			return c.Status(http.StatusBadGateway).JSON(fiber.Map{"msg": fmt.Sprintf("get website data failed, code:%v", data.Code)})
		}
		response, err := generator.GenerateFeedsByTypeContext(ctx, config, data, rssType)
		if ctx.Err() != nil {
			return timedOut()
		}
		if err != nil {
			logger.Instance.Errorf("Generate feed failed: %v", err)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"msg": "generate feed failed"})
		}
		c.Set(fiber.HeaderContentType, mimetype)
		return c.Status(http.StatusOK).SendString(response)
	}
}
