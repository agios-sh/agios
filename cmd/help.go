package cmd

import (
	"os"

	"github.com/agios-sh/agios/config"
)

type commandInfo struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Usage   string `json:"usage"`
}

func builtinCommands() []commandInfo {
	return []commandInfo{
		{Name: "init", Summary: "Initialize agios.yaml in the current directory", Usage: "agios init"},
		{Name: "add", Summary: "Add an app to agios.yaml", Usage: "agios add <name>"},
		{Name: "remove", Summary: "Remove an app from agios.yaml", Usage: "agios remove <name>"},
		{Name: "status", Summary: "Check health of all configured apps", Usage: "agios status"},
		{Name: "help", Summary: "Show this help message", Usage: "agios help"},
		{Name: "jobs", Summary: "List or check background jobs", Usage: "agios jobs [<id>]"},
		{Name: "browser", Summary: "Built-in browser for web automation", Usage: "agios browser <command>"},
		{Name: "terminal", Summary: "Built-in terminal for interactive shell sessions", Usage: "agios terminal <command>"},
		{Name: "tasks", Summary: "Built-in task tracking using local files", Usage: "agios tasks <command>"},
		{Name: "update", Summary: "Check for and install agios updates", Usage: "agios update [check]"},
	}
}

func RunHelp() {
	result := map[string]any{
		"usage":    "agios [command|app] [args]",
		"commands": builtinCommands(),
		"help": []string{
			"Run `agios init` to create a new agios.yaml config",
			"Run `agios add <name>` to register an app",
			"Run `agios <name> <command>` to interact with a registered app",
		},
	}

	cwd, err := os.Getwd()
	if err == nil {
		cfg, err := config.Load(cwd)
		if err == nil && len(cfg.Apps) > 0 {
			result["apps"] = cfg.Apps
		}
	}

	writePipelinedJSON(result)
}

func RunVersion(version string) {
	writePipelinedJSON(map[string]any{
		"version": version,
	})
}
