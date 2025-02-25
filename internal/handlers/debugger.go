package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"debugger-api/internal/debugger"
	"debugger-api/internal/storage"

	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/websocket"
)

// Add as package-level variable
var store *storage.Store

// Add init function
func init() {
	var err error
	store, err = storage.NewStore("./data")
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize storage: %v", err))
	}
}

// HandleDebugger now accepts a list of URLs to debug
func HandleDebugger(c *fiber.Ctx) error {
	fmt.Println("🚀 Starting debug session...")

	// Clear previous sessions
	if err := store.ClearAllSessions(); err != nil {
		fmt.Printf("❌ Failed to clear sessions: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to cleanup"})
	}
	fmt.Println("✅ Cleared previous sessions")

	var req debugger.DebugRequest
	if err := c.BodyParser(&req); err != nil {
		fmt.Printf("❌ Invalid request body: %v\n", err)
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	fmt.Printf("📍 Debugging URLs: %v\n", req.URLs)

	chrome := debugger.NewChromeDebugger()
	targets, err := chrome.GetDebuggingTargets(req.URLs)
	if err != nil {
		fmt.Printf("❌ Failed to get targets: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Printf("✅ Found %d targets\n", len(targets))

	response := debugger.DebugResponse{
		Results: make(map[string]debugger.PageResults),
		Errors:  make(map[string]string),
	}

	// Debug each target sequentially for now (removing goroutines for simplicity)
	for url, target := range targets {
		fmt.Printf("📍 Debugging target: %s\n", url)
		logs, err := debugTarget(target)
		if err != nil {
			fmt.Printf("❌ Error debugging %s: %v\n", url, err)
			response.Errors[url] = err.Error()
			continue
		}
		
		results := categorizeMessages(logs)
		response.Results[url] = results
		fmt.Printf("✅ Collected %d console, %d errors messages\n", 
			len(results.Console), len(results.Errors))
	}

	// Save results
	for url, results := range response.Results {
		if err := store.SaveSession(url, results, response.Errors); err != nil {
			fmt.Printf("❌ Failed to save session for %s: %v\n", url, err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to save"})
		}
	}

	fmt.Println("✅ Debug session completed")
	return c.JSON(response)
}

func enableDebugging(ws *websocket.Conn) error {
	// Enable Runtime domain first
	enableRuntime := map[string]interface{}{
		"id":     1,
		"method": "Runtime.enable",
		"params": map[string]interface{}{
			"notifyOnConsoleAPICalled": true,
		},
	}

	// Enable Console domain
	enableConsole := map[string]interface{}{
		"id":     2,
		"method": "Console.enable",
	}

	// Enable property collection
	enableProperties := map[string]interface{}{
		"id":     3,
		"method": "Runtime.setCustomObjectFormatterEnabled",
		"params": map[string]interface{}{
			"enabled": true,
		},
	}

	for _, command := range []map[string]interface{}{enableRuntime, enableConsole, enableProperties} {
		if err := ws.WriteJSON(command); err != nil {
			return fmt.Errorf("failed to enable debugging feature: %v", err)
		}
		// Wait for response
		_, _, err := ws.ReadMessage()
		if err != nil {
			return fmt.Errorf("failed to read response: %v", err)
		}
	}
	return nil
}

func captureDebugMessages(ws *websocket.Conn) []debugger.ConsoleMessage {
	messages := []debugger.ConsoleMessage{}
	timeout := time.After(30 * time.Second)
	messageChannel := make(chan []byte)

	// Start message reader
	go func() {
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
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
				return messages
			}

			var data map[string]interface{}
			if err := json.Unmarshal(message, &data); err != nil {
				continue
			}

			// Handle the message (responses and events)
			handleWebSocketMessage(data)

			// Process console messages
			if method, ok := data["method"].(string); ok {
				switch method {
				case "Console.messageAdded":
					msg := parseConsoleMessage(data)
					messages = append(messages, msg)
				case "Runtime.consoleAPICalled":
					msg := parseRuntimeConsole(ws, data)
					messages = append(messages, msg)
				}
			}

		case <-timeout:
			return messages
		}
	}
}

func debugTarget(target *debugger.DebuggingTarget) ([]debugger.ConsoleMessage, error) {
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

func categorizeMessages(messages []debugger.ConsoleMessage) debugger.PageResults {
	results := debugger.PageResults{
		Console: make([]debugger.ConsoleMessage, 0),
		Errors:  make([]debugger.ConsoleMessage, 0),
	}

	for _, msg := range messages {
		switch msg.Type {
		case "error", "exception":
			results.Errors = append(results.Errors, msg)
		default:
			results.Console = append(results.Console, msg)
		}
	}

	return results
}

func parseConsoleMessage(data map[string]interface{}) debugger.ConsoleMessage {
	params, ok := data["params"].(map[string]interface{})
	if !ok {
		return debugger.ConsoleMessage{}
	}

	message, ok := params["message"].(map[string]interface{})
	if !ok {
		return debugger.ConsoleMessage{}
	}

	return debugger.ConsoleMessage{
		Type:    message["level"].(string),
		Time:    time.Now(),
		Message: message["text"].(string),
		URL:     message["url"].(string),
	}
}

func parseRuntimeConsole(ws *websocket.Conn, data map[string]interface{}) debugger.ConsoleMessage {
	params, ok := data["params"].(map[string]interface{})
	if !ok {
			return debugger.ConsoleMessage{}
	}

	args := params["args"].([]interface{})
	var message strings.Builder

	for i, arg := range args {
			argMap := arg.(map[string]interface{})
			
			if i > 0 {
					message.WriteString(" ")
			}

			switch argMap["type"].(string) {
			case "string":
					if value, ok := argMap["value"]; ok {
							message.WriteString(fmt.Sprintf("%v", value))
					}
			case "number", "boolean":
					if value, ok := argMap["value"]; ok {
							message.WriteString(fmt.Sprintf("%v", value))
					}
			case "object":
					if subtype, ok := argMap["subtype"].(string); ok {
							switch subtype {
							case "null":
									message.WriteString("null")
							case "array":
									if preview, ok := argMap["preview"].(map[string]interface{}); ok {
											message.WriteString(formatArray(preview))
									} else {
											message.WriteString(fmt.Sprintf("Array(%v)", argMap["description"]))
									}
							default:
									if objectID, ok := argMap["objectId"].(string); ok {
											props := getObjectProperties(ws, objectID)
											if props != nil {
													message.WriteString(formatDetailedObject(props))
											} else if preview, ok := argMap["preview"].(map[string]interface{}); ok {
													message.WriteString(formatObject(preview))
											} else {
													message.WriteString(argMap["description"].(string))
											}
									}
							}
					}
			}
	}

	return debugger.ConsoleMessage{
			Type:    params["type"].(string),
			Time:    time.Now(),
			Message: strings.TrimSpace(message.String()),
			URL:     getSourceURL(params),
	}
}

type responseChannel struct {
	ch      chan map[string]interface{}
	timeout time.Time
}

var (
	requestCounter   int64
	pendingRequests = make(map[int64]responseChannel)
	requestMutex    sync.RWMutex
)

func getObjectProperties(ws *websocket.Conn, objectID string) map[string]interface{} {
	reqID := atomic.AddInt64(&requestCounter, 1)
	responseChan := make(chan map[string]interface{}, 5)
	
	// Register request
	requestMutex.Lock()
	pendingRequests[reqID] = responseChannel{
		ch:      responseChan,
		timeout: time.Now().Add(30 * time.Second),
	}
	requestMutex.Unlock()

	// Cleanup on exit
	defer func() {
		requestMutex.Lock()
		delete(pendingRequests, reqID)
		requestMutex.Unlock()
	}()

	// Send request
	request := map[string]interface{}{
		"id":     reqID,
		"method": "Runtime.getProperties",
		"params": map[string]interface{}{
			"objectId":               objectID,
			"ownProperties":          true,
			"accessorPropertiesOnly": false,
			"generatePreview":        true,
		},
	}

	if err := ws.WriteJSON(request); err != nil {
		fmt.Printf("❌ Failed to send getProperties request: %v\n", err)
		return nil
	}

	fmt.Printf("📍 Requesting properties for object: %s (reqID: %d)\n", objectID, reqID)

	// Wait for response with timeout
	select {
	case response := <-responseChan:
		if result, ok := response["result"].(map[string]interface{}); ok {
			return result
		}
		return nil
	case <-time.After(30 * time.Second):
		fmt.Printf("⏰ Timeout waiting for response to reqID: %d\n", reqID)
		return nil
	}
}

func handleWebSocketMessage(data map[string]interface{}) {
	if id, ok := data["id"].(float64); ok {
		reqID := int64(id)
		requestMutex.RLock()
		if respChan, exists := pendingRequests[reqID]; exists {
			select {
			case respChan.ch <- data:
				fmt.Printf("✅ Sent response for reqID: %d\n", reqID)
			default:
				fmt.Printf("⚠️ Channel full for reqID: %d\n", reqID)
			}
		}
		requestMutex.RUnlock()
		return
	}

	// Log the full message for debugging
	if bytes, err := json.Marshal(data); err == nil {
		fmt.Printf("📍 Full message: %s\n", string(bytes))
	}
}

func formatDetailedObject(props map[string]interface{}) string {
	if props == nil {
		return "[object Object]"
	}

	var builder strings.Builder
	builder.WriteString("{")

	if result, ok := props["result"].([]interface{}); ok {
		for i, p := range result {
			prop := p.(map[string]interface{})
			if i > 0 {
				builder.WriteString(", ")
			}

			name := prop["name"].(string)
			if value, ok := prop["value"].(map[string]interface{}); ok {
				valueType := value["type"].(string)
				switch valueType {
				case "string":
					builder.WriteString(fmt.Sprintf("%s: %q", name, value["value"]))
				case "number", "boolean":
					builder.WriteString(fmt.Sprintf("%s: %v", name, value["value"]))
				case "object":
					if description, ok := value["description"].(string); ok {
						builder.WriteString(fmt.Sprintf("%s: %s", name, description))
					} else {
						builder.WriteString(fmt.Sprintf("%s: [object Object]", name))
					}
				}
			}
		}
	}

	builder.WriteString("}")
	return builder.String()
}

func formatObject(preview map[string]interface{}) string {
	var result strings.Builder
	result.WriteString("{")
	
	if properties, ok := preview["properties"].([]interface{}); ok {
		for i, p := range properties {
			prop := p.(map[string]interface{})
			if i > 0 {
				result.WriteString(", ")
			}
			name := prop["name"].(string)
			value := prop["value"].(string)
			
			// Handle nested object previews
			if valuePreview, ok := prop["valuePreview"].(map[string]interface{}); ok {
				value = formatObject(valuePreview)
			}
			
			result.WriteString(fmt.Sprintf("%s: %s", name, value))
		}
	}
	
	result.WriteString("}")
	return result.String()
}

func formatArray(preview map[string]interface{}) string {
	var result strings.Builder
	result.WriteString("[")
	
	if properties, ok := preview["properties"].([]interface{}); ok {
		for i, p := range properties {
			prop := p.(map[string]interface{})
			if i > 0 {
				result.WriteString(", ")
			}
			
			if valuePreview, ok := prop["valuePreview"].(map[string]interface{}); ok {
				result.WriteString(formatObject(valuePreview))
			} else {
				result.WriteString(prop["value"].(string))
			}
		}
	}
	
	result.WriteString("]")
	return result.String()
}

func getSourceURL(params map[string]interface{}) string {
	if stackTrace, ok := params["stackTrace"].(map[string]interface{}); ok {
		if frames, ok := stackTrace["callFrames"].([]interface{}); ok && len(frames) > 0 {
			if frame, ok := frames[0].(map[string]interface{}); ok {
				if url, ok := frame["url"].(string); ok {
					return url
				}
			}
		}
	}
	return ""
} 