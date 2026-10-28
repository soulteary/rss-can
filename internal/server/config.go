package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/rule"
)

func apiConfig() fiber.Handler {
	return apiConfigWithReload(rule.Reload)
}

func apiConfigWithReload(reload func(context.Context) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		configType := c.Params("type")
		configValue := c.Params("value")
		if configType == "" || configValue == "" {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"msg": "missing config parameter"})
		}

		switch strings.ToLower(configType) {
		case "set-loglevel":
			logger.SetLevel(configValue)
			return c.Status(http.StatusOK).JSON(fiber.Map{"msg": fmt.Sprintf("Update loglevel with %s", configValue)})
		case "rules":
			if strings.ToLower(configValue) == "fresh" {
				ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
				defer cancel()
				if err := reload(ctx); err != nil {
					logger.Instance.Errorf("Rule reload failed: %v", err)
					return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"msg": "Rule reload failed; previous rules retained"})
				}
				return c.Status(http.StatusOK).JSON(fiber.Map{"msg": "Rules refreshed"})
			}
		}
		return c.SendStatus(http.StatusNotFound)
	}
}
