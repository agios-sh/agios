package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromCurrentDir(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, FileName)
	os.WriteFile(configPath, []byte("apps: [gh, linear]\n"), 0644)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Path != configPath {
		t.Errorf("Path = %q, want %q", cfg.Path, configPath)
	}
	if len(cfg.Apps) != 2 {
		t.Fatalf("len(Apps) = %d, want 2", len(cfg.Apps))
	}
	if cfg.Apps[0] != "gh" || cfg.Apps[1] != "linear" {
		t.Errorf("Apps = %v, want [gh linear]", cfg.Apps)
	}
}

func TestLoadWalksUpDirectories(t *testing.T) {
	// Create: root/agios.yaml and root/sub/deep/
	root := t.TempDir()
	configPath := filepath.Join(root, FileName)
	os.WriteFile(configPath, []byte("apps: [myapp]\n"), 0644)

	deep := filepath.Join(root, "sub", "deep")
	os.MkdirAll(deep, 0755)

	cfg, err := Load(deep)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Path != configPath {
		t.Errorf("Path = %q, want %q", cfg.Path, configPath)
	}
	if len(cfg.Apps) != 1 || cfg.Apps[0] != "myapp" {
		t.Errorf("Apps = %v, want [myapp]", cfg.Apps)
	}
}

func TestLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load() expected error when no config exists, got nil")
	}
}

func TestLoadEmptyApps(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, FileName)
	os.WriteFile(configPath, []byte("apps: []\n"), 0644)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.Apps) != 0 {
		t.Errorf("len(Apps) = %d, want 0", len(cfg.Apps))
	}
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, FileName)

	cfg := &Config{
		Apps: []string{"gh", "slack"},
		Path: configPath,
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Read it back
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}
	if len(loaded.Apps) != 2 {
		t.Fatalf("len(Apps) = %d, want 2", len(loaded.Apps))
	}
	if loaded.Apps[0] != "gh" || loaded.Apps[1] != "slack" {
		t.Errorf("Apps = %v, want [gh slack]", loaded.Apps)
	}
}

func TestSaveNoPath(t *testing.T) {
	cfg := &Config{Apps: []string{}}
	if err := cfg.Save(); err == nil {
		t.Fatal("Save() with no Path expected error, got nil")
	}
}

func TestHasApp(t *testing.T) {
	cfg := &Config{Apps: []string{"gh", "linear"}}

	if !cfg.HasApp("gh") {
		t.Error("HasApp(gh) = false, want true")
	}
	if !cfg.HasApp("linear") {
		t.Error("HasApp(linear) = false, want true")
	}
	if cfg.HasApp("slack") {
		t.Error("HasApp(slack) = true, want false")
	}
}

func TestFindFromSubdirectory(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, FileName)
	os.WriteFile(configPath, []byte("apps: []\n"), 0644)

	sub := filepath.Join(root, "a", "b", "c")
	os.MkdirAll(sub, 0755)

	found, err := Find(sub)
	if err != nil {
		t.Fatalf("Find() error: %v", err)
	}
	if found != configPath {
		t.Errorf("Find() = %q, want %q", found, configPath)
	}
}

func TestFindNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Find(dir)
	if err == nil {
		t.Fatal("Find() expected error when no config exists, got nil")
	}
}
