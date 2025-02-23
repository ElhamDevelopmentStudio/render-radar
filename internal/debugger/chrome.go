package debugger

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ChromeDebugger handles communication with Chrome's debugging protocol
type ChromeDebugger struct {
	debugURL string
}

// NewChromeDebugger creates a new ChromeDebugger instance
func NewChromeDebugger() *ChromeDebugger {
	return &ChromeDebugger{
		debugURL: ChromeDebuggerURL,
	}
}

// GetDebuggingTargets now accepts URLs to filter
func (c *ChromeDebugger) GetDebuggingTargets(urls []string) (map[string]*DebuggingTarget, error) {
	fmt.Println("Fetching debugging targets...")
	resp, err := http.Get(c.debugURL)
	if err != nil {
		return nil, fmt.Errorf("error getting debug targets: %v", err)
	}
	defer resp.Body.Close()

	var targets []DebuggingTarget
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return nil, fmt.Errorf("error decoding targets: %v", err)
	}

	// Create a map of URL -> DebuggingTarget
	urlTargets := make(map[string]*DebuggingTarget)
	for _, url := range urls {
		for i, target := range targets {
			if target.Type == "page" && strings.Contains(target.URL, url) {
				fmt.Printf("Found target for %s: %s\n", url, target.WebSocketDebuggerUrl)
				urlTargets[url] = &targets[i]
				break
			}
		}
	}

	return urlTargets, nil
} 