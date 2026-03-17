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

type appStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
	User    string `json:"user,omitempty"`
	Error   string `json:"error,omitempty"`
}

// RunStatus concurrently queries status for all configured apps and built-ins.
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

	if s, ok := obj["status"].(string); ok {
		result.Status = s
	} else {
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

func tasksStatus() appStatus {
	result := appStatus{
		Name:    "tasks",
		Version: "1.0.0",
	}

	status, _ := tasks.TasksStatus()
	result.Status = status

	return result
}
