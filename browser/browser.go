package browser

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/agios-sh/agios/output"
)

// Run dispatches browser subcommands.
func Run(args []string) {
	if len(args) == 0 {
		respondOverview()
		return
	}

	switch args[0] {
	case "open":
		headed := false
		for _, a := range args[1:] {
			if a == "--headed" {
				headed = true
			}
		}
		alreadyRunning, err := StartChrome(headed)
		if err != nil {
			emitError(err.Error(), "BROWSER_ERROR",
				"Run `agios browser status` to check browser state",
			)
			os.Exit(1)
		}
		if alreadyRunning {
			emitResult(map[string]any{
				"message": "Chrome is already running, using existing session",
				"help": []string{
					"Run `agios browser go <url>` to navigate to a page",
					"Run `agios browser quit` to stop the browser",
				},
			})
		} else {
			mode := "headless"
			if headed {
				mode = "headed"
			}
			emitResult(map[string]any{
				"message": "Chrome started (" + mode + ")",
				"help": []string{
					"Run `agios browser go <url>` to navigate to a page",
					"Run `agios browser quit` to stop the browser",
				},
			})
		}

	case "quit":
		if err := StopChrome(); err != nil {
			emitError(err.Error(), "BROWSER_ERROR",
				"Run `agios browser open` to start a new session",
			)
			os.Exit(1)
		}
		emitResult(map[string]any{
			"message": "Chrome stopped",
		})

	case "go":
		if len(args) < 2 {
			emitError("URL required", "INVALID_ARGS",
				"Usage: `agios browser go <url>`",
			)
			os.Exit(1)
		}
		sess, err := RequireSession()
		if err != nil {
			emitError(err.Error(), "NO_SESSION", "Run `agios browser open` first")
			os.Exit(1)
		}
		// NOTE: intentionally not calling sess.Cancel() — process exit closes
		// the WebSocket connection without sending Target.closeTarget, so the
		// Chrome tab stays alive for subsequent agios invocations.
		_ = sess.Cancel
		doGo(sess, args[1])

	case "page":
		actionsOnly := false
		for _, a := range args[1:] {
			if a == "--actions-only" {
				actionsOnly = true
			}
		}
		sess, err := RequireSession()
		if err != nil {
			emitError(err.Error(), "NO_SESSION", "Run `agios browser open` first")
			os.Exit(1)
		}
		// NOTE: intentionally not calling sess.Cancel() — process exit closes
		// the WebSocket connection without sending Target.closeTarget, so the
		// Chrome tab stays alive for subsequent agios invocations.
		_ = sess.Cancel
		doPage(sess, actionsOnly)

	case "click":
		if len(args) < 2 {
			emitError("Handle required", "INVALID_ARGS",
				"Usage: `agios browser click <handle>`",
				"Run `agios browser page` to see available handles",
			)
			os.Exit(1)
		}
		sess, err := RequireSession()
		if err != nil {
			emitError(err.Error(), "NO_SESSION", "Run `agios browser open` first")
			os.Exit(1)
		}
		// NOTE: intentionally not calling sess.Cancel() — process exit closes
		// the WebSocket connection without sending Target.closeTarget, so the
		// Chrome tab stays alive for subsequent agios invocations.
		_ = sess.Cancel
		doClick(sess, args[1])

	case "input":
		if len(args) < 3 {
			emitError("Handle and text required", "INVALID_ARGS",
				"Usage: `agios browser input <handle> <text>`",
				"Run `agios browser page` to see available handles",
			)
			os.Exit(1)
		}
		sess, err := RequireSession()
		if err != nil {
			emitError(err.Error(), "NO_SESSION", "Run `agios browser open` first")
			os.Exit(1)
		}
		// NOTE: intentionally not calling sess.Cancel() — process exit closes
		// the WebSocket connection without sending Target.closeTarget, so the
		// Chrome tab stays alive for subsequent agios invocations.
		_ = sess.Cancel
		text := strings.Join(args[2:], " ")
		doInput(sess, args[1], text)

	case "set":
		if len(args) < 3 {
			emitError("Handle and value required", "INVALID_ARGS",
				"Usage: `agios browser set <handle> <value>`",
				"Run `agios browser page` to see available handles",
			)
			os.Exit(1)
		}
		sess, err := RequireSession()
		if err != nil {
			emitError(err.Error(), "NO_SESSION", "Run `agios browser open` first")
			os.Exit(1)
		}
		// NOTE: intentionally not calling sess.Cancel() — process exit closes
		// the WebSocket connection without sending Target.closeTarget, so the
		// Chrome tab stays alive for subsequent agios invocations.
		_ = sess.Cancel
		value := strings.Join(args[2:], " ")
		doSet(sess, args[1], value)

	case "key":
		if len(args) < 2 {
			emitError("Key required", "INVALID_ARGS",
				"Usage: `agios browser key <key>`",
				"Example: `agios browser key Enter`",
			)
			os.Exit(1)
		}
		sess, err := RequireSession()
		if err != nil {
			emitError(err.Error(), "NO_SESSION", "Run `agios browser open` first")
			os.Exit(1)
		}
		// NOTE: intentionally not calling sess.Cancel() — process exit closes
		// the WebSocket connection without sending Target.closeTarget, so the
		// Chrome tab stays alive for subsequent agios invocations.
		_ = sess.Cancel
		doKey(sess, args[1])

	case "hover":
		if len(args) < 2 {
			emitError("Handle required", "INVALID_ARGS",
				"Usage: `agios browser hover <handle>`",
				"Run `agios browser page` to see available handles",
			)
			os.Exit(1)
		}
		sess, err := RequireSession()
		if err != nil {
			emitError(err.Error(), "NO_SESSION", "Run `agios browser open` first")
			os.Exit(1)
		}
		// NOTE: intentionally not calling sess.Cancel() — process exit closes
		// the WebSocket connection without sending Target.closeTarget, so the
		// Chrome tab stays alive for subsequent agios invocations.
		_ = sess.Cancel
		doHover(sess, args[1])

	case "scroll":
		target := ""
		if len(args) >= 2 {
			target = args[1]
		}
		sess, err := RequireSession()
		if err != nil {
			emitError(err.Error(), "NO_SESSION", "Run `agios browser open` first")
			os.Exit(1)
		}
		// NOTE: intentionally not calling sess.Cancel() — process exit closes
		// the WebSocket connection without sending Target.closeTarget, so the
		// Chrome tab stays alive for subsequent agios invocations.
		_ = sess.Cancel
		doScroll(sess, target)

	case "pick":
		if len(args) < 3 {
			emitError("Handle and value required", "INVALID_ARGS",
				"Usage: `agios browser pick <handle> <value>`",
				"Run `agios browser page` to see available handles",
			)
			os.Exit(1)
		}
		sess, err := RequireSession()
		if err != nil {
			emitError(err.Error(), "NO_SESSION", "Run `agios browser open` first")
			os.Exit(1)
		}
		// NOTE: intentionally not calling sess.Cancel() — process exit closes
		// the WebSocket connection without sending Target.closeTarget, so the
		// Chrome tab stays alive for subsequent agios invocations.
		_ = sess.Cancel
		value := strings.Join(args[2:], " ")
		doPick(sess, args[1], value)

	case "content":
		sess, err := RequireSession()
		if err != nil {
			emitError(err.Error(), "NO_SESSION", "Run `agios browser open` first")
			os.Exit(1)
		}
		// NOTE: intentionally not calling sess.Cancel() — process exit closes
		// the WebSocket connection without sending Target.closeTarget, so the
		// Chrome tab stays alive for subsequent agios invocations.
		_ = sess.Cancel
		doContent(sess)

	case "capture":
		outPath := ""
		for i, a := range args[1:] {
			if a == "-o" && i+2 < len(args) {
				outPath = args[i+2]
			}
		}
		sess, err := RequireSession()
		if err != nil {
			emitError(err.Error(), "NO_SESSION", "Run `agios browser open` first")
			os.Exit(1)
		}
		// NOTE: intentionally not calling sess.Cancel() — process exit closes
		// the WebSocket connection without sending Target.closeTarget, so the
		// Chrome tab stays alive for subsequent agios invocations.
		_ = sess.Cancel
		doCapture(sess, outPath)

	case "tabs":
		sub := ""
		tabArgs := []string{}
		if len(args) >= 2 {
			sub = args[1]
		}
		if len(args) >= 3 {
			tabArgs = args[2:]
		}
		sess, err := RequireSession()
		if err != nil {
			emitError(err.Error(), "NO_SESSION", "Run `agios browser open` first")
			os.Exit(1)
		}
		// NOTE: intentionally not calling sess.Cancel() — process exit closes
		// the WebSocket connection without sending Target.closeTarget, so the
		// Chrome tab stays alive for subsequent agios invocations.
		_ = sess.Cancel
		doTabs(sess, sub, tabArgs)

	case "run":
		if len(args) < 2 {
			emitError("JavaScript expression required", "INVALID_ARGS",
				"Usage: `agios browser run <js>`",
			)
			os.Exit(1)
		}
		sess, err := RequireSession()
		if err != nil {
			emitError(err.Error(), "NO_SESSION", "Run `agios browser open` first")
			os.Exit(1)
		}
		// NOTE: intentionally not calling sess.Cancel() — process exit closes
		// the WebSocket connection without sending Target.closeTarget, so the
		// Chrome tab stays alive for subsequent agios invocations.
		_ = sess.Cancel
		js := strings.Join(args[1:], " ")
		doRun(sess, js)

	case "status":
		respondStatus()

	case "help":
		respondHelp()

	case "peek":
		respondPeek()

	default:
		emitError("Unknown command: "+args[0], "UNKNOWN_COMMAND",
			"Run `agios browser help` to see available commands",
		)
		os.Exit(1)
	}
}

// emitResult outputs a result through the agios output pipeline.
func emitResult(v map[string]any) {
	data, err := output.Process(v)
	if err != nil {
		enc := json.NewEncoder(os.Stdout)
		enc.Encode(v)
		return
	}
	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
}

// emitError outputs an AIP-compliant error response.
func emitError(msg, code string, help ...string) {
	result := map[string]any{
		"error": msg,
		"code":  code,
	}
	if len(help) > 0 {
		result["help"] = help
	} else {
		result["help"] = []string{"Run `agios browser help` for usage information"}
	}
	emitResult(result)
}
