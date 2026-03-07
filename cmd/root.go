package cmd

import (
	"github.com/agios-sh/agios/browser"
	"github.com/agios-sh/agios/peek"
	"github.com/agios-sh/agios/tasks"
	"github.com/agios-sh/agios/terminal"
)

// RunHome implements the home command: `agios` with no arguments.
// It fetches peek data from all apps and presents a unified dock view.
func RunHome() {
	cfg := loadConfig()

	builtins := []peek.AppEntry{
		{Name: "browser", Summary: "Built-in browser for web automation", Peek: browser.PeekData()},
		{Name: "terminal", Summary: "Built-in terminal for interactive shell sessions", Peek: terminal.PeekData()},
		{Name: "tasks", Summary: "Built-in task tracking across local files and GitHub Issues", Peek: tasks.PeekData()},
	}

	help := []string{
		"Run `agios <app>` to see an app's current state",
		"Run `agios <app> help` to see all commands for an app",
	}

	if len(cfg.Apps) == 0 {
		writePipelinedJSON(map[string]any{
			"apps": builtins,
			"help": help,
		})
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

	writePipelinedJSON(map[string]any{
		"apps": apps,
		"help": help,
	})
}
