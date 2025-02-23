package server

import (
	"debugger-api/internal/handlers"
	"fmt"

	"github.com/gofiber/fiber/v2"
)

func SetupAndRun() error {
	app := fiber.New()

	// Root route
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("🚀 Debugger API is running!")
	})

	// Test route that creates errors in Chrome
	app.Get("/test-errors", handlers.HandleTestErrors)

	// Start Debugger Route - support both GET and POST
	app.Post("/start-debugger", handlers.HandleDebugger)

	port := 8000
	fmt.Printf("🚀 Server starting on http://localhost:%d\n", port)
	
	return app.Listen(fmt.Sprintf(":%d", port))
} 