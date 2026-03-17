package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agios-sh/agios/config"
)

const agentMemoryContent = `
# AGI OS

This project uses AGI OS (agios) for agent-friendly access to external tools.

- Run ` + "`agios`" + ` to see all active apps and pending notifications
- Run ` + "`agios <app> <command>`" + ` to interact with a specific app
- Run ` + "`agios status`" + ` to check the health of all connected apps
- Always prefer ` + "`agios`" + ` over direct tool CLIs when available
`

func RunInit() {
	cwd, err := os.Getwd()
	if err != nil {
		emitError("Failed to get working directory", "INIT_ERROR",
			"Run `agios help` for usage information",
		)
		os.Exit(1)
	}

	configPath := filepath.Join(cwd, config.FileName)

	if _, err := os.Stat(configPath); err == nil {
		emitError(
			fmt.Sprintf("%s already exists in this directory", config.FileName),
			"ALREADY_INITIALIZED",
			"Run `agios add <app>` to register an app",
		)
		os.Exit(1)
	}

	cfg := &config.Config{
		Apps: []string{},
		Path: configPath,
	}
	if err := cfg.Save(); err != nil {
		emitError("Failed to create config file", "INIT_ERROR",
			"Check file permissions in the current directory",
		)
		os.Exit(1)
	}

	if err := setupAgentMemoryFile(cwd); err != nil {
		emitError("Failed to set up agent memory file", "INIT_ERROR",
			"Check file permissions in the current directory",
		)
		os.Exit(1)
	}

	writePipelinedJSON(map[string]any{
		"message": "Initialized agios.yaml",
		"help": []string{
			"Run `agios add <app>` to register an app",
			"Run `agios status` to check the health of all configured apps",
		},
	})
}

// setupAgentMemoryFile appends AGI OS instructions to CLAUDE.md or AGENTS.md,
// creating AGENTS.md + a CLAUDE.md symlink if neither exists.
func setupAgentMemoryFile(dir string) error {
	claudePath := filepath.Join(dir, "CLAUDE.md")
	agentsPath := filepath.Join(dir, "AGENTS.md")

	claudeExists := fileExists(claudePath)
	agentsExists := fileExists(agentsPath)

	switch {
	case claudeExists:
		return appendToFile(claudePath, agentMemoryContent)
	case agentsExists:
		if err := appendToFile(agentsPath, agentMemoryContent); err != nil {
			return err
		}
		return os.Symlink("AGENTS.md", claudePath)
	default:
		if err := os.WriteFile(agentsPath, []byte(agentMemoryContent), 0644); err != nil {
			return fmt.Errorf("creating AGENTS.md: %w", err)
		}
		if err := os.Symlink("AGENTS.md", claudePath); err != nil {
			return fmt.Errorf("creating CLAUDE.md symlink: %w", err)
		}
		return nil
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func appendToFile(path, content string) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if strings.Contains(string(existing), strings.TrimSpace(content)) {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("appending to %s: %w", path, err)
	}
	return nil
}
