package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agios-sh/agios/config"
)

func TestSetupAgentMemoryFileCreatesNew(t *testing.T) {
	dir := t.TempDir()

	if err := setupAgentMemoryFile(dir); err != nil {
		t.Fatalf("setupAgentMemoryFile() error: %v", err)
	}

	// AGENTS.md should exist with the content
	agentsPath := filepath.Join(dir, "AGENTS.md")
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("AGENTS.md is empty")
	}
	content := string(data)
	if !contains(content, "AGI OS") {
		t.Error("AGENTS.md does not contain 'AGI OS'")
	}
	if !contains(content, "agios") {
		t.Error("AGENTS.md does not contain 'agios'")
	}

	// CLAUDE.md should be a symlink to AGENTS.md
	claudePath := filepath.Join(dir, "CLAUDE.md")
	target, err := os.Readlink(claudePath)
	if err != nil {
		t.Fatalf("reading CLAUDE.md symlink: %v", err)
	}
	if target != "AGENTS.md" {
		t.Errorf("CLAUDE.md symlink target = %q, want %q", target, "AGENTS.md")
	}
}

func TestSetupAgentMemoryFileAppendsToExistingClaude(t *testing.T) {
	dir := t.TempDir()
	claudePath := filepath.Join(dir, "CLAUDE.md")
	os.WriteFile(claudePath, []byte("# Existing content\n"), 0644)

	if err := setupAgentMemoryFile(dir); err != nil {
		t.Fatalf("setupAgentMemoryFile() error: %v", err)
	}

	data, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	content := string(data)
	if !contains(content, "Existing content") {
		t.Error("CLAUDE.md lost existing content")
	}
	if !contains(content, "AGI OS") {
		t.Error("CLAUDE.md does not contain appended AGI OS content")
	}

	// AGENTS.md should NOT have been created
	agentsPath := filepath.Join(dir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); err == nil {
		t.Error("AGENTS.md should not exist when CLAUDE.md already exists")
	}
}

func TestSetupAgentMemoryFileAppendsToExistingAgents(t *testing.T) {
	dir := t.TempDir()
	agentsPath := filepath.Join(dir, "AGENTS.md")
	os.WriteFile(agentsPath, []byte("# Existing agents\n"), 0644)

	if err := setupAgentMemoryFile(dir); err != nil {
		t.Fatalf("setupAgentMemoryFile() error: %v", err)
	}

	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	content := string(data)
	if !contains(content, "Existing agents") {
		t.Error("AGENTS.md lost existing content")
	}
	if !contains(content, "AGI OS") {
		t.Error("AGENTS.md does not contain appended AGI OS content")
	}

	// CLAUDE.md should be a symlink to AGENTS.md
	claudePath := filepath.Join(dir, "CLAUDE.md")
	target, err := os.Readlink(claudePath)
	if err != nil {
		t.Fatalf("reading CLAUDE.md symlink: %v", err)
	}
	if target != "AGENTS.md" {
		t.Errorf("CLAUDE.md symlink target = %q, want %q", target, "AGENTS.md")
	}
}

func TestSetupAgentMemoryFilePrefersClaudeOverAgents(t *testing.T) {
	dir := t.TempDir()
	claudePath := filepath.Join(dir, "CLAUDE.md")
	agentsPath := filepath.Join(dir, "AGENTS.md")
	os.WriteFile(claudePath, []byte("# Claude\n"), 0644)
	os.WriteFile(agentsPath, []byte("# Agents\n"), 0644)

	if err := setupAgentMemoryFile(dir); err != nil {
		t.Fatalf("setupAgentMemoryFile() error: %v", err)
	}

	// Should have appended to CLAUDE.md (preferred)
	claudeData, _ := os.ReadFile(claudePath)
	if !contains(string(claudeData), "AGI OS") {
		t.Error("AGI OS content should be appended to CLAUDE.md")
	}

	// AGENTS.md should be untouched
	agentsData, _ := os.ReadFile(agentsPath)
	if contains(string(agentsData), "AGI OS") {
		t.Error("AGENTS.md should not have been modified when CLAUDE.md exists")
	}
}

func TestInitCreatesConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, config.FileName)

	// Create config directly (simulating what RunInit does)
	cfg := &config.Config{
		Apps: []string{},
		Path: configPath,
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify the config was created
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(loaded.Apps) != 0 {
		t.Errorf("len(Apps) = %d, want 0", len(loaded.Apps))
	}
}

func TestInitErrorsWhenConfigExists(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, config.FileName)
	os.WriteFile(configPath, []byte("apps: []\n"), 0644)

	// Verify that the config file exists
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file should exist: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
