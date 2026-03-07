package terminal

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// serverInfo stores terminal server connection details persisted to disk.
type serverInfo struct {
	PID       int       `json:"pid"`
	Socket    string    `json:"socket"`
	StartedAt time.Time `json:"started_at"`
}

// Request is a JSON command sent from client to server.
type Request struct {
	Command   string `json:"command"`
	Name      string `json:"name,omitempty"`
	Shell     string `json:"shell,omitempty"`
	Dir       string `json:"dir,omitempty"`
	SessionID int    `json:"session_id,omitempty"`
	Input     string `json:"input,omitempty"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
	Rows      int    `json:"rows,omitempty"`
	Cols      int    `json:"cols,omitempty"`
}

// Response is a JSON response sent from server to client.
type Response struct {
	OK        bool          `json:"ok"`
	SessionID int           `json:"session_id,omitempty"`
	Output    string        `json:"output,omitempty"`
	CursorRow int           `json:"cursor_row,omitempty"`
	CursorCol int           `json:"cursor_col,omitempty"`
	Exited    bool          `json:"exited,omitempty"`
	Sessions  []SessionInfo `json:"sessions,omitempty"`
	ActiveID  int           `json:"active_id,omitempty"`
	Error     string        `json:"error,omitempty"`
}

// terminalDir returns the path to ~/.agios/terminal/, creating it if needed.
func terminalDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	dir := filepath.Join(home, ".agios", "terminal")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating terminal directory: %w", err)
	}
	return dir, nil
}

// serverInfoPath returns the path to server.json.
func serverInfoPath() (string, error) {
	dir, err := terminalDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "server.json"), nil
}

// socketPath returns the path to the Unix domain socket.
func socketPath() (string, error) {
	dir, err := terminalDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "server.sock"), nil
}

// runServer is the server daemon main loop. Called by RunServer() from terminal.go.
func runServer() {
	sockPath, err := socketPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "terminal server: %v\n", err)
		os.Exit(1)
	}

	// Remove stale socket
	os.Remove(sockPath)

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "terminal server: listen: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()
	defer os.Remove(sockPath)

	// Save server info
	info := serverInfo{
		PID:       os.Getpid(),
		Socket:    sockPath,
		StartedAt: time.Now(),
	}
	infoPath, err := serverInfoPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "terminal server: %v\n", err)
		os.Exit(1)
	}
	jsonData, _ := json.MarshalIndent(info, "", "  ")
	if err := os.WriteFile(infoPath, jsonData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "terminal server: save info: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(infoPath)

	mgr := NewSessionManager()

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		mgr.KillAll()
		listener.Close()
		os.Remove(sockPath)
		os.Remove(infoPath)
		os.Exit(0)
	}()

	// Accept loop
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Listener closed (shutdown)
			return
		}
		go handleConnection(conn, mgr)
	}
}

// handleConnection processes a single client connection.
func handleConnection(conn net.Conn, mgr *SessionManager) {
	defer conn.Close()

	var req Request
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&req); err != nil {
		writeResponse(conn, Response{OK: false, Error: "invalid request"})
		return
	}

	var resp Response

	switch req.Command {
	case "ping":
		resp = Response{OK: true}

	case "start":
		sess, err := mgr.Start(req.Name, req.Shell, req.Dir)
		if err != nil {
			resp = Response{OK: false, Error: err.Error()}
		} else {
			resp = Response{OK: true, SessionID: sess.ID}
		}

	case "send":
		sess, err := mgr.Get(req.SessionID)
		if err != nil {
			resp = Response{OK: false, Error: err.Error()}
		} else {
			timeoutMs := req.TimeoutMs
			if timeoutMs <= 0 {
				timeoutMs = 5000
			}
			state, err := sess.SendAndWait([]byte(req.Input), timeoutMs)
			if err != nil {
				resp = Response{OK: false, Error: err.Error()}
			} else {
				sess.mu.Lock()
				exited := sess.Exited
				sess.mu.Unlock()
				resp = Response{OK: true, SessionID: sess.ID, Output: state.Text, CursorRow: state.CursorRow, CursorCol: state.CursorCol, Exited: exited}
			}
		}

	case "read":
		sess, err := mgr.Get(req.SessionID)
		if err != nil {
			resp = Response{OK: false, Error: err.Error()}
		} else {
			state := sess.ReadScreen()
			sess.mu.Lock()
			exited := sess.Exited
			sess.mu.Unlock()
			resp = Response{OK: true, SessionID: sess.ID, Output: state.Text, CursorRow: state.CursorRow, CursorCol: state.CursorCol, Exited: exited}
		}

	case "list":
		sessions, activeID := mgr.List()
		if sessions == nil {
			sessions = []SessionInfo{}
		}
		resp = Response{OK: true, Sessions: sessions, ActiveID: activeID}

	case "switch":
		err := mgr.Switch(req.SessionID)
		if err != nil {
			resp = Response{OK: false, Error: err.Error()}
		} else {
			resp = Response{OK: true, SessionID: req.SessionID}
		}

	case "kill":
		err := mgr.Kill(req.SessionID)
		if err != nil {
			resp = Response{OK: false, Error: err.Error()}
		} else {
			resp = Response{OK: true}
		}

	case "resize":
		sess, err := mgr.Get(req.SessionID)
		if err != nil {
			resp = Response{OK: false, Error: err.Error()}
		} else {
			rows := req.Rows
			cols := req.Cols
			if rows <= 0 {
				rows = 80
			}
			if cols <= 0 {
				cols = 120
			}
			if err := sess.pty.Resize(rows, cols); err != nil {
				resp = Response{OK: false, Error: err.Error()}
			} else {
				sess.screen.Resize(rows, cols)
				resp = Response{OK: true, SessionID: sess.ID}
			}
		}

	case "quit":
		mgr.KillAll()
		resp = Response{OK: true}
		writeResponse(conn, resp)
		// Shut down after responding
		os.Exit(0)

	default:
		resp = Response{OK: false, Error: "unknown command: " + req.Command}
	}

	writeResponse(conn, resp)
}

// writeResponse encodes a response to a connection.
func writeResponse(conn net.Conn, resp Response) {
	enc := json.NewEncoder(conn)
	enc.Encode(resp)
}

// loadServerInfo reads server.json.
func loadServerInfo() (serverInfo, error) {
	var info serverInfo
	infoPath, err := serverInfoPath()
	if err != nil {
		return info, err
	}
	data, err := os.ReadFile(infoPath)
	if err != nil {
		return info, err
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return info, err
	}
	return info, nil
}

// isServerAlive checks if the terminal server is running.
func isServerAlive(info serverInfo) bool {
	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	// Try connecting to the socket
	conn, err := net.DialTimeout("unix", info.Socket, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// StartServer ensures the terminal server daemon is running.
// Returns true if it was already running.
func StartServer() (bool, error) {
	info, err := loadServerInfo()
	if err == nil && isServerAlive(info) {
		return true, nil
	}

	// Clean stale files
	cleanServer()

	// Find our own binary
	self, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("finding executable: %w", err)
	}

	cmd := exec.Command(self, "--terminal-server")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("starting terminal server: %w", err)
	}

	// Detach
	go cmd.Wait()

	// Wait for server to be ready
	sockPath, err := socketPath()
	if err != nil {
		return false, err
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			return false, fmt.Errorf("timed out waiting for terminal server to start")
		default:
		}

		conn, err := net.DialTimeout("unix", sockPath, 500*time.Millisecond)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		conn.Close()
		return false, nil
	}
}

// StopServer stops the terminal server daemon.
func StopServer() error {
	info, err := loadServerInfo()
	if err != nil {
		return fmt.Errorf("no active terminal server")
	}

	// Try sending quit command
	conn, err := net.DialTimeout("unix", info.Socket, 2*time.Second)
	if err == nil {
		enc := json.NewEncoder(conn)
		enc.Encode(Request{Command: "quit"})
		// Read response (best effort)
		dec := json.NewDecoder(conn)
		var resp Response
		dec.Decode(&resp)
		conn.Close()
		// Give it a moment to exit
		time.Sleep(200 * time.Millisecond)
		cleanServer()
		return nil
	}

	// Fallback: kill the process
	proc, err := os.FindProcess(info.PID)
	if err == nil {
		proc.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() {
			proc.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			proc.Kill()
		}
	}

	cleanServer()
	return nil
}

// cleanServer removes server.json and the socket file.
func cleanServer() {
	if p, err := serverInfoPath(); err == nil {
		os.Remove(p)
	}
	if p, err := socketPath(); err == nil {
		os.Remove(p)
	}
}

// TerminalStatus returns the status of the terminal server for cmd/status.go.
func TerminalStatus() (running bool, pid int) {
	info, err := loadServerInfo()
	if err != nil {
		return false, 0
	}
	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return false, 0
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, 0
	}
	return true, info.PID
}

// ActiveSessionCount returns the number of active sessions by querying the server.
func ActiveSessionCount() int {
	info, err := loadServerInfo()
	if err != nil {
		return 0
	}

	conn, err := net.DialTimeout("unix", info.Socket, 2*time.Second)
	if err != nil {
		return 0
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	enc.Encode(Request{Command: "list"})

	var resp Response
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&resp); err != nil {
		return 0
	}

	return len(resp.Sessions)
}

// FormatPID formats a PID as string for use in status messages.
func FormatPID(pid int) string {
	return strconv.Itoa(pid)
}
