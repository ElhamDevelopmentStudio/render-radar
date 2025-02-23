package debugger

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

// DebugRequest represents the incoming request to debug specific URLs
type DebugRequest struct {
    URLs []string `json:"urls"`
}

// DebugResponse represents the debugging results for multiple targets
type DebugResponse struct {
    Results map[string][]string `json:"results"` // URL -> debug logs mapping
    Errors  map[string]string   `json:"errors"`  // URL -> error message mapping
}

// DebugMessage represents a processed debugging message
type DebugMessage struct {
    Type    string
    Message string
} 