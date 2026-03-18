package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/agios-sh/agios/runner"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// sessionInfo stores Chrome connection details persisted to disk.
type sessionInfo struct {
	PID      int       `json:"pid"`
	WSURL    string    `json:"ws_url"`
	Port     int       `json:"debug_port"`
	Started  time.Time `json:"started_at"`
	Headless bool      `json:"headless"`
	DataDir  string    `json:"data_dir"`
}

// Session holds a live chromedp context connected to a running Chrome.
type Session struct {
	Ctx    context.Context
	Cancel context.CancelFunc
	Info   sessionInfo
}

func browserDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	dir := filepath.Join(home, ".agios", "browser")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating browser directory: %w", err)
	}
	return dir, nil
}

func sessionPath() (string, error) {
	dir, err := browserDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "session.json"), nil
}

// handlesPath returns the path to handles.json.
func handlesPath() (string, error) {
	dir, err := browserDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "handles.json"), nil
}

// findChrome locates the Chrome binary on the system.
func findChrome() (string, error) {
	// Check common binary names
	names := []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"}

	if runtime.GOOS == "darwin" {
		// macOS app bundle paths
		macPaths := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		}
		for _, p := range macPaths {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}

	if runtime.GOOS == "windows" {
		names = append(names, "chrome")
		winPaths := []string{
			filepath.Join(os.Getenv("PROGRAMFILES"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
		}
		for _, p := range winPaths {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}

	for _, name := range names {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("Chrome not found. Install Google Chrome or Chromium")
}

// isAlive checks if a Chrome instance from sessionInfo is still running and responsive.
func isAlive(info sessionInfo) bool {
	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", info.Port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// StartChrome launches a Chrome instance with remote debugging enabled.
func StartChrome(headed bool) (alreadyRunning bool, err error) {
	// Check if already running
	if sp, err := sessionPath(); err == nil {
		if data, err := os.ReadFile(sp); err == nil {
			var info sessionInfo
			if err := json.Unmarshal(data, &info); err == nil && isAlive(info) {
				return true, nil
			}
			// Stale session file — clean up
			cleanSession()
		}
	}

	chromeBin, err := findChrome()
	if err != nil {
		return false, err
	}

	dir, err := browserDir()
	if err != nil {
		return false, err
	}
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return false, fmt.Errorf("creating data directory: %w", err)
	}

	// Remove stale DevToolsActivePort file before starting
	devToolsPortFile := filepath.Join(dataDir, "DevToolsActivePort")
	os.Remove(devToolsPortFile)

	args := []string{
		"--remote-debugging-port=0",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--disable-sync",
		"--user-data-dir=" + dataDir,
	}

	if !headed {
		args = append(args, "--headless=new")
	}

	cmd := exec.Command(chromeBin, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil // /dev/null — avoids SIGPIPE killing Chrome when agios exits

	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("starting Chrome: %w", err)
	}

	// Detach: don't wait for Chrome to exit
	go cmd.Wait()

	// Wait for Chrome to write DevToolsActivePort file with port and WS path
	wsURL := ""
	deadline := time.After(15 * time.Second)
	for wsURL == "" {
		select {
		case <-deadline:
			cmd.Process.Kill()
			return false, fmt.Errorf("timed out waiting for Chrome DevTools URL")
		default:
		}

		data, err := os.ReadFile(devToolsPortFile)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		lines := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
		if len(lines) < 2 {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		port, err := strconv.Atoi(strings.TrimSpace(lines[0]))
		if err != nil || port == 0 {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		wsPath := strings.TrimSpace(lines[1])
		if wsPath == "" {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		wsURL = fmt.Sprintf("ws://127.0.0.1:%d%s", port, wsPath)
	}

	// Parse port from WebSocket URL
	port := 0
	if u, err := url.Parse(wsURL); err == nil {
		port, _ = strconv.Atoi(u.Port())
	}

	info := sessionInfo{
		PID:      cmd.Process.Pid,
		WSURL:    wsURL,
		Port:     port,
		Started:  time.Now(),
		Headless: !headed,
		DataDir:  dataDir,
	}

	sp, err := sessionPath()
	if err != nil {
		return false, err
	}
	jsonData, _ := json.MarshalIndent(info, "", "  ")
	if err := os.WriteFile(sp, jsonData, 0o644); err != nil {
		return false, fmt.Errorf("saving session: %w", err)
	}

	return false, nil
}

// findPageTarget queries Chrome's /json endpoint to find the first page target ID.
func findPageTarget(port int) (string, error) {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json", port))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var targets []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal(body, &targets); err != nil {
		return "", err
	}

	// Prefer a non-blank page target
	for _, t := range targets {
		if t.Type == "page" && t.URL != "about:blank" {
			return t.ID, nil
		}
	}
	for _, t := range targets {
		if t.Type == "page" {
			return t.ID, nil
		}
	}

	return "", fmt.Errorf("no page targets found")
}

// Dial connects to a running Chrome instance using session.json.
func Dial() (*Session, error) {
	sp, err := sessionPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(sp)
	if err != nil {
		return nil, fmt.Errorf("no active session: %w", err)
	}

	var info sessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("invalid session file: %w", err)
	}

	// Check if the process is still alive
	proc, err := os.FindProcess(info.PID)
	if err != nil {
		cleanSession()
		return nil, fmt.Errorf("Chrome process not found")
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		cleanSession()
		return nil, fmt.Errorf("Chrome process (PID %d) is not running", info.PID)
	}

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), info.WSURL)

	// Attach to an existing page target if one exists, otherwise create a new tab.
	var ctx context.Context
	var cancel context.CancelFunc
	if tid, err := findPageTarget(info.Port); err == nil {
		ctx, cancel = chromedp.NewContext(allocCtx, chromedp.WithTargetID(target.ID(tid)))
	} else {
		ctx, cancel = chromedp.NewContext(allocCtx)
	}

	return &Session{
		Ctx: ctx,
		Cancel: func() {
			cancel()
			allocCancel()
		},
		Info: info,
	}, nil
}

// RequireSession dials Chrome or returns a helpful error.
//
// Callers should NOT call sess.Cancel() — process exit closes the WebSocket
// connection without sending Target.closeTarget, keeping the Chrome tab alive
// for subsequent agios invocations. Use `_ = sess.Cancel` to silence the
// unused-variable linter.
func RequireSession() (*Session, error) {
	sess, err := Dial()
	if err != nil {
		return nil, fmt.Errorf("no active browser session. Run `agios browser open` first")
	}
	return sess, nil
}

// StopChrome terminates the running Chrome instance and cleans up state files.
func StopChrome() error {
	sp, err := sessionPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(sp)
	if err != nil {
		return fmt.Errorf("no active browser session")
	}

	var info sessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		cleanSession()
		return fmt.Errorf("invalid session file (cleaned up)")
	}

	proc, err := os.FindProcess(info.PID)
	if err == nil {
		runner.GracefulKill(proc, 5*time.Second)
	}

	cleanSession()
	return nil
}

func cleanSession() {
	if sp, err := sessionPath(); err == nil {
		os.Remove(sp)
	}
	if hp, err := handlesPath(); err == nil {
		os.Remove(hp)
	}
}
