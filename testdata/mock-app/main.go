// mock-app is a test binary that implements the full AIP protocol.
// It is configurable via environment variables to simulate various scenarios:
//
//	MOCK_STATUS_CODE   - exit code for status command (default: 0)
//	MOCK_ERROR         - if set, return this as an error message
//	MOCK_INVALID_JSON  - if "1", return invalid (non-JSON) output
//	MOCK_SLOW          - duration to sleep before responding (e.g. "10s")
//	MOCK_PROGRESS      - if "1", emit progress lines before result
//	MOCK_PEEK          - JSON object to return as peek data
//	MOCK_EMPTY_PEEK    - if "1", return empty peek object
//	MOCK_LARGE_VALUE   - if "1", include a large string value in output (>4096 chars)
//	MOCK_STDIN_ECHO    - if "1", read stdin and include it in the response
//	MOCK_DESCRIPTION   - custom app description (default: "Mock AIP app for testing")
//	MOCK_VERSION       - custom version string (default: "1.0.0")
//	MOCK_USER          - custom user string (default: "testuser@example.com")
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		writeJSON(map[string]any{
			"error": "No command provided",
			"code":  "MISSING_COMMAND",
		})
		os.Exit(1)
	}

	command := os.Args[1]

	// Check for invalid JSON mode (applies to all commands)
	if os.Getenv("MOCK_INVALID_JSON") == "1" {
		fmt.Println("this is not valid json output")
		return
	}

	// Check for slow mode
	if dur := os.Getenv("MOCK_SLOW"); dur != "" {
		d, err := time.ParseDuration(dur)
		if err == nil {
			// If progress is enabled, emit progress lines while sleeping
			if os.Getenv("MOCK_PROGRESS") == "1" {
				emitProgressWithDelay(d)
			} else {
				time.Sleep(d)
			}
		}
	} else if os.Getenv("MOCK_PROGRESS") == "1" {
		// Emit progress lines without delay
		writeJSON(map[string]any{"progress": map[string]any{"message": "Starting...", "percent": 0}})
		writeJSON(map[string]any{"progress": map[string]any{"message": "Working...", "percent": 50}})
	}

	// Check for error mode
	if errMsg := os.Getenv("MOCK_ERROR"); errMsg != "" {
		exitCode := 1
		if code := os.Getenv("MOCK_STATUS_CODE"); code != "" {
			fmt.Sscanf(code, "%d", &exitCode)
		}
		writeJSON(map[string]any{
			"error":  errMsg,
			"status": "error",
		})
		os.Exit(exitCode)
	}

	switch command {
	case "status":
		handleStatus()
	case "help":
		handleHelp()
	case "peek":
		handlePeek()
	case "list":
		handleList()
	case "get":
		handleGet()
	case "create":
		handleCreate()
	default:
		writeJSON(map[string]any{
			"error": fmt.Sprintf("Unknown command: %s", command),
			"code":  "UNKNOWN_COMMAND",
		})
		os.Exit(1)
	}
}

func handleStatus() {
	description := envOr("MOCK_DESCRIPTION", "Mock AIP app for testing")
	version := envOr("MOCK_VERSION", "1.0.0")
	user := envOr("MOCK_USER", "testuser@example.com")

	result := map[string]any{
		"name":        "mock-app",
		"description": description,
		"version":     version,
		"status":      "ok",
		"user":        user,
		"commands": []map[string]string{
			{"name": "list", "description": "List items"},
			{"name": "get", "description": "Get item details"},
			{"name": "create", "description": "Create a new item"},
			{"name": "peek", "description": "Get peek data"},
		},
	}

	writeJSON(result)
}

func handleHelp() {
	writeJSON(map[string]any{
		"usage": "mock-app <command> [args]",
		"commands": []map[string]string{
			{"name": "status", "description": "App status and health", "usage": "mock-app status"},
			{"name": "list", "description": "List items", "usage": "mock-app list [--status <status>]"},
			{"name": "get", "description": "Get item details", "usage": "mock-app get <id>"},
			{"name": "create", "description": "Create a new item", "usage": "mock-app create --title <title>"},
			{"name": "peek", "description": "Get peek data", "usage": "mock-app peek"},
		},
	})
}

func handlePeek() {
	if os.Getenv("MOCK_EMPTY_PEEK") == "1" {
		writeJSON(map[string]any{})
		return
	}

	if peekJSON := os.Getenv("MOCK_PEEK"); peekJSON != "" {
		fmt.Println(peekJSON)
		return
	}

	// Default peek response — free-form JSON
	writeJSON(map[string]any{
		"recent_activity": []map[string]any{
			{"type": "pr_approved", "title": "PR #123 approved by alice"},
			{"type": "review_requested", "title": "Review requested on PR #456"},
			{"type": "ci_passed", "title": "CI passed on PR #789"},
		},
		"unread_count": 3,
	})
}

func handleList() {
	result := map[string]any{
		"items": []map[string]any{
			{"id": "1", "title": "First item", "status": "open"},
			{"id": "2", "title": "Second item", "status": "closed"},
			{"id": "3", "title": "Third item", "status": "open"},
		},
		"help": []string{
			"Run `agios mock-app get <id>` to view details",
			"Run `agios mock-app list --status done` to see completed items",
		},
	}

	// Check for large value mode
	if os.Getenv("MOCK_LARGE_VALUE") == "1" {
		result["large_field"] = strings.Repeat("A", 5000)
	}

	// Check for stdin echo mode
	if os.Getenv("MOCK_STDIN_ECHO") == "1" {
		data, err := io.ReadAll(os.Stdin)
		if err == nil && len(data) > 0 {
			result["stdin_content"] = string(data)
		}
	}

	writeJSON(result)
}

func handleGet() {
	id := "unknown"
	if len(os.Args) > 2 {
		id = os.Args[2]
	}

	writeJSON(map[string]any{
		"id":     id,
		"title":  fmt.Sprintf("Item %s", id),
		"status": "open",
		"body":   "This is the item body content.",
		"help": []string{
			"Run `agios mock-app list` to see all items",
		},
	})
}

func handleCreate() {
	// Read stdin for content body
	var content string
	if os.Getenv("MOCK_STDIN_ECHO") == "1" {
		data, err := io.ReadAll(os.Stdin)
		if err == nil {
			content = string(data)
		}
	}

	result := map[string]any{
		"id":      "new-1",
		"title":   "New item",
		"status":  "open",
		"message": "Item created",
		"help": []string{
			"Run `agios mock-app get new-1` to view the created item",
		},
	}
	if content != "" {
		result["body"] = content
	}

	writeJSON(result)
}

func emitProgressWithDelay(total time.Duration) {
	steps := 3
	interval := total / time.Duration(steps)

	for i := 1; i <= steps; i++ {
		time.Sleep(interval)
		pct := (i * 100) / steps
		writeJSON(map[string]any{
			"progress": map[string]any{
				"message": fmt.Sprintf("Step %d/%d...", i, steps),
				"percent": pct,
			},
		})
	}
}

func writeJSON(v any) {
	data, _ := json.Marshal(v)
	fmt.Println(string(data))
}

func envOr(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
