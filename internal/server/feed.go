package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/generator"
	"github.com/soulteary/RSS-Can/internal/rule"
)

func generateFeedResponse(config define.JavaScriptConfig, data define.BodyParsed, rssType string) (mimetype string, response string) {
	switch rssType {
	case define.FEED_TYPE_RSS:
		mimetype = define.FEED_MIME_TYPE_RSS
		response = generator.GenerateFeedsByType(config, data, rssType)
	case define.FEED_TYPE_ATOM:
		mimetype = define.FEED_MIME_TYPE_ATOM
		response = generator.GenerateFeedsByType(config, data, rssType)
	case define.FEED_TYPE_JSON:
		mimetype = define.FEED_MIME_TYPE_JSON
		response = generator.GenerateFeedsByType(config, data, rssType)
	default:
		mimetype = define.FEED_MIME_TYPE_JSON
		response = "incorrect type"
	}
	return mimetype, response
}

func apiRSS() fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		rssType := c.Params("type")
		if id == "" || rssType == "" {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"msg": "missing feed parameter"})
		}

		ruleFile, exist := rule.Lookup(id)
		if !exist {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"msg": "rule not found"})
		}

		config, err := rule.GenerateConfigByRule(ruleFile)
		if err != nil {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"msg": "parse config failed"})
		}

		data := rule.GetWebsiteDataWithConfig(config)
		if data.Code != define.ERROR_CODE_NULL {
			return c.Status(http.StatusBadGateway).JSON(fiber.Map{"msg": fmt.Sprintf("get website data failed, code:%v", data.Code)})
		}

		mimetype, response := generateFeedResponse(config, data, strings.ToLower(rssType))
		c.Set(fiber.HeaderContentType, mimetype)
		return c.Status(http.StatusOK).SendString(response)
	}
}
