package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	"debugger-api/internal/debugger"

	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/websocket"
)

// HandleDebugger now accepts a list of URLs to debug
func HandleDebugger(c *fiber.Ctx) error {
	var req debugger.DebugRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if len(req.URLs) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "No URLs provided"})
	}

	chrome := debugger.NewChromeDebugger()
	targets, err := chrome.GetDebuggingTargets(req.URLs)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	response := debugger.DebugResponse{
		Results: make(map[string][]string),
		Errors:  make(map[string]string),
	}

	// Create a channel to collect results from all goroutines
	resultsChan := make(chan struct {
		url    string
		logs   []string
		err    error
	}, len(req.URLs))

	// Start a goroutine for each target
	for url, target := range targets {
		go func(url string, target *debugger.DebuggingTarget) {
			logs, err := debugTarget(target)
			resultsChan <- struct {
				url    string
				logs   []string
				err    error
			}{url, logs, err}
		}(url, target)
	}

	// Collect results
	for i := 0; i < len(targets); i++ {
		result := <-resultsChan
		if result.err != nil {
			response.Errors[result.url] = result.err.Error()
		} else {
			response.Results[result.url] = result.logs
		}
	}

	return c.JSON(response)
}

func enableDebugging(ws *websocket.Conn) error {
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
		return fmt.Errorf("failed to enable Network debugging: %v", err)
	}
	if err := ws.WriteJSON(enableRuntime); err != nil {
		return fmt.Errorf("failed to enable Runtime debugging: %v", err)
	}
	return nil
}

func captureDebugMessages(ws *websocket.Conn) []string {
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
				return debuggingData
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
						}
					}
				case "Network.requestWillBeSent":
					if params, ok := data["params"].(map[string]interface{}); ok {
						if request, ok := params["request"].(map[string]interface{}); ok {
							debuggingData = append(debuggingData, fmt.Sprintf("📡 Network Request: %v %v", 
								request["method"], request["url"]))
						}
					}
				}
			}

		case <-timeout:
			fmt.Println("Debugger timeout reached")
			return debuggingData
		}
	}
}

func debugTarget(target *debugger.DebuggingTarget) ([]string, error) {
	ws, _, err := websocket.DefaultDialer.Dial(target.WebSocketDebuggerUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket connection error: %v", err)
	}
	defer ws.Close()

	if err := enableDebugging(ws); err != nil {
		return nil, err
	}

	return captureDebugMessages(ws), nil
} 