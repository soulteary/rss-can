package server

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
)

func apiHealth() fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(fiber.Map{"message": "pong"})
	}
}
