package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// HandleTestErrors serves the test page that generates JavaScript errors
func HandleTestErrors(c *fiber.Ctx) error {
    html := `
    <html>
        <head>
            <style>
                .log { margin: 10px; padding: 10px; border: 1px solid #ccc; }
                .error { color: red; }
            </style>
        </head>
        <body>
            <h1>Testing Console Output</h1>
            <div id="logs"></div>
            <script>
                function addLog(msg, isError = false) {
                    const div = document.createElement('div');
                    div.className = 'log' + (isError ? ' error' : '');
                    div.textContent = msg;
                    document.getElementById('logs').appendChild(div);
                }

                // Complex object logging
                const user = {
                    id: 1234,
                    name: "Test User",
                    preferences: {
                        theme: "dark",
                        notifications: true,
                        settings: {
                            autoSave: true,
                            compression: "high"
                        }
                    },
                    activities: [
                        { type: "login", timestamp: new Date().toISOString() },
                        { type: "action", details: "Updated profile" }
                    ]
                };
                console.log("User object:", user);
                console.table(user.activities);

                // Array and structured data
                const metrics = [
                    { name: "CPU", value: 85.5, unit: "%" },
                    { name: "Memory", value: 2048, unit: "MB" },
                    { name: "Disk", value: 256, unit: "GB" }
                ];
                console.log("System metrics:", metrics);
                console.table(metrics);

                // Warning with object
                const performanceWarning = {
                    type: "Performance",
                    message: "High memory usage detected",
                    details: {
                        current: "85%",
                        threshold: "75%",
                        recommendations: ["Clear cache", "Close unused tabs"]
                    }
                };
                console.warn("Performance warning:", performanceWarning);

                // Error with stack trace
                try {
                    const obj = null;
                    obj.nonexistentMethod();
                } catch (e) {
                    console.error("Critical error:", {
                        name: e.name,
                        message: e.message,
                        stack: e.stack,
                        timestamp: new Date().toISOString()
                    });
                }

                // Network error simulation
                fetch('https://api.nonexistent.com/data')
                    .then(response => response.json())
                    .catch(error => {
                        console.error("Network failure:", {
                            type: "API_ERROR",
                            endpoint: "https://api.nonexistent.com/data",
                            error: error.message,
                            timestamp: new Date().toISOString()
                        });
                    });

                // Info with nested data
                console.info("Application state:", {
                    version: "1.0.0",
                    environment: "testing",
                    features: {
                        enabled: ["debug", "metrics", "logging"],
                        disabled: ["analytics"]
                    },
                    session: {
                        id: "sess_" + Math.random().toString(36).substr(2),
                        started: new Date().toISOString(),
                        user: "test@example.com"
                    }
                });

                // Debug with timing
                console.time("operation");
                for (let i = 0; i < 1000; i++) {
                    // Simulate work
                }
                console.timeEnd("operation");

                // Group logged messages
                console.group("Authentication Flow");
                console.log("Checking credentials...");
                console.log("Token generated:", "eyJhbGc...[truncated]");
                console.warn("Using development keys");
                console.groupEnd();

                // Custom error with detailed context
                const customError = new Error("Custom Application Error");
                customError.code = "APP_ERR_001";
                customError.context = {
                    component: "UserManager",
                    action: "profile_update",
                    params: { userId: 123, updates: { email: "new@example.com" } }
                };
                console.error("Application error:", customError);

                // Throw final error to test error boundary
                throw new Error("Test completed with simulated crash");
            </script>
        </body>
    </html>
    `
    return c.Type("html").SendString(html)
} 