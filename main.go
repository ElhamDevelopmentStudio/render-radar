package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/websocket"
)

// Chrome DevTools Protocol (CDP) URL
const CHROME_DEBUGGER_URL = "http://localhost:9222/json"

// Struct to hold debugging targets
type DebuggingTarget struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	WebSocketDebuggerUrl string `json:"webSocketDebuggerUrl"`
}

// Fetch debugging targets from Chrome
func getDebuggingTargets() (*DebuggingTarget, error) {
	fmt.Println("Fetching debugging targets...")
	resp, err := http.Get(CHROME_DEBUGGER_URL)
	if err != nil {
		fmt.Printf("Error getting debug targets: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	var targets []DebuggingTarget
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		fmt.Printf("Error decoding targets: %v\n", err)
		return nil, err
	}

	// Find the test-errors page target
	for _, target := range targets {
		fmt.Printf("Found target: Type=%s, Title=%s, URL=%s\n", target.Type, target.Title, target.URL)
		if target.Type == "page" && strings.Contains(target.URL, "/test-errors") {
			fmt.Printf("Selected target with WebSocket URL: %s\n", target.WebSocketDebuggerUrl)
			return &target, nil
		}
	}

	return nil, fmt.Errorf("test-errors page not found")
}

// Connect to Chrome's WebSocket Debugger
func connectToDebugger(c *fiber.Ctx) error {
	fmt.Println("Starting debugger connection...")
	target, err := getDebuggingTargets()
	if err != nil {
		fmt.Printf("Error getting debug target: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Printf("Connecting to WebSocket URL: %s\n", target.WebSocketDebuggerUrl)
	ws, _, err := websocket.DefaultDialer.Dial(target.WebSocketDebuggerUrl, nil)
	if err != nil {
		fmt.Printf("WebSocket connection error: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to connect to Chrome Debugger"})
	}
	defer ws.Close()

	// Enable debugging features with more detailed parameters
	enableNetwork := map[string]interface{}{
		"id":     1,
		"method": "Network.enable",
		"params": map[string]interface{}{
			"maxTotalBufferSize":    10000000,
			"maxResourceBufferSize": 5000000,
		},
	}
	enableRuntime := map[string]interface{}{
		"id":     2,
		"method": "Runtime.enable",
		"params": map[string]interface{}{
			"notifyOnExceptionThrown": true,
		},
	}

	if err := ws.WriteJSON(enableNetwork); err != nil {
		fmt.Printf("Error enabling network: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to enable Network debugging"})
	}
	if err := ws.WriteJSON(enableRuntime); err != nil {
		fmt.Printf("Error enabling runtime: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to enable Runtime debugging"})
	}

	debuggingData := []string{}
	timeout := time.After(30 * time.Second)
	messageChannel := make(chan []byte)

	go func() {
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				fmt.Printf("WebSocket read error: %v\n", err)
				close(messageChannel)
				return
			}
			messageChannel <- message
		}
	}()

	for {
		select {
		case message, ok := <-messageChannel:
			if !ok {
				fmt.Println("Message channel closed")
				return c.JSON(fiber.Map{"debug_logs": debuggingData})
			}
			
			var data map[string]interface{}
			if err := json.Unmarshal(message, &data); err != nil {
				fmt.Printf("JSON unmarshal error: %v\n", err)
				continue
			}

			fmt.Printf("Received message: %s\n", string(message))

			if method, ok := data["method"].(string); ok {
				switch method {
				case "Runtime.exceptionThrown":
					if params, ok := data["params"].(map[string]interface{}); ok {
						if exceptionDetails, ok := params["exceptionDetails"].(map[string]interface{}); ok {
							errorMessage := ""
							if text, ok := exceptionDetails["text"].(string); ok {
								errorMessage = text
							}
							if exception, ok := exceptionDetails["exception"].(map[string]interface{}); ok {
								if description, ok := exception["description"].(string); ok {
									errorMessage = description
								}
							}
							debuggingData = append(debuggingData, fmt.Sprintf("❌ JS Error: %s", errorMessage))
							fmt.Printf("Captured error: %s\n", errorMessage)
						}
					}

				case "Network.requestWillBeSent":
					if params, ok := data["params"].(map[string]interface{}); ok {
						if request, ok := params["request"].(map[string]interface{}); ok {
							debuggingData = append(debuggingData, fmt.Sprintf("📡 Network Request: %v %v", 
								request["method"], request["url"]))
							fmt.Printf("Captured network request: %v %v\n", request["method"], request["url"])
						}
					}
				}
			}

		case <-timeout:
			fmt.Println("Debugger timeout reached")
			return c.JSON(fiber.Map{"debug_logs": debuggingData})
		}
	}
}

func main() {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: false,
	})

	fmt.Println("Setting up routes...")

	// Root route
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("🚀 Debugger API is running!")
	})

	// Test route that creates errors in Chrome
	app.Get("/test-errors", func(c *fiber.Ctx) error {
		fmt.Println("Serving test-errors page...")
		html := `
		<html>
			<head>
				<style>
					.log { margin: 10px; padding: 10px; border: 1px solid #ccc; }
					.error { color: red; }
				</style>
			</head>
			<body>
				<h1>Testing Errors</h1>
				<div id="logs"></div>
				<script>
					function addLog(msg, isError = false) {
						const div = document.createElement('div');
						div.className = 'log' + (isError ? ' error' : '');
						div.textContent = msg;
						document.getElementById('logs').appendChild(div);
					}

					// Log start
					addLog('Starting error tests...');
					
					// Network requests
					addLog('Making network requests...');
					fetch('https://api.example.com/nonexistent')
						.catch(e => addLog('Network error: ' + e.message, true));
					
					// Reference error
					addLog('Triggering reference error...');
					try {
						undefinedVariable.someMethod();
					} catch(e) {
						addLog('Reference error: ' + e.message, true);
						console.error(e);
					}
					
					// Syntax error
					addLog('Triggering syntax error...');
					try {
						eval('if true { console.log("bad syntax") }');
					} catch(e) {
						addLog('Syntax error: ' + e.message, true);
						console.error(e);
					}
					
					// Type error
					addLog('Triggering type error...');
					try {
						null.toString();
					} catch(e) {
						addLog('Type error: ' + e.message, true);
						console.error(e);
					}

					// Custom error
					addLog('Throwing custom error...');
					throw new Error("This is a test error");
				</script>
			</body>
		</html>
		`
		return c.Type("html").SendString(html)
	})

	// Start Debugger Route
	app.Get("/start-debugger", connectToDebugger)

	port := 8000
	fmt.Printf("🚀 Server starting on http://localhost:%d\n", port)
	
	if err := app.Listen(fmt.Sprintf(":%d", port)); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}