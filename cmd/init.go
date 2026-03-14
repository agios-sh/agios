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

// RunInit implements the "agios init" command.
func RunInit() {
	cwd, err := os.Getwd()
	if err != nil {
		writeError("Failed to get working directory", "INIT_ERROR",
			"Run `agios help` for usage information",
		)
		os.Exit(1)
	}

	configPath := filepath.Join(cwd, config.FileName)

	// Error if agios.yaml already exists in the current directory
	if _, err := os.Stat(configPath); err == nil {
		writeError(
			fmt.Sprintf("%s already exists in this directory", config.FileName),
			"ALREADY_INITIALIZED",
			"Run `agios add <app>` to register an app",
		)
		os.Exit(1)
	}

	// Create agios.yaml with empty apps list
	cfg := &config.Config{
		Apps: []string{},
		Path: configPath,
	}
	if err := cfg.Save(); err != nil {
		writeError("Failed to create config file", "INIT_ERROR",
			"Check file permissions in the current directory",
		)
		os.Exit(1)
	}

	// Handle agent memory file
	if err := setupAgentMemoryFile(cwd); err != nil {
		writeError("Failed to set up agent memory file", "INIT_ERROR",
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

// setupAgentMemoryFile detects CLAUDE.md / AGENTS.md and appends AGI OS instructions.
// If neither exists, creates AGENTS.md and symlinks CLAUDE.md to it.
func setupAgentMemoryFile(dir string) error {
	claudePath := filepath.Join(dir, "CLAUDE.md")
	agentsPath := filepath.Join(dir, "AGENTS.md")

	claudeExists := fileExists(claudePath)
	agentsExists := fileExists(agentsPath)

	switch {
	case claudeExists:
		// Append to existing CLAUDE.md
		return appendToFile(claudePath, agentMemoryContent)
	case agentsExists:
		// Append to existing AGENTS.md
		if err := appendToFile(agentsPath, agentMemoryContent); err != nil {
			return err
		}
		// Create CLAUDE.md symlink so agents discover agios
		return os.Symlink("AGENTS.md", claudePath)
	default:
		// Create AGENTS.md and symlink CLAUDE.md to it
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
	// Check if content already exists to avoid duplicate appends.
	existing, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if strings.Contains(string(existing), strings.TrimSpace(content)) {
		return nil // already present
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

// Shared helpers (loadConfig, writeJSON, writePipelinedJSON, writeError)
// live in cmd/helpers.go.
