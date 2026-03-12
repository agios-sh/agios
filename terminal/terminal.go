package terminal

import (
	"os"
	"strconv"
	"strings"

	"github.com/agios-sh/agios/output"
)

// Run dispatches terminal subcommands.
func Run(args []string) {
	if len(args) == 0 {
		respondOverview()
		return
	}

	switch args[0] {
	case "start":
		name := ""
		shell := ""
		dir := ""
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--name":
				if i+1 < len(args) {
					i++
					name = args[i]
				}
			case "--shell":
				if i+1 < len(args) {
					i++
					shell = args[i]
				}
			case "--dir":
				if i+1 < len(args) {
					i++
					dir = args[i]
				}
			}
		}
		doStart(name, shell, dir)

	case "send":
		if len(args) < 2 {
			emitError("Input required", "INVALID_ARGS",
				"Usage: `agios terminal send <input> [--session <id>] [--timeout <ms>] [--raw]`",
			)
			os.Exit(1)
		}
		sessionID := 0
		timeoutMs := 5000
		raw := false
		// Collect non-flag arguments as input
		var inputParts []string
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--session":
				if i+1 < len(args) {
					i++
					sessionID, _ = strconv.Atoi(args[i])
				}
			case "--timeout":
				if i+1 < len(args) {
					i++
					timeoutMs, _ = strconv.Atoi(args[i])
				}
			case "--raw":
				raw = true
			default:
				inputParts = append(inputParts, args[i])
			}
		}
		input := strings.Join(inputParts, " ")
		if input == "" {
			emitError("Input required", "INVALID_ARGS",
				"Usage: `agios terminal send <input>`",
			)
			os.Exit(1)
		}
		doSend(sessionID, input, timeoutMs, raw)

	case "read":
		sessionID := 0
		for i := 1; i < len(args); i++ {
			if args[i] == "--session" && i+1 < len(args) {
				i++
				sessionID, _ = strconv.Atoi(args[i])
			}
		}
		doRead(sessionID)

	case "switch":
		sessionID := 0
		for i := 1; i < len(args); i++ {
			if args[i] == "--session" && i+1 < len(args) {
				i++
				sessionID, _ = strconv.Atoi(args[i])
			}
		}
		if sessionID == 0 {
			emitError("Session ID required", "INVALID_ARGS",
				"Usage: `agios terminal switch --session <id>`",
			)
			os.Exit(1)
		}
		doSwitch(sessionID)

	case "kill":
		sessionID := 0
		for i := 1; i < len(args); i++ {
			if args[i] == "--session" && i+1 < len(args) {
				i++
				sessionID, _ = strconv.Atoi(args[i])
			}
		}
		doKill(sessionID)

	case "resize":
		sessionID := 0
		rows := DefaultRows
		cols := DefaultCols
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--session":
				if i+1 < len(args) {
					i++
					sessionID, _ = strconv.Atoi(args[i])
				}
			case "--rows":
				if i+1 < len(args) {
					i++
					rows, _ = strconv.Atoi(args[i])
				}
			case "--cols":
				if i+1 < len(args) {
					i++
					cols, _ = strconv.Atoi(args[i])
				}
			}
		}
		doResize(sessionID, rows, cols)

	case "quit":
		doQuit()

	case "status":
		respondStatus()

	case "help":
		respondHelp()

	case "peek":
		respondPeek()

	default:
		emitError("Unknown command: "+args[0], "UNKNOWN_COMMAND",
			"Run `agios terminal help` to see available commands",
		)
		os.Exit(1)
	}
}

// RunServer starts the terminal server daemon. Called via --terminal-server flag.
func RunServer() {
	runServer()
}

const defaultHelp = "Run `agios terminal help` for usage information"

func emitResult(v map[string]any) { output.EmitResult(v) }

func emitError(msg, code string, help ...string) {
	output.EmitError(msg, code, defaultHelp, help...)
}

// respondStatus returns the AIP status response for the terminal app.
func respondStatus() {
	status := "info"
	statusMsg := "No active terminal server"

	info, err := loadServerInfo()
	if err == nil && isServerAlive(info) {
		status = "ok"
		statusMsg = "Terminal server running (PID " + strconv.Itoa(info.PID) + ")"
	}

	emitResult(map[string]any{
		"name":        "terminal",
		"description": "Built-in terminal for interactive shell sessions",
		"version":     "1.0.0",
		"status":      status,
		"status_msg":  statusMsg,
		"commands": []map[string]string{
			{"name": "start", "usage": "agios terminal start [--name <n>] [--shell <s>] [--dir <d>]", "summary": "Start a new PTY session"},
			{"name": "send", "usage": "agios terminal send <input> [--session <id>] [--timeout <ms>] [--raw]", "summary": "Send input and return output"},
			{"name": "read", "usage": "agios terminal read [--session <id>]", "summary": "Read current screen content"},
			{"name": "switch", "usage": "agios terminal switch --session <id>", "summary": "Switch active session"},
			{"name": "kill", "usage": "agios terminal kill [--session <id>]", "summary": "Kill a session"},
			{"name": "resize", "usage": "agios terminal resize --rows <r> --cols <c> [--session <id>]", "summary": "Resize PTY"},
			{"name": "quit", "usage": "agios terminal quit", "summary": "Kill all sessions, stop server"},
			{"name": "status", "usage": "agios terminal status", "summary": "AIP status"},
			{"name": "help", "usage": "agios terminal help", "summary": "AIP help"},
		},
	})
}

// respondHelp returns the AIP help response for the terminal app.
func respondHelp() {
	emitResult(map[string]any{
		"name":  "terminal",
		"usage": "agios terminal <command> [args]",
		"description": "Built-in terminal for interactive shell sessions. " +
			"Manages long-running PTY sessions that persist across CLI invocations.",
		"commands": []map[string]string{
			{"name": "start", "usage": "agios terminal start [--name <n>] [--shell <s>] [--dir <d>]", "summary": "Start a new PTY session. Uses $SHELL or /bin/sh by default."},
			{"name": "send", "usage": "agios terminal send <input> [--session <id>] [--timeout <ms>] [--raw]", "summary": "Send input to the session and wait for output. Appends newline by default. Use --raw to send bytes as-is (e.g., \\x03 for Ctrl+C)."},
			{"name": "read", "usage": "agios terminal read [--session <id>]", "summary": "Read current screen content."},
			{"name": "switch", "usage": "agios terminal switch --session <id>", "summary": "Switch the active session. Subsequent commands without --session target this session."},
			{"name": "kill", "usage": "agios terminal kill [--session <id>]", "summary": "Kill a session. Uses active session if --session is omitted."},
			{"name": "resize", "usage": "agios terminal resize --rows <r> --cols <c> [--session <id>]", "summary": "Resize the PTY window."},
			{"name": "quit", "usage": "agios terminal quit", "summary": "Kill all sessions and stop the terminal server."},
			{"name": "status", "usage": "agios terminal status", "summary": "Show terminal status (AIP protocol)."},
			{"name": "help", "usage": "agios terminal help", "summary": "Show this help message (AIP protocol)."},
			{"name": "peek", "usage": "agios terminal peek", "summary": "Show a snapshot of terminal state (AIP protocol)."},
		},
		"help": []string{
			"Run `agios terminal start` to create a new session",
			"Run `agios terminal send <input>` to send a command",
			"Run `agios terminal read` to see the current screen",
		},
	})
}

// PeekData returns a snapshot of active terminal sessions for the home command.
func PeekData() map[string]any {
	info, err := loadServerInfo()
	if err != nil || !isServerAlive(info) {
		return nil
	}

	resp, err := sendCommand(Request{Command: "list"})
	if err != nil || len(resp.Sessions) == 0 {
		return nil
	}

	return map[string]any{
		"sessions":  resp.Sessions,
		"active_id": resp.ActiveID,
	}
}

// respondPeek returns a snapshot of active terminal sessions.
func respondPeek() {
	data := PeekData()
	if data == nil {
		data = map[string]any{}
	}
	emitResult(data)
}
