package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

// doRun executes JavaScript in the page and returns the result.
func doRun(sess *Session, js string) {
	ctx, cancel := context.WithTimeout(sess.Ctx, 10*time.Second)
	defer cancel()

	var result any
	err := chromedp.Run(ctx, chromedp.Evaluate(js, &result))
	if err != nil {
		emitError(fmt.Sprintf("JavaScript execution failed: %v", err), "RUN_ERROR",
			"Check your JavaScript expression and try again",
		)
		os.Exit(1)
	}

	// Try to present the result in a structured way
	output := map[string]any{
		"help": []string{
			"Run `agios browser page` to see the current page state",
		},
	}

	if result == nil {
		output["result"] = nil
	} else {
		// Check if result is already a JSON-like structure
		switch v := result.(type) {
		case map[string]any:
			output["result"] = v
		case []any:
			output["result"] = v
		case string:
			// Try to parse as JSON
			var parsed any
			if err := json.Unmarshal([]byte(v), &parsed); err == nil {
				output["result"] = parsed
			} else {
				output["result"] = v
			}
		default:
			output["result"] = v
		}
	}

	emitResult(output)
}
