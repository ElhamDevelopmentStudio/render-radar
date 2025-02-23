package handlers

import (
	"github.com/gofiber/fiber/v2"
)

func GetSessions(c *fiber.Ctx) error {
	url := c.Query("url")
	if url == "" {
		return c.Status(400).JSON(fiber.Map{"error": "URL parameter is required"})
	}

	sessions := store.GetSessions(url)
	return c.JSON(fiber.Map{
		"url":      url,
		"sessions": sessions,
	})
}

func ClearSessions(c *fiber.Ctx) error {
	url := c.Query("url")
	if url == "" {
		return c.Status(400).JSON(fiber.Map{"error": "URL parameter is required"})
	}

	if err := store.ClearSessions(url); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to clear sessions"})
	}

	return c.JSON(fiber.Map{
		"message": "Sessions cleared successfully",
		"url":     url,
	})
} 