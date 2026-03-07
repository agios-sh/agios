package cmd

import (
	"os"

	"github.com/agios-sh/agios/config"
)

// commandInfo describes a built-in agios command.
type commandInfo struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Usage   string `json:"usage"`
}

// builtinCommands returns the list of built-in agios commands.
func builtinCommands() []commandInfo {
	return []commandInfo{
		{Name: "init", Summary: "Initialize agios.yaml in the current directory", Usage: "agios init"},
		{Name: "add", Summary: "Add an app to agios.yaml", Usage: "agios add <app>"},
		{Name: "remove", Summary: "Remove an app from agios.yaml", Usage: "agios remove <app>"},
		{Name: "status", Summary: "Check health of all configured apps", Usage: "agios status"},
		{Name: "help", Summary: "Show this help message", Usage: "agios help"},
		{Name: "jobs", Summary: "List or check background jobs", Usage: "agios jobs [id]"},
		{Name: "browser", Summary: "Built-in browser for web automation", Usage: "agios browser <command>"},
		{Name: "terminal", Summary: "Built-in terminal for interactive shell sessions", Usage: "agios terminal <command>"},
		{Name: "tasks", Summary: "Built-in task tracking across local files and GitHub Issues", Usage: "agios tasks <command>"},
	}
}

// RunHelp implements the "agios help" command.
// It returns output describing all available commands and active apps from config.
func RunHelp() {
	result := map[string]any{
		"usage":    "agios [command|app] [args]",
		"commands": builtinCommands(),
		"help": []string{
			"Run `agios init` to create a new agios.yaml config",
			"Run `agios add <app>` to register an app",
			"Run `agios <app> <command>` to interact with a registered app",
		},
	}

	// Try to load config and list active apps
	cwd, err := os.Getwd()
	if err == nil {
		cfg, err := config.Load(cwd)
		if err == nil && len(cfg.Apps) > 0 {
			result["apps"] = cfg.Apps
		}
	}

	writePipelinedJSON(result)
}

// RunVersion implements the "agios --version" command.
func RunVersion(version string) {
	writePipelinedJSON(map[string]any{
		"version": version,
	})
}
