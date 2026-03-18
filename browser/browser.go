package browser

import (
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
		sess := requireSessionOrExit()
		doGo(sess, args[1])

	case "page":
		actionsOnly := false
		for _, a := range args[1:] {
			if a == "--actions-only" {
				actionsOnly = true
			}
		}
		sess := requireSessionOrExit()
		doPage(sess, actionsOnly)

	case "click":
		if len(args) < 2 {
			emitError("Handle required", "INVALID_ARGS",
				"Usage: `agios browser click <handle>`",
				"Run `agios browser page` to see available handles",
			)
			os.Exit(1)
		}
		sess := requireSessionOrExit()
		doClick(sess, args[1])

	case "input":
		if len(args) < 3 {
			emitError("Handle and text required", "INVALID_ARGS",
				"Usage: `agios browser input <handle> <text>`",
				"Run `agios browser page` to see available handles",
			)
			os.Exit(1)
		}
		sess := requireSessionOrExit()
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
		sess := requireSessionOrExit()
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
		sess := requireSessionOrExit()
		doKey(sess, args[1])

	case "hover":
		if len(args) < 2 {
			emitError("Handle required", "INVALID_ARGS",
				"Usage: `agios browser hover <handle>`",
				"Run `agios browser page` to see available handles",
			)
			os.Exit(1)
		}
		sess := requireSessionOrExit()
		doHover(sess, args[1])

	case "scroll":
		target := ""
		if len(args) >= 2 {
			target = args[1]
		}
		sess := requireSessionOrExit()
		doScroll(sess, target)

	case "pick":
		if len(args) < 3 {
			emitError("Handle and value required", "INVALID_ARGS",
				"Usage: `agios browser pick <handle> <value>`",
				"Run `agios browser page` to see available handles",
			)
			os.Exit(1)
		}
		sess := requireSessionOrExit()
		value := strings.Join(args[2:], " ")
		doPick(sess, args[1], value)

	case "content":
		sess := requireSessionOrExit()
		doContent(sess)

	case "capture":
		outPath := ""
		for i, a := range args[1:] {
			if a == "-o" && i+2 < len(args) {
				outPath = args[i+2]
			}
		}
		sess := requireSessionOrExit()
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
		sess := requireSessionOrExit()
		doTabs(sess, sub, tabArgs)

	case "run":
		if len(args) < 2 {
			emitError("JavaScript expression required", "INVALID_ARGS",
				"Usage: `agios browser run <js>`",
			)
			os.Exit(1)
		}
		sess := requireSessionOrExit()
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

const defaultHelp = "Run `agios browser help` for usage information"

// requireSessionOrExit acquires a browser session, exiting on failure.
// The returned cancel func is intentionally unused — process exit closes the
// WebSocket without killing the Chrome tab.
func requireSessionOrExit() *Session {
	sess, err := RequireSession()
	if err != nil {
		emitError(err.Error(), "NO_SESSION", "Run `agios browser open` first")
		os.Exit(1)
	}
	_ = sess.Cancel
	return sess
}

func emitResult(v map[string]any) { output.EmitResult(v) }

func emitError(msg, code string, help ...string) {
	output.EmitError(msg, code, defaultHelp, help...)
}
