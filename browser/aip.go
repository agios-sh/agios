package browser

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// respondOverview shows the current browser state — like clicking Chrome in the dock.
// If Chrome is running, shows tabs. If not, shows how to start it.
func respondOverview() {
	sp, err := sessionPath()
	if err != nil {
		emitNotRunning()
		return
	}

	data, err := os.ReadFile(sp)
	if err != nil {
		emitNotRunning()
		return
	}

	var info sessionInfo
	if err := json.Unmarshal(data, &info); err != nil || !isAlive(info) {
		emitNotRunning()
		return
	}

	// Chrome is running — fetch tabs via HTTP endpoint
	type tabEntry struct {
		Index int    `json:"index"`
		Title string `json:"title"`
		URL   string `json:"url"`
	}

	var tabs []tabEntry
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json", info.Port))
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var targets []struct {
			Type  string `json:"type"`
			Title string `json:"title"`
			URL   string `json:"url"`
		}
		if json.Unmarshal(body, &targets) == nil {
			idx := 0
			for _, t := range targets {
				if t.Type == "page" {
					tabs = append(tabs, tabEntry{Index: idx, Title: t.Title, URL: t.URL})
					idx++
				}
			}
		}
	}

	// Find which tab is active (first tab by default)
	activeTab := 0
	if len(tabs) > 0 {
		activeTab = tabs[0].Index
	}

	emitResult(map[string]any{
		"tabs":       tabs,
		"active_tab": activeTab,
		"help": []string{
			"Run `agios browser go <url>` to navigate",
			"Run `agios browser page` to see page structure",
			"Run `agios browser quit` to stop the browser",
		},
	})
}

func emitNotRunning() {
	emitResult(map[string]any{
		"tabs": []any{},
		"help": []string{
			"Run `agios browser open` to start a headless browser",
			"Run `agios browser open --headed` for a visible window",
		},
	})
}

// respondStatus returns the AIP status response for the browser app.
func respondStatus() {
	status := "info"
	statusMsg := "No active browser session"

	if sess, err := Dial(); err == nil {
		sess.Cancel()
		status = "ok"
		statusMsg = "Chrome running (PID " + fmt.Sprintf("%d", sess.Info.PID) + ")"
	}

	emitResult(map[string]any{
		"name":        "browser",
		"description": "Built-in browser for web automation",
		"version":     "1.0.0",
		"status":      status,
		"status_msg":  statusMsg,
		"commands": []map[string]string{
			{"name": "open", "usage": "agios browser open [--headed]", "summary": "Start Chrome instance"},
			{"name": "quit", "usage": "agios browser quit", "summary": "Stop Chrome instance"},
			{"name": "go", "usage": "agios browser go <url>", "summary": "Navigate to URL"},
			{"name": "page", "usage": "agios browser page [--actions-only]", "summary": "Get accessibility tree with handles"},
			{"name": "click", "usage": "agios browser click <handle>", "summary": "Click element"},
			{"name": "input", "usage": "agios browser input <handle> <text>", "summary": "Type keystrokes into element"},
			{"name": "set", "usage": "agios browser set <handle> <value>", "summary": "Set input value directly"},
			{"name": "key", "usage": "agios browser key <key>", "summary": "Press key (Enter, Tab, etc.)"},
			{"name": "hover", "usage": "agios browser hover <handle>", "summary": "Hover over element"},
			{"name": "scroll", "usage": "agios browser scroll [<handle>|<pixels>]", "summary": "Scroll page or to element"},
			{"name": "pick", "usage": "agios browser pick <handle> <value>", "summary": "Select dropdown option"},
			{"name": "content", "usage": "agios browser content", "summary": "Extract page text"},
			{"name": "capture", "usage": "agios browser capture [-o path]", "summary": "Save screenshot, return path"},
			{"name": "tabs", "usage": "agios browser tabs [create|close|switch]", "summary": "Tab management"},
			{"name": "run", "usage": "agios browser run <js>", "summary": "Execute JavaScript"},
			{"name": "status", "usage": "agios browser status", "summary": "AIP status"},
			{"name": "help", "usage": "agios browser help", "summary": "AIP help"},
		},
	})
}

// respondHelp returns the AIP help response for the browser app.
func respondHelp() {
	emitResult(map[string]any{
		"name":  "browser",
		"usage": "agios browser <command> [args]",
		"description": "Built-in browser for web automation using Chrome DevTools Protocol. " +
			"Uses an accessibility-tree-first approach with stable @N element handles.",
		"commands": []map[string]string{
			{"name": "open", "usage": "agios browser open [--headed]", "summary": "Start a Chrome instance. Use --headed for a visible window."},
			{"name": "quit", "usage": "agios browser quit", "summary": "Stop the Chrome instance and clean up state files."},
			{"name": "go", "usage": "agios browser go <url>", "summary": "Navigate to the given URL and wait for the page to load."},
			{"name": "page", "usage": "agios browser page [--actions-only]", "summary": "Get the page's accessibility tree with @N handles. Use --actions-only to show only actionable elements (buttons, links, inputs, etc.)."},
			{"name": "click", "usage": "agios browser click <handle>", "summary": "Click the element identified by @N handle."},
			{"name": "input", "usage": "agios browser input <handle> <text>", "summary": "Focus the element and type keystrokes into it."},
			{"name": "set", "usage": "agios browser set <handle> <value>", "summary": "Set the value of an input element directly (clears existing value first)."},
			{"name": "key", "usage": "agios browser key <key>", "summary": "Press a key or key combination (e.g., Enter, Tab, Control+a)."},
			{"name": "hover", "usage": "agios browser hover <handle>", "summary": "Hover over the element identified by @N handle."},
			{"name": "scroll", "usage": "agios browser scroll [<handle>|<pixels>]", "summary": "Scroll to an element by handle, by pixel amount, or scroll down one viewport if no argument given."},
			{"name": "pick", "usage": "agios browser pick <handle> <value>", "summary": "Select an option from a dropdown/select element."},
			{"name": "content", "usage": "agios browser content", "summary": "Extract all visible text from the page."},
			{"name": "capture", "usage": "agios browser capture [-o path]", "summary": "Take a screenshot and save it. Returns the file path."},
			{"name": "tabs", "usage": "agios browser tabs [create|close|switch] [url|index]", "summary": "List tabs, or create/close/switch tabs."},
			{"name": "run", "usage": "agios browser run <js>", "summary": "Execute JavaScript in the page and return the result."},
			{"name": "status", "usage": "agios browser status", "summary": "Show browser status (AIP protocol)."},
			{"name": "help", "usage": "agios browser help", "summary": "Show this help message (AIP protocol)."},
			{"name": "peek", "usage": "agios browser peek", "summary": "Show a snapshot of the browser state (AIP protocol)."},
		},
		"help": []string{
			"Run `agios browser open` to start a browser session",
			"Run `agios browser go <url>` to navigate to a page",
			"Run `agios browser page` to see the page structure with @N handles",
			"Use @N handles with click, input, set, hover, pick commands",
		},
	})
}

// PeekData returns a lightweight snapshot of the browser state for the home command.
func PeekData() map[string]any {
	sp, err := sessionPath()
	if err != nil {
		return nil
	}

	data, err := os.ReadFile(sp)
	if err != nil {
		return nil
	}

	var info sessionInfo
	if err := json.Unmarshal(data, &info); err != nil || !isAlive(info) {
		return nil
	}

	// Chrome is running — fetch tabs
	type tabEntry struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	}

	var tabs []tabEntry
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json", info.Port))
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var targets []struct {
			Type  string `json:"type"`
			Title string `json:"title"`
			URL   string `json:"url"`
		}
		if json.Unmarshal(body, &targets) == nil {
			for _, t := range targets {
				if t.Type == "page" {
					tabs = append(tabs, tabEntry{Title: t.Title, URL: t.URL})
				}
			}
		}
	}

	if tabs == nil {
		return nil
	}

	return map[string]any{
		"tabs":       tabs,
		"active_tab": 0,
	}
}

// respondPeek returns a lightweight snapshot of the browser state.
func respondPeek() {
	data := PeekData()
	if data == nil {
		data = map[string]any{}
	}
	emitResult(data)
}
