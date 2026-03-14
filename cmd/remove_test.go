package cmd

import (
	"path/filepath"
	"testing"

	"github.com/agios-sh/agios/config"
)

func TestRemoveAppSuccessfully(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Apps: []string{"gh", "slack"},
		Path: filepath.Join(dir, config.FileName),
	}
	cfg.Save()

	err := removeApp(cfg, "gh")
	if err != nil {
		t.Fatalf("removeApp() error: %v", err)
	}

	if cfg.HasApp("gh") {
		t.Error("app was not removed from config")
	}
	if !cfg.HasApp("slack") {
		t.Error("other app was incorrectly removed")
	}

	// Verify persistence
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.HasApp("gh") {
		t.Error("removed app still persisted in config file")
	}
	if !loaded.HasApp("slack") {
		t.Error("other app was not persisted in config file")
	}
}

func TestRemoveAppNotConfigured(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Apps: []string{"gh"},
		Path: filepath.Join(dir, config.FileName),
	}
	cfg.Save()

	err := removeApp(cfg, "nonexistent")
	if err == nil {
		t.Fatal("expected error for app not in config")
	}
	ce, ok := err.(*cmdError)
	if !ok {
		t.Fatalf("expected *cmdError, got %T", err)
	}
	if ce.code != "NOT_CONFIGURED" {
		t.Errorf("code = %q, want %q", ce.code, "NOT_CONFIGURED")
	}
}

func TestRemoveAppEmptyConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Apps: []string{},
		Path: filepath.Join(dir, config.FileName),
	}
	cfg.Save()

	err := removeApp(cfg, "gh")
	if err == nil {
		t.Fatal("expected error for empty config")
	}
	ce, ok := err.(*cmdError)
	if !ok {
		t.Fatalf("expected *cmdError, got %T", err)
	}
	if ce.code != "NOT_CONFIGURED" {
		t.Errorf("code = %q, want %q", ce.code, "NOT_CONFIGURED")
	}
}

func TestRemoveLastApp(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Apps: []string{"gh"},
		Path: filepath.Join(dir, config.FileName),
	}
	cfg.Save()

	err := removeApp(cfg, "gh")
	if err != nil {
		t.Fatalf("removeApp() error: %v", err)
	}

	if len(cfg.Apps) != 0 {
		t.Errorf("len(Apps) = %d, want 0", len(cfg.Apps))
	}

	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(loaded.Apps) != 0 {
		t.Errorf("persisted len(Apps) = %d, want 0", len(loaded.Apps))
	}
}

func TestRemoveAppPreservesOrder(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Apps: []string{"alpha", "beta", "gamma"},
		Path: filepath.Join(dir, config.FileName),
	}
	cfg.Save()

	err := removeApp(cfg, "beta")
	if err != nil {
		t.Fatalf("removeApp() error: %v", err)
	}

	expected := []string{"alpha", "gamma"}
	if len(cfg.Apps) != len(expected) {
		t.Fatalf("len(Apps) = %d, want %d", len(cfg.Apps), len(expected))
	}
	for i, name := range expected {
		if cfg.Apps[i] != name {
			t.Errorf("Apps[%d] = %q, want %q", i, cfg.Apps[i], name)
		}
	}
}
