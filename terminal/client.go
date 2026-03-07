package terminal

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// sendCommand sends a request to the terminal server and returns the response.
func sendCommand(req Request) (Response, error) {
	info, err := loadServerInfo()
	if err != nil {
		return Response{}, fmt.Errorf("no terminal server running")
	}

	conn, err := net.DialTimeout("unix", info.Socket, 2*time.Second)
	if err != nil {
		return Response{}, fmt.Errorf("connecting to terminal server: %w", err)
	}
	defer conn.Close()

	// Set deadline for the entire exchange
	timeout := 30 * time.Second
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs+5000) * time.Millisecond
	}
	conn.SetDeadline(time.Now().Add(timeout))

	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		return Response{}, fmt.Errorf("sending request: %w", err)
	}

	var resp Response
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&resp); err != nil {
		return Response{}, fmt.Errorf("reading response: %w", err)
	}

	return resp, nil
}

// requireServer ensures the terminal server is running, starting it if needed.
func requireServer() error {
	info, err := loadServerInfo()
	if err == nil && isServerAlive(info) {
		return nil
	}

	// Server not running — start it
	_, err = StartServer()
	return err
}

// doStart handles the "terminal start" command.
func doStart(name, shell, dir string) {
	if err := requireServer(); err != nil {
		emitError("Failed to start terminal server: "+err.Error(), "SERVER_ERROR",
			"Check permissions on ~/.agios/terminal/",
		)
		os.Exit(1)
	}

	resp, err := sendCommand(Request{
		Command: "start",
		Name:    name,
		Shell:   shell,
		Dir:     dir,
	})
	if err != nil {
		emitError("Failed to create session: "+err.Error(), "SESSION_ERROR",
			"Run `agios terminal status` to check server state",
		)
		os.Exit(1)
	}
	if !resp.OK {
		emitError(resp.Error, "SESSION_ERROR",
			"Run `agios terminal status` to check server state",
		)
		os.Exit(1)
	}

	emitResult(map[string]any{
		"message":    "Session started",
		"session_id": resp.SessionID,
		"help": []string{
			"Run `agios terminal read` to see the current screen",
		},
	})
}

// doSend handles the "terminal send" command.
func doSend(sessionID int, input string, timeoutMs int, raw bool) {
	if err := requireServer(); err != nil {
		emitError("Failed to start terminal server: "+err.Error(), "SERVER_ERROR")
		os.Exit(1)
	}

	// Auto-create session if none exists
	resp, err := sendCommand(Request{Command: "list"})
	if err != nil {
		emitError("Failed to query sessions: "+err.Error(), "SERVER_ERROR")
		os.Exit(1)
	}
	if len(resp.Sessions) == 0 {
		// Auto-start a default session
		startResp, err := sendCommand(Request{Command: "start"})
		if err != nil || !startResp.OK {
			errMsg := "failed to auto-start session"
			if err != nil {
				errMsg = err.Error()
			} else if !startResp.OK {
				errMsg = startResp.Error
			}
			emitError(errMsg, "SESSION_ERROR",
				"Run `agios terminal start` to create a session manually",
			)
			os.Exit(1)
		}
		sessionID = startResp.SessionID
	}

	// Process input: append newline unless raw
	sendInput := input
	if !raw {
		sendInput = input + "\r"
	} else {
		// Process escape sequences in raw mode
		sendInput = processEscapes(input)
	}

	resp, err = sendCommand(Request{
		Command:   "send",
		SessionID: sessionID,
		Input:     sendInput,
		TimeoutMs: timeoutMs,
	})
	if err != nil {
		emitError("Failed to send input: "+err.Error(), "SEND_ERROR",
			"Run `agios terminal read` to see the current screen",
		)
		os.Exit(1)
	}
	if !resp.OK {
		emitError(resp.Error, "SEND_ERROR",
			"Run `agios terminal read` to see the current screen",
		)
		os.Exit(1)
	}

	result := map[string]any{
		"output":     formatScreen(resp.Output, resp.CursorRow, resp.CursorCol),
		"cursor":     fmt.Sprintf("%d:%d", resp.CursorRow, resp.CursorCol),
		"session_id": resp.SessionID,
	}
	if resp.Exited {
		result["exited"] = true
	}
	result["help"] = []string{
		"Run `agios terminal send <input>` to send another command",
		"Run `agios terminal read` to read more output",
	}

	emitResult(result)
}

// doRead handles the "terminal read" command.
func doRead(sessionID int) {
	if err := requireServer(); err != nil {
		emitError("No terminal server running", "SERVER_ERROR",
			"Run `agios terminal start` to create a session",
		)
		os.Exit(1)
	}

	resp, err := sendCommand(Request{
		Command:   "read",
		SessionID: sessionID,
	})
	if err != nil {
		emitError("Failed to read output: "+err.Error(), "READ_ERROR")
		os.Exit(1)
	}
	if !resp.OK {
		emitError(resp.Error, "READ_ERROR",
			"Run `agios terminal read` to see the current screen",
		)
		os.Exit(1)
	}

	result := map[string]any{
		"output":     formatScreen(resp.Output, resp.CursorRow, resp.CursorCol),
		"cursor":     fmt.Sprintf("%d:%d", resp.CursorRow, resp.CursorCol),
		"session_id": resp.SessionID,
	}
	if resp.Exited {
		result["exited"] = true
	}
	result["help"] = []string{
		"Run `agios terminal send <input>` to send a command",
		"Run `agios terminal send --raw \"\\x03\"` to send Ctrl+C",
		"Run `agios terminal kill` to kill the session",
	}

	emitResult(result)
}

// doSwitch handles the "terminal switch" command.
func doSwitch(sessionID int) {
	if err := requireServer(); err != nil {
		emitError("No terminal server running", "SERVER_ERROR",
			"Run `agios terminal start` to create a session",
		)
		os.Exit(1)
	}

	resp, err := sendCommand(Request{
		Command:   "switch",
		SessionID: sessionID,
	})
	if err != nil {
		emitError("Failed to switch session: "+err.Error(), "SWITCH_ERROR")
		os.Exit(1)
	}
	if !resp.OK {
		emitError(resp.Error, "SWITCH_ERROR",
			"Run `agios terminal` to see active sessions",
		)
		os.Exit(1)
	}

	emitResult(map[string]any{
		"message":    fmt.Sprintf("Switched to session %d", sessionID),
		"session_id": resp.SessionID,
		"help": []string{
			"Run `agios terminal read` to see the current screen",
		},
	})
}

// doKill handles the "terminal kill" command.
func doKill(sessionID int) {
	if err := requireServer(); err != nil {
		emitError("No terminal server running", "SERVER_ERROR")
		os.Exit(1)
	}

	resp, err := sendCommand(Request{
		Command:   "kill",
		SessionID: sessionID,
	})
	if err != nil {
		emitError("Failed to kill session: "+err.Error(), "KILL_ERROR")
		os.Exit(1)
	}
	if !resp.OK {
		emitError(resp.Error, "KILL_ERROR",
			"Run `agios terminal` to see active sessions",
		)
		os.Exit(1)
	}

	emitResult(map[string]any{
		"message": "Session killed",
		"help": []string{
			"Run `agios terminal start` to create a new session",
			"Run `agios terminal` to see remaining sessions",
		},
	})
}

// doResize handles the "terminal resize" command.
func doResize(sessionID, rows, cols int) {
	if err := requireServer(); err != nil {
		emitError("No terminal server running", "SERVER_ERROR")
		os.Exit(1)
	}

	resp, err := sendCommand(Request{
		Command:   "resize",
		SessionID: sessionID,
		Rows:      rows,
		Cols:      cols,
	})
	if err != nil {
		emitError("Failed to resize: "+err.Error(), "RESIZE_ERROR")
		os.Exit(1)
	}
	if !resp.OK {
		emitError(resp.Error, "RESIZE_ERROR",
			"Run `agios terminal read` to see the current screen",
		)
		os.Exit(1)
	}

	emitResult(map[string]any{
		"message":    fmt.Sprintf("Resized to %dx%d", cols, rows),
		"session_id": resp.SessionID,
		"help": []string{
			"Run `agios terminal send <input>` to send a command",
		},
	})
}

// doQuit handles the "terminal quit" command.
func doQuit() {
	info, err := loadServerInfo()
	if err != nil {
		emitResult(map[string]any{
			"message": "Terminal server is not running",
		})
		return
	}

	if !isServerAlive(info) {
		cleanServer()
		emitResult(map[string]any{
			"message": "Terminal server is not running (cleaned stale files)",
		})
		return
	}

	if err := StopServer(); err != nil {
		emitError("Failed to stop server: "+err.Error(), "SERVER_ERROR")
		os.Exit(1)
	}

	emitResult(map[string]any{
		"message": "Terminal server stopped",
	})
}

// respondOverview shows the current terminal state — dock view.
func respondOverview() {
	var sessions []SessionInfo
	var activeID int

	info, err := loadServerInfo()
	if err == nil && isServerAlive(info) {
		resp, err := sendCommand(Request{Command: "list"})
		if err == nil {
			sessions = resp.Sessions
			activeID = resp.ActiveID
		}
	}

	if sessions == nil {
		sessions = []SessionInfo{}
	}

	help := []string{
		"Run `agios terminal start` to create a new session",
	}
	result := map[string]any{
		"sessions": sessions,
		"help":     help,
	}
	if len(sessions) > 0 {
		result["active_id"] = activeID
		help = []string{
			"Run `agios terminal send <input>` to send a command",
			"Run `agios terminal read` to see the current screen",
		}
		if len(sessions) > 1 {
			help = append(help, "Run `agios terminal switch --session <id>` to switch active session")
		}
		result["help"] = help
	}

	emitResult(result)
}

// formatScreen renders screen output with row numbers and a cursor indicator.
// Each content line is prefixed with a padded row number. A line without a row
// number is inserted after the cursor's row, with a "^" pointing at the column.
func formatScreen(output string, cursorRow, cursorCol int) string {
	lines := strings.Split(output, "\n")

	// Extend if cursor is beyond visible lines
	for cursorRow >= len(lines) {
		lines = append(lines, "")
	}

	width := len(fmt.Sprintf("%d", len(lines)))

	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%*d: %s\n", width, i+1, line)
		if i == cursorRow {
			prefix := strings.Repeat(" ", width) + ": "
			pointer := strings.Repeat(" ", cursorCol) + "^"
			fmt.Fprintf(&b, "%s%s\n", prefix, pointer)
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// processEscapes converts escape sequences like \x03 to their byte values.
func processEscapes(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if i+3 < len(s) && s[i] == '\\' && s[i+1] == 'x' {
			if val, err := strconv.ParseUint(s[i+2:i+4], 16, 8); err == nil {
				result.WriteByte(byte(val))
				i += 4
				continue
			}
		}
		if i+1 < len(s) && s[i] == '\\' {
			switch s[i+1] {
			case 'n':
				result.WriteByte('\n')
				i += 2
				continue
			case 'r':
				result.WriteByte('\r')
				i += 2
				continue
			case 't':
				result.WriteByte('\t')
				i += 2
				continue
			case '\\':
				result.WriteByte('\\')
				i += 2
				continue
			}
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}
