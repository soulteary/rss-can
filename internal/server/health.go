package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/jssdk"
	"github.com/soulteary/RSS-Can/internal/telemetry"
)

// Liveness stays cheap and compatible; dependency failures belong to readiness.
func apiHealth() fiber.Handler {
	return func(c fiber.Ctx) error { return c.Status(http.StatusOK).JSON(fiber.Map{"message": "pong"}) }
}

func apiReady() fiber.Handler {
	return readiness(func(ctx context.Context) error {
		_, err := jssdk.RunCodeContext(ctx, "", "0", "readiness.js")
		return err
	}, cacher.Check)
}

func readiness(runtime func(context.Context) error, cache func(context.Context) cacher.Status) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), time.Second)
		defer cancel()
		// Probe concurrently so slow Redis cannot prevent the runtime check.
		jsResult := make(chan error, 1)
		cacheResult := make(chan cacher.Status, 1)
		go func() { jsResult <- runtime(ctx) }()
		go func() { cacheResult <- cache(ctx) }()
		jsOK := false
		cacheStatus := cacher.Status{Backend: "unknown", Degraded: true}
		for remaining := 2; remaining > 0; remaining-- {
			select {
			case err := <-jsResult:
				jsOK = err == nil
				jsResult = nil
			case status := <-cacheResult:
				cacheStatus = status
				cacheResult = nil
			case <-ctx.Done():
				remaining = 1
			}
		}
		ready := jsOK && cacheStatus.Ready && ctx.Err() == nil
		status := http.StatusOK
		if !ready {
			status = http.StatusServiceUnavailable
		}
		c.Set("Cache-Control", "no-store")
		return c.Status(status).JSON(fiber.Map{"ready": ready, "javascript": jsOK, "cache": cacheStatus})
	}
}

func apiMetrics() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "text/plain; version=0.0.4; charset=utf-8")
		c.Set("Cache-Control", "no-store")
		return c.SendString(telemetry.Default.Prometheus())
	}
}
