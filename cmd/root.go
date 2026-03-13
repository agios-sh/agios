package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/agios-sh/agios/browser"
	"github.com/agios-sh/agios/peek"
	"github.com/agios-sh/agios/tasks"
	"github.com/agios-sh/agios/terminal"
	"github.com/agios-sh/agios/updater"
)

// RunHome implements the home command: `agios` with no arguments.
// It fetches peek data from all apps and presents a unified dock view.
func RunHome(version string) {
	cfg := loadConfig()

	builtins := []peek.AppEntry{
		{Name: "browser", Summary: "Built-in browser for web automation", Peek: browser.PeekData()},
		{Name: "terminal", Summary: "Built-in terminal for interactive shell sessions", Peek: terminal.PeekData()},
		{Name: "tasks", Summary: "Built-in task tracking using local files", Peek: tasks.PeekData()},
	}

	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = os.Args[0]
	}
	if resolved, err := filepath.EvalSymlinks(binaryPath); err == nil {
		binaryPath = resolved
	}

	inPath := false
	if pathBin, err := exec.LookPath("agios"); err == nil {
		if resolved, err := filepath.EvalSymlinks(pathBin); err == nil && resolved == binaryPath {
			inPath = true
		}
	}

	if !inPath {
		if cwd, err := os.Getwd(); err == nil && filepath.Dir(binaryPath) == cwd {
			binaryPath = "./" + filepath.Base(binaryPath)
		}
	}

	binLabel := binaryPath
	if inPath {
		binLabel = "agios"
	}

	help := []string{
		fmt.Sprintf("Run `%s <name>` to see an app's current state", binLabel),
		fmt.Sprintf("Run `%s <name> help` to see all commands for an app", binLabel),
	}

	// Check for updates (non-blocking)
	if version != "dev" && os.Getenv("AGIOS_NO_UPDATE_CHECK") == "" {
		cached := updater.ReadCache(version)
		if updater.IsCacheStale() {
			_ = updater.SpawnBackgroundCheck(version)
		}
		if cached != nil && cached.UpdateAvailable {
			help = append([]string{
				fmt.Sprintf("Inform your user that agios %s is available (current: %s). They need to run `%s update` to update.", cached.LatestVersion, cached.CurrentVersion, binLabel),
			}, help...)
		}
	}

	if len(cfg.Apps) == 0 {
		out := map[string]any{
			"apps": builtins,
			"help": help,
		}
		if !inPath {
			out["agios_bin"] = binaryPath
		}
		writePipelinedJSON(out)
		return
	}

	// Fetch peek data from all apps concurrently
	results := peek.FetchAll(cfg.Apps)

	// Build app entries with inline peek data
	apps := make([]peek.AppEntry, len(results))
	for i, r := range results {
		apps[i] = peek.AppEntry{
			Name:    r.AppName,
			Summary: r.Description,
			Peek:    r.Peek,
			Error:   r.Error,
		}
	}

	// Append built-in apps
	apps = append(apps, builtins...)

	out := map[string]any{
		"apps": apps,
		"help": help,
	}
	if !inPath {
		out["agios_bin"] = binaryPath
	}
	writePipelinedJSON(out)
}
