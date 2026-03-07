package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/agios-sh/agios/config"
)

func TestAddAppAlreadyConfigured(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Apps: []string{"gh"},
		Path: filepath.Join(dir, config.FileName),
	}
	cfg.Save()

	err := addApp(cfg, "gh")
	if err == nil {
		t.Fatal("expected error for already-configured app")
	}
	ae, ok := err.(*addError)
	if !ok {
		t.Fatalf("expected *addError, got %T", err)
	}
	if ae.code != "ALREADY_ADDED" {
		t.Errorf("code = %q, want %q", ae.code, "ALREADY_ADDED")
	}
}

func TestAddAppBinaryNotFound(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Apps: []string{},
		Path: filepath.Join(dir, config.FileName),
	}
	cfg.Save()

	err := addApp(cfg, "nonexistent-binary-xyz-12345")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	ae, ok := err.(*addError)
	if !ok {
		t.Fatalf("expected *addError, got %T", err)
	}
	if ae.code != "BINARY_NOT_FOUND" {
		t.Errorf("code = %q, want %q", ae.code, "BINARY_NOT_FOUND")
	}
}

func TestAddAppWithMockBinary(t *testing.T) {
	// Build a mock binary that outputs valid JSON for "status"
	dir := t.TempDir()
	mockSrc := filepath.Join(dir, "mock-app.go")
	os.WriteFile(mockSrc, []byte(`package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "status" {
		fmt.Println("{\"status\": \"ok\", \"version\": \"1.0.0\"}")
		return
	}
	fmt.Println("{\"error\": \"unknown command\"}")
	os.Exit(1)
}
`), 0644)

	mockBin := filepath.Join(dir, "mock-app")
	if runtime.GOOS == "windows" {
		mockBin += ".exe"
	}

	// Compile the mock binary
	build := exec.Command("go", "build", "-o", mockBin, mockSrc)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building mock binary: %v\n%s", err, out)
	}

	// Add mock-app dir to PATH
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
	defer os.Setenv("PATH", origPath)

	cfgDir := t.TempDir()
	cfg := &config.Config{
		Apps: []string{},
		Path: filepath.Join(cfgDir, config.FileName),
	}
	cfg.Save()

	err := addApp(cfg, "mock-app")
	if err != nil {
		t.Fatalf("addApp() error: %v", err)
	}

	// Verify app was added
	if !cfg.HasApp("mock-app") {
		t.Error("app was not added to config")
	}

	// Verify config was persisted
	loaded, err := config.Load(cfgDir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !loaded.HasApp("mock-app") {
		t.Error("app was not persisted in config file")
	}
}

func TestAddAppAIPValidationFails(t *testing.T) {
	// Build a mock binary that outputs invalid (non-JSON) for "status"
	dir := t.TempDir()
	mockSrc := filepath.Join(dir, "bad-app.go")
	os.WriteFile(mockSrc, []byte(`package main

import "fmt"

func main() {
	fmt.Println("this is not json")
}
`), 0644)

	mockBin := filepath.Join(dir, "bad-app")
	if runtime.GOOS == "windows" {
		mockBin += ".exe"
	}

	build := exec.Command("go", "build", "-o", mockBin, mockSrc)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building mock binary: %v\n%s", err, out)
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
	defer os.Setenv("PATH", origPath)

	cfgDir := t.TempDir()
	cfg := &config.Config{
		Apps: []string{},
		Path: filepath.Join(cfgDir, config.FileName),
	}
	cfg.Save()

	err := addApp(cfg, "bad-app")
	if err == nil {
		t.Fatal("expected AIP validation error")
	}
	ae, ok := err.(*addError)
	if !ok {
		t.Fatalf("expected *addError, got %T", err)
	}
	if ae.code != "AIP_VALIDATION_FAILED" {
		t.Errorf("code = %q, want %q", ae.code, "AIP_VALIDATION_FAILED")
	}
}

func TestAddAppAIPExecFails(t *testing.T) {
	// Build a mock binary that exits non-zero with no stdout for "status"
	dir := t.TempDir()
	mockSrc := filepath.Join(dir, "fail-app.go")
	os.WriteFile(mockSrc, []byte(`package main

import "os"

func main() {
	os.Exit(1)
}
`), 0644)

	mockBin := filepath.Join(dir, "fail-app")
	if runtime.GOOS == "windows" {
		mockBin += ".exe"
	}

	build := exec.Command("go", "build", "-o", mockBin, mockSrc)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building mock binary: %v\n%s", err, out)
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
	defer os.Setenv("PATH", origPath)

	cfgDir := t.TempDir()
	cfg := &config.Config{
		Apps: []string{},
		Path: filepath.Join(cfgDir, config.FileName),
	}
	cfg.Save()

	err := addApp(cfg, "fail-app")
	if err == nil {
		t.Fatal("expected AIP validation error")
	}
	ae, ok := err.(*addError)
	if !ok {
		t.Fatalf("expected *addError, got %T", err)
	}
	if ae.code != "AIP_VALIDATION_FAILED" {
		t.Errorf("code = %q, want %q", ae.code, "AIP_VALIDATION_FAILED")
	}
}

func TestAddAppPreservesExistingApps(t *testing.T) {
	// Build a valid mock binary
	dir := t.TempDir()
	mockSrc := filepath.Join(dir, "new-app.go")
	os.WriteFile(mockSrc, []byte(`package main

import "fmt"

func main() {
	fmt.Println("{\"status\": \"ok\"}")
}
`), 0644)

	mockBin := filepath.Join(dir, "new-app")
	if runtime.GOOS == "windows" {
		mockBin += ".exe"
	}

	build := exec.Command("go", "build", "-o", mockBin, mockSrc)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building mock binary: %v\n%s", err, out)
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
	defer os.Setenv("PATH", origPath)

	cfgDir := t.TempDir()
	cfg := &config.Config{
		Apps: []string{"existing-app"},
		Path: filepath.Join(cfgDir, config.FileName),
	}
	cfg.Save()

	err := addApp(cfg, "new-app")
	if err != nil {
		t.Fatalf("addApp() error: %v", err)
	}

	loaded, err := config.Load(cfgDir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !loaded.HasApp("existing-app") {
		t.Error("existing app was lost")
	}
	if !loaded.HasApp("new-app") {
		t.Error("new app was not added")
	}
}
