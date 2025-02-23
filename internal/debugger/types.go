package debugger

import (
	"time"
)

// Chrome DevTools Protocol (CDP) URL
const ChromeDebuggerURL = "http://localhost:9222/json"

// DebuggingTarget represents a Chrome debugging target
type DebuggingTarget struct {
    ID                   string `json:"id"`
    Type                 string `json:"type"`
    Title                string `json:"title"`
    URL                  string `json:"url"`
    WebSocketDebuggerUrl string `json:"webSocketDebuggerUrl"`
}

// ConsoleMessage represents a structured console message
type ConsoleMessage struct {
    Type    string    `json:"type"`    // log, warn, error, info
    Time    time.Time `json:"time"`
    Message string    `json:"message"`
    URL     string    `json:"url,omitempty"`
}

// PageResults contains categorized messages for a single page
type PageResults struct {
    Console []ConsoleMessage `json:"console"`
    Errors  []ConsoleMessage `json:"errors"`
}

// DebugRequest represents the incoming request to debug specific URLs
type DebugRequest struct {
    URLs []string `json:"urls"`
}

// DebugResponse represents the debugging results for multiple targets
type DebugResponse struct {
    Results map[string]PageResults `json:"results"` // URL -> results mapping
    Errors  map[string]string      `json:"errors"`  // URL -> error message mapping
}