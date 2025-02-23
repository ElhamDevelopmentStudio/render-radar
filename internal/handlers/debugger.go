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
		Results: make(map[string]debugger.PageResults),
		Errors:  make(map[string]string),
	}

	// Create a channel to collect results from all goroutines
	resultsChan := make(chan struct {
		url    string
		logs   []debugger.ConsoleMessage
		err    error
	}, len(req.URLs))

	// Start a goroutine for each target
	for url, target := range targets {
		go func(url string, target *debugger.DebuggingTarget) {
			logs, err := debugTarget(target)
			resultsChan <- struct {
				url    string
				logs   []debugger.ConsoleMessage
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
			response.Results[result.url] = categorizeMessages(result.logs)
		}
	}

	return c.JSON(response)
}

func enableDebugging(ws *websocket.Conn) error {
	// Enable Console domain
	enableConsole := map[string]interface{}{
		"id":     1,
		"method": "Console.enable",
	}
	
	// Enable Runtime domain with console API
	enableRuntime := map[string]interface{}{
		"id":     2,
		"method": "Runtime.enable",
		"params": map[string]interface{}{
			"notifyOnConsoleAPICalled": true,
			"notifyOnExceptionThrown":  true,
		},
	}

	// Enable Network domain
	enableNetwork := map[string]interface{}{
		"id":     3,
		"method": "Network.enable",
		"params": map[string]interface{}{
			"maxTotalBufferSize":    10000000,
			"maxResourceBufferSize": 5000000,
		},
	}

	for _, command := range []map[string]interface{}{enableConsole, enableRuntime, enableNetwork} {
		if err := ws.WriteJSON(command); err != nil {
			return fmt.Errorf("failed to enable debugging feature: %v", err)
		}
	}
	return nil
}

func captureDebugMessages(ws *websocket.Conn) []debugger.ConsoleMessage {
	messages := []debugger.ConsoleMessage{}
	timeout := time.After(30 * time.Second)
	messageChannel := make(chan []byte)

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

			if method, ok := data["method"].(string); ok {
				switch method {
				case "Console.messageAdded":
					msg := parseConsoleMessage(data)
					messages = append(messages, msg)

				case "Runtime.consoleAPICalled":
					msg := parseRuntimeConsole(data)
					messages = append(messages, msg)

				case "Runtime.exceptionThrown":
					msg := parseException(data)
					messages = append(messages, msg)

				case "Network.requestWillBeSent":
					msg := parseNetworkRequest(data)
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
		Network: make([]debugger.ConsoleMessage, 0),
	}

	for _, msg := range messages {
		switch msg.Type {
		case "error", "exception":
			results.Errors = append(results.Errors, msg)
		case "network":
			results.Network = append(results.Network, msg)
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
		Line:    int(message["line"].(float64)),
		Column:  int(message["column"].(float64)),
	}
}

func parseRuntimeConsole(data map[string]interface{}) debugger.ConsoleMessage {
	params, ok := data["params"].(map[string]interface{})
	if !ok {
		return debugger.ConsoleMessage{}
	}

	args := params["args"].([]interface{})
	var messageData []interface{}
	for _, arg := range args {
		argMap := arg.(map[string]interface{})
		if preview, ok := argMap["preview"].(map[string]interface{}); ok {
			messageData = append(messageData, preview)
		} else {
			messageData = append(messageData, argMap["value"])
		}
	}

	return debugger.ConsoleMessage{
		Type:    params["type"].(string),
		Time:    time.Now(),
		Data:    messageData,
		Stack:   getStackTrace(params),
	}
}

func parseException(data map[string]interface{}) debugger.ConsoleMessage {
	params, ok := data["params"].(map[string]interface{})
	if !ok {
		return debugger.ConsoleMessage{}
	}

	exceptionDetails, ok := params["exceptionDetails"].(map[string]interface{})
	if !ok {
		return debugger.ConsoleMessage{}
	}

	var message string
	if text, ok := exceptionDetails["text"].(string); ok {
		message = text
	}
	if exception, ok := exceptionDetails["exception"].(map[string]interface{}); ok {
		if description, ok := exception["description"].(string); ok {
			message = description
		}
	}

	return debugger.ConsoleMessage{
		Type:    "exception",
		Time:    time.Now(),
		Message: message,
		Stack:   getStackTrace(exceptionDetails),
	}
}

func parseNetworkRequest(data map[string]interface{}) debugger.ConsoleMessage {
	params, ok := data["params"].(map[string]interface{})
	if !ok {
		return debugger.ConsoleMessage{}
	}

	request, ok := params["request"].(map[string]interface{})
	if !ok {
		return debugger.ConsoleMessage{}
	}

	return debugger.ConsoleMessage{
		Type:    "network",
		Time:    time.Now(),
		Message: fmt.Sprintf("%v %v", request["method"], request["url"]),
		Data:    request,
	}
}

func getStackTrace(data map[string]interface{}) string {
	if stackTrace, ok := data["stackTrace"].(map[string]interface{}); ok {
		if callFrames, ok := stackTrace["callFrames"].([]interface{}); ok {
			var stack string
			for _, frame := range callFrames {
				if frameMap, ok := frame.(map[string]interface{}); ok {
					stack += fmt.Sprintf("    at %s (%s:%v:%v)\n",
						frameMap["functionName"],
						frameMap["url"],
						frameMap["lineNumber"],
						frameMap["columnNumber"])
				}
			}
			return stack
		}
	}
	return ""
} 