package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/agios-sh/agios/runner"
	"github.com/agios-sh/agios/tasks"
	"github.com/agios-sh/agios/terminal"
	"golang.org/x/sync/errgroup"
)

// appStatus holds the status result for a single app.
type appStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
	User    string `json:"user,omitempty"`
	Error   string `json:"error,omitempty"`
}

// RunStatus implements the "agios status" command.
// It concurrently runs "<binary> status" for each app in config using errgroup.
func RunStatus() {
	cfg := loadConfig()

	if len(cfg.Apps) == 0 {
		writePipelinedJSON(map[string]any{
			"apps": []appStatus{browserStatus(), terminalStatus(), tasksStatus()},
			"help": []string{
				"No apps configured in the current directory. Run `agios add <name>` to register an app.",
			},
		})
		return
	}

	results := collectAppStatuses(cfg.Apps)

	// Append built-in app statuses
	results = append(results, browserStatus(), terminalStatus(), tasksStatus())

	writePipelinedJSON(map[string]any{
		"apps": results,
		"help": []string{
			"Run `agios <name> <command>` to interact with a specific app",
			"Run `agios add <name>` to register a new app",
		},
	})
}

// collectAppStatuses concurrently runs "<app> status" for each app and returns results.
// Missing or broken apps produce errors rather than blocking other apps.
func collectAppStatuses(apps []string) []appStatus {
	var mu sync.Mutex
	results := make([]appStatus, len(apps))

	g := new(errgroup.Group)

	for i, app := range apps {
		i, app := i, app
		g.Go(func() error {
			status := queryAppStatus(app)
			mu.Lock()
			results[i] = status
			mu.Unlock()
			return nil // never return error — errors are captured in the result
		})
	}

	g.Wait()
	return results
}

// queryAppStatus runs "<app> status" and returns the structured result.
// Failures are returned as error entries in the result.
func queryAppStatus(appName string) appStatus {
	result := appStatus{Name: appName}

	binPath, err := runner.Resolve(appName)
	if err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("Binary %q not found on PATH", appName)
		return result
	}

	execResult, execErr := runner.Exec(binPath, []string{"status"}, runner.DefaultTimeout)

	if execResult == nil || len(execResult.Stdout) == 0 {
		result.Status = "error"
		if execErr != nil {
			result.Error = fmt.Sprintf("Failed to run `%s status`: %s", appName, execErr.Error())
		} else {
			result.Error = fmt.Sprintf("`%s status` produced no output", appName)
		}
		return result
	}

	// Try to parse as single JSON first, then fall back to JSONL
	var obj map[string]any
	if err := json.Unmarshal(execResult.Stdout, &obj); err != nil {
		parsed, parseErr := runner.ParseJSONL(execResult.Stdout)
		if parseErr != nil {
			result.Status = "error"
			result.Error = fmt.Sprintf("`%s status` returned invalid output", appName)
			return result
		}
		obj = parsed.Result
	}

	// Extract known fields
	if s, ok := obj["status"].(string); ok {
		result.Status = s
	} else {
		// If no explicit status field but we got valid JSON, consider it ok
		if execErr != nil {
			result.Status = "error"
		} else {
			result.Status = "ok"
		}
	}

	if v, ok := obj["version"].(string); ok {
		result.Version = v
	}

	if u, ok := obj["user"].(string); ok {
		result.User = u
	}

	// If exec failed, capture warning from the error field or mark as error
	if execErr != nil {
		result.Status = "error"
		if errMsg, hasError := obj["error"].(string); hasError {
			result.Error = errMsg
		} else {
			result.Error = fmt.Sprintf("`%s status` exited with error", appName)
		}
	}

	return result
}

// browserStatus returns the status of the built-in browser app.
func browserStatus() appStatus {
	result := appStatus{
		Name:    "browser",
		Version: "1.0.0",
	}

	home, err := os.UserHomeDir()
	if err != nil {
		result.Status = "info"
		return result
	}

	data, err := os.ReadFile(filepath.Join(home, ".agios", "browser", "session.json"))
	if err != nil {
		result.Status = "info"
		return result
	}

	var info struct {
		PID int `json:"pid"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		result.Status = "info"
		return result
	}

	// Check if process is alive
	proc, err := os.FindProcess(info.PID)
	if err != nil {
		result.Status = "info"
		return result
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		result.Status = "info"
		return result
	}

	result.Status = "ok"
	return result
}

// terminalStatus returns the status of the built-in terminal app.
func terminalStatus() appStatus {
	result := appStatus{
		Name:    "terminal",
		Version: "1.0.0",
	}

	running, _ := terminal.TerminalStatus()
	if running {
		result.Status = "ok"
	} else {
		result.Status = "info"
	}

	return result
}

// tasksStatus returns the status of the built-in tasks app.
func tasksStatus() appStatus {
	result := appStatus{
		Name:    "tasks",
		Version: "1.0.0",
	}

	status, _ := tasks.TasksStatus()
	result.Status = status

	return result
}
