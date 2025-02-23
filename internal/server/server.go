package server

import (
	"debugger-api/internal/handlers"
	"fmt"
	"net"

	"github.com/gofiber/fiber/v2"
)

func SetupAndRun() error {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Kill any existing process on port 8000
	killExistingProcess(8000)

	// Root route
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("🚀 Debugger API is running!")
	})

	// Test route that creates errors in Chrome
	app.Get("/test-errors", handlers.HandleTestErrors)

	// Start Debugger Route - support both GET and POST
	app.Post("/start-debugger", handlers.HandleDebugger)

	// Sessions route
	app.Get("/sessions", handlers.GetSessions)
	app.Delete("/sessions", handlers.ClearSessions)

	port := 8000
	fmt.Printf("🚀 Server starting on http://localhost:%d\n", port)
	
	return app.Listen(fmt.Sprintf(":%d", port))
}

func killExistingProcess(port int) {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err == nil {
		listener.Close()
		return
	}
	// On Windows, you might need to run the server with admin privileges
	// or manually kill the process using Task Manager
} 