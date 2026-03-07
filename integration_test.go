package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// testEnv holds the test environment with a compiled agios binary and mock app.
type testEnv struct {
	agiosBin string
	mockBin  string
	dir      string // working directory for tests
	origPath string
}

// setupTestEnv compiles both agios and the mock-app binary, sets up PATH,
// and returns a testEnv with cleanup. This is called once per test that needs it.
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	binDir := t.TempDir()
	workDir := t.TempDir()

	agiosBin := filepath.Join(binDir, "agios")
	mockBin := filepath.Join(binDir, "mock-app")
	if runtime.GOOS == "windows" {
		agiosBin += ".exe"
		mockBin += ".exe"
	}

	// Get the project root (where this test file lives)
	projectRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting project root: %v", err)
	}

	// Compile agios
	build := exec.Command("go", "build", "-o", agiosBin, ".")
	build.Dir = projectRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building agios: %v\n%s", err, out)
	}

	// Compile mock-app
	build = exec.Command("go", "build", "-o", mockBin, "./testdata/mock-app/")
	build.Dir = projectRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building mock-app: %v\n%s", err, out)
	}

	// Prepend binDir to PATH so both binaries are found
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)
	t.Cleanup(func() {
		os.Setenv("PATH", origPath)
	})

	return &testEnv{
		agiosBin: agiosBin,
		mockBin:  mockBin,
		dir:      workDir,
		origPath: origPath,
	}
}

// run executes the agios binary with the given args in the test working directory.
// Returns stdout, stderr, and any error.
func (e *testEnv) run(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	return e.runWithEnv(t, nil, args...)
}

// runWithEnv executes agios with extra environment variables.
// AGIOS_FORMAT=json is always set so integration tests can parse output as JSON.
func (e *testEnv) runWithEnv(t *testing.T, env []string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(e.agiosBin, args...)
	cmd.Dir = e.dir

	// Build env: inherit current env + extras + force JSON output for tests
	cmd.Env = append(os.Environ(), "AGIOS_FORMAT=json")
	for _, kv := range env {
		cmd.Env = append(cmd.Env, kv)
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// runWithStdin executes agios with stdin content.
func (e *testEnv) runWithStdin(t *testing.T, stdin string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(e.agiosBin, args...)
	cmd.Dir = e.dir
	cmd.Env = append(os.Environ(), "AGIOS_FORMAT=json")
	cmd.Stdin = strings.NewReader(stdin)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// parseJSON parses stdout as JSON into a map.
func parseJSON(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing JSON output: %v\nraw output: %s", err, stdout)
	}
	return result
}

// TestIntegrationInit tests agios init creates config and agent memory file.
func TestIntegrationInit(t *testing.T) {
	env := setupTestEnv(t)

	stdout, _, err := env.run(t, "init")
	if err != nil {
		t.Fatalf("agios init failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	if msg, ok := result["message"].(string); !ok || !strings.Contains(msg, "Initialized") {
		t.Errorf("expected initialized message, got: %v", result["message"])
	}

	// Verify agios.yaml was created
	configPath := filepath.Join(env.dir, "agios.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("agios.yaml not created: %v", err)
	}
	if !strings.Contains(string(data), "apps:") {
		t.Errorf("agios.yaml missing apps field: %s", data)
	}

	// Verify agent memory file was created
	agentsPath := filepath.Join(env.dir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		t.Error("AGENTS.md was not created")
	}

	// Verify CLAUDE.md symlink
	claudePath := filepath.Join(env.dir, "CLAUDE.md")
	link, err := os.Readlink(claudePath)
	if err != nil {
		t.Errorf("CLAUDE.md symlink not created: %v", err)
	} else if link != "AGENTS.md" {
		t.Errorf("CLAUDE.md symlink target = %q, want %q", link, "AGENTS.md")
	}
}

// TestIntegrationInitAlreadyExists tests that agios init errors if config exists.
func TestIntegrationInitAlreadyExists(t *testing.T) {
	env := setupTestEnv(t)

	// First init
	env.run(t, "init")

	// Second init should error
	stdout, _, err := env.run(t, "init")
	if err == nil {
		t.Fatal("expected error on second init")
	}

	result := parseJSON(t, stdout)
	if code, ok := result["code"].(string); !ok || code != "ALREADY_INITIALIZED" {
		t.Errorf("code = %q, want %q", code, "ALREADY_INITIALIZED")
	}
}

// TestIntegrationAddApp tests agios add validates and adds an app.
func TestIntegrationAddApp(t *testing.T) {
	env := setupTestEnv(t)

	// Init first
	env.run(t, "init")

	// Add mock-app
	stdout, _, err := env.run(t, "add", "mock-app")
	if err != nil {
		t.Fatalf("agios add mock-app failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	if msg, ok := result["message"].(string); !ok || !strings.Contains(msg, "mock-app") {
		t.Errorf("expected add confirmation, got: %v", result["message"])
	}

	// Verify agios.yaml was updated
	data, err := os.ReadFile(filepath.Join(env.dir, "agios.yaml"))
	if err != nil {
		t.Fatalf("reading agios.yaml: %v", err)
	}
	if !strings.Contains(string(data), "mock-app") {
		t.Errorf("agios.yaml does not contain mock-app: %s", data)
	}
}

// TestIntegrationAddAppAlreadyConfigured tests error when adding an already-configured app.
func TestIntegrationAddAppAlreadyConfigured(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")
	env.run(t, "add", "mock-app")

	// Adding again should error
	stdout, _, err := env.run(t, "add", "mock-app")
	if err == nil {
		t.Fatal("expected error for already-configured app")
	}

	result := parseJSON(t, stdout)
	if code, ok := result["code"].(string); !ok || code != "ALREADY_ADDED" {
		t.Errorf("code = %q, want %q", code, "ALREADY_ADDED")
	}
}

// TestIntegrationAppStatus tests agios <app> status routes correctly.
func TestIntegrationAppStatus(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")
	env.run(t, "add", "mock-app")

	stdout, _, err := env.run(t, "mock-app", "status")
	if err != nil {
		t.Fatalf("agios mock-app status failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	if name, ok := result["name"].(string); !ok || name != "mock-app" {
		t.Errorf("name = %q, want %q", name, "mock-app")
	}
	if status, ok := result["status"].(string); !ok || status != "ok" {
		t.Errorf("status = %q, want %q", status, "ok")
	}
	if version, ok := result["version"].(string); !ok || version != "1.0.0" {
		t.Errorf("version = %q, want %q", version, "1.0.0")
	}
}

// TestIntegrationAppList tests agios <app> list routes and validates output.
func TestIntegrationAppList(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")
	env.run(t, "add", "mock-app")

	stdout, _, err := env.run(t, "mock-app", "list")
	if err != nil {
		t.Fatalf("agios mock-app list failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	items, ok := result["items"].([]any)
	if !ok {
		t.Fatalf("expected items array, got: %v", result["items"])
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}

	// Verify help array is present
	help, ok := result["help"].([]any)
	if !ok || len(help) == 0 {
		t.Error("expected non-empty help array")
	}
}

// TestIntegrationHomeCommand tests agios (no args) shows per-app peek data.
func TestIntegrationHomeCommand(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")
	env.run(t, "add", "mock-app")

	stdout, _, err := env.run(t)
	if err != nil {
		t.Fatalf("agios (home) failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	apps, ok := result["apps"].([]any)
	if !ok {
		t.Fatalf("expected apps array, got: %v", result["apps"])
	}
	if len(apps) < 1 {
		t.Fatalf("expected at least 1 app, got %d", len(apps))
	}

	// Find mock-app and verify it has inline peek data
	found := false
	for _, a := range apps {
		entry := a.(map[string]any)
		if entry["name"] == "mock-app" {
			found = true
			// Verify peek data is inline
			peekData, ok := entry["peek"].(map[string]any)
			if !ok {
				t.Fatalf("expected peek data for mock-app, got: %v", entry["peek"])
			}
			if peekData["unread_count"].(float64) != 3 {
				t.Errorf("unread_count = %v, want 3", peekData["unread_count"])
			}
			break
		}
	}
	if !found {
		t.Fatalf("mock-app not found in apps: %v", apps)
	}

	// No top-level "notifications" key
	if _, ok := result["notifications"]; ok {
		t.Error("expected no top-level notifications key")
	}
}

// TestIntegrationStatus tests agios status concurrent health check.
func TestIntegrationStatus(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")
	env.run(t, "add", "mock-app")

	stdout, _, err := env.run(t, "status")
	if err != nil {
		t.Fatalf("agios status failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	apps, ok := result["apps"].([]any)
	if !ok || len(apps) != 4 {
		t.Fatalf("expected 4 apps in status (including browser, terminal, and tasks), got: %v", result["apps"])
	}

	app := apps[0].(map[string]any)
	if app["name"] != "mock-app" {
		t.Errorf("name = %q, want %q", app["name"], "mock-app")
	}
	if app["status"] != "ok" {
		t.Errorf("status = %q, want %q", app["status"], "ok")
	}
	if app["version"] != "1.0.0" {
		t.Errorf("version = %q, want %q", app["version"], "1.0.0")
	}
	if app["user"] != "testuser@example.com" {
		t.Errorf("user = %q, want %q", app["user"], "testuser@example.com")
	}
}

// TestIntegrationHelp tests agios help returns valid JSON with commands.
func TestIntegrationHelp(t *testing.T) {
	env := setupTestEnv(t)

	// agios with no config should error (not fall back to help)
	stdout, _, err := env.run(t)
	if err == nil {
		t.Error("expected error when running agios without config")
	}
	errResult := parseJSON(t, stdout)
	if errResult["code"] != "NO_CONFIG" {
		t.Errorf("expected NO_CONFIG error, got: %v", errResult["code"])
	}

	stdout, _, err = env.run(t, "help")
	if err != nil {
		t.Fatalf("agios help failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	if _, ok := result["usage"]; !ok {
		t.Error("expected usage field in help output")
	}
	commands, ok := result["commands"].([]any)
	if !ok || len(commands) == 0 {
		t.Error("expected non-empty commands array")
	}
}

// TestIntegrationAppNotConfigured tests error when app is not in config.
func TestIntegrationAppNotConfigured(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")

	stdout, _, err := env.run(t, "unknown-app", "status")
	if err == nil {
		t.Fatal("expected error for unconfigured app")
	}

	result := parseJSON(t, stdout)
	if code := result["code"]; code != "APP_NOT_CONFIGURED" {
		t.Errorf("code = %q, want %q", code, "APP_NOT_CONFIGURED")
	}
}

// TestIntegrationBinaryNotFound tests error when binary is not on PATH.
func TestIntegrationBinaryNotFound(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")

	// Manually add a non-existent app to config
	configPath := filepath.Join(env.dir, "agios.yaml")
	os.WriteFile(configPath, []byte("apps:\n- nonexistent-binary-xyz\n"), 0644)

	stdout, _, err := env.run(t, "nonexistent-binary-xyz", "status")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}

	result := parseJSON(t, stdout)
	if code := result["code"]; code != "BINARY_NOT_FOUND" {
		t.Errorf("code = %q, want %q", code, "BINARY_NOT_FOUND")
	}
}

// TestIntegrationInvalidOutput tests error when app returns invalid JSON.
func TestIntegrationInvalidOutput(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")
	env.run(t, "add", "mock-app")

	stdout, _, err := env.runWithEnv(t, []string{"MOCK_INVALID_JSON=1"}, "mock-app", "list")
	if err == nil {
		t.Fatal("expected error for invalid JSON output")
	}

	result := parseJSON(t, stdout)
	if code := result["code"]; code != "INVALID_OUTPUT" {
		t.Errorf("code = %q, want %q", code, "INVALID_OUTPUT")
	}
	if raw, ok := result["raw"].(string); !ok || raw == "" {
		t.Error("expected raw output in error response")
	}
}

// TestIntegrationNoConfig tests error when no agios.yaml exists.
func TestIntegrationNoConfig(t *testing.T) {
	env := setupTestEnv(t)

	stdout, _, err := env.run(t, "mock-app", "status")
	if err == nil {
		t.Fatal("expected error for missing config")
	}

	result := parseJSON(t, stdout)
	if code := result["code"]; code != "NO_CONFIG" {
		t.Errorf("code = %q, want %q", code, "NO_CONFIG")
	}
}

// TestIntegrationLargeValueTruncation tests that large string values are spilled to files.
func TestIntegrationLargeValueTruncation(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")
	env.run(t, "add", "mock-app")

	stdout, _, err := env.runWithEnv(t, []string{"MOCK_LARGE_VALUE=1"}, "mock-app", "list")
	if err != nil {
		t.Fatalf("agios mock-app list (large value) failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	largeField, ok := result["large_field"].(string)
	if !ok {
		t.Fatalf("expected large_field string, got: %v", result["large_field"])
	}

	// Value should be truncated to a file reference
	if !strings.Contains(largeField, "[truncated: see ") {
		t.Errorf("expected truncated reference, got: %s", largeField[:min(100, len(largeField))])
	}

	// Extract file path and verify file exists
	// Format: [truncated: see /path/to/file.txt]
	path := strings.TrimPrefix(largeField, "[truncated: see ")
	path = strings.TrimSuffix(path, "]")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading spilled file: %v", err)
	}
	if len(data) != 5000 {
		t.Errorf("spilled file size = %d, want 5000", len(data))
	}
}

// TestIntegrationStdinForwarding tests that stdin is forwarded to apps.
func TestIntegrationStdinForwarding(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")
	env.run(t, "add", "mock-app")

	// Set MOCK_STDIN_ECHO so mock-app reads and echoes stdin
	os.Setenv("MOCK_STDIN_ECHO", "1")
	defer os.Unsetenv("MOCK_STDIN_ECHO")

	stdinContent := "hello from stdin"
	stdout, _, err := env.runWithStdin(t, stdinContent, "mock-app", "list")
	if err != nil {
		t.Fatalf("agios mock-app list (stdin) failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	if echoed, ok := result["stdin_content"].(string); !ok || echoed != stdinContent {
		t.Errorf("stdin_content = %q, want %q", echoed, stdinContent)
	}
}

// TestIntegrationJobBackgrounding tests that slow commands are backgrounded as jobs.
func TestIntegrationJobBackgrounding(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")
	env.run(t, "add", "mock-app")

	// Use a slow response that exceeds the default 5s timeout
	// The mock-app sleeps for 8s, which will trigger backgrounding
	stdout, _, _ := env.runWithEnv(t, []string{"MOCK_SLOW=8s", "MOCK_PROGRESS=1"}, "mock-app", "list")

	result := parseJSON(t, stdout)

	// Should get a job response
	jobID, ok := result["job"].(string)
	if !ok || !strings.HasPrefix(jobID, "j_") {
		t.Fatalf("expected job ID starting with 'j_', got: %v", result["job"])
	}
	if result["status"] != "running" {
		t.Errorf("status = %q, want %q", result["status"], "running")
	}
	if result["app"] != "mock-app" {
		t.Errorf("app = %q, want %q", result["app"], "mock-app")
	}

	// List jobs — should show the backgrounded job
	stdout, _, err := env.run(t, "jobs")
	if err != nil {
		t.Fatalf("agios jobs failed: %v\nstdout: %s", err, stdout)
	}

	jobsResult := parseJSON(t, stdout)
	jobs, ok := jobsResult["jobs"].([]any)
	if !ok || len(jobs) == 0 {
		t.Fatalf("expected at least 1 job, got: %v", jobsResult["jobs"])
	}

	// Check job by ID
	stdout, _, err = env.run(t, "jobs", jobID)
	if err != nil {
		t.Fatalf("agios jobs %s failed: %v\nstdout: %s", jobID, err, stdout)
	}

	jobResult := parseJSON(t, stdout)
	if jobResult["job"] != jobID {
		t.Errorf("job = %q, want %q", jobResult["job"], jobID)
	}

	// Wait for the job to complete (the background process should finish within ~8s total)
	// Poll until completed or timeout
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		stdout, _, _ = env.run(t, "jobs", jobID)
		jobResult = parseJSON(t, stdout)
		if jobResult["status"] == "completed" {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if jobResult["status"] != "completed" {
		t.Errorf("job status = %q, want %q (job may still be running)", jobResult["status"], "completed")
	}

	// Completed job should have the result
	if _, hasResult := jobResult["result"]; jobResult["status"] == "completed" && !hasResult {
		t.Error("completed job should have a result field")
	}
}

// TestIntegrationAppError tests that app errors are passed through correctly.
func TestIntegrationAppError(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")
	env.run(t, "add", "mock-app")

	stdout, _, err := env.runWithEnv(t, []string{"MOCK_ERROR=something went wrong"}, "mock-app", "list")
	if err == nil {
		t.Fatal("expected non-zero exit for app error")
	}

	result := parseJSON(t, stdout)
	if errMsg, ok := result["error"].(string); !ok || errMsg != "something went wrong" {
		t.Errorf("error = %q, want %q", errMsg, "something went wrong")
	}
}

// TestIntegrationProgressLines tests that progress lines are handled correctly.
func TestIntegrationProgressLines(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")
	env.run(t, "add", "mock-app")

	// mock-app with MOCK_PROGRESS=1 emits progress lines before the result
	stdout, _, err := env.runWithEnv(t, []string{"MOCK_PROGRESS=1"}, "mock-app", "list")
	if err != nil {
		t.Fatalf("agios mock-app list (progress) failed: %v\nstdout: %s", err, stdout)
	}

	// The output pipeline only returns the final result, not progress lines
	result := parseJSON(t, stdout)
	items, ok := result["items"].([]any)
	if !ok {
		t.Fatalf("expected items in final result, got: %v", result)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

// TestIntegrationRemoveApp tests agios remove removes an app.
func TestIntegrationRemoveApp(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")
	env.run(t, "add", "mock-app")

	stdout, _, err := env.run(t, "remove", "mock-app")
	if err != nil {
		t.Fatalf("agios remove mock-app failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	if msg, ok := result["message"].(string); !ok || !strings.Contains(msg, "mock-app") {
		t.Errorf("expected removal confirmation, got: %v", result["message"])
	}

	// Verify app was removed from config
	data, _ := os.ReadFile(filepath.Join(env.dir, "agios.yaml"))
	if strings.Contains(string(data), "mock-app") {
		t.Errorf("agios.yaml still contains mock-app after removal")
	}
}

// TestIntegrationRemoveAppNotConfigured tests error when removing a non-configured app.
func TestIntegrationRemoveAppNotConfigured(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")

	stdout, _, err := env.run(t, "remove", "nonexistent")
	if err == nil {
		t.Fatal("expected error for removing non-configured app")
	}

	result := parseJSON(t, stdout)
	if code := result["code"]; code != "NOT_CONFIGURED" {
		t.Errorf("code = %q, want %q", code, "NOT_CONFIGURED")
	}
}

// TestIntegrationVersion tests agios --version.
func TestIntegrationVersion(t *testing.T) {
	env := setupTestEnv(t)

	stdout, _, err := env.run(t, "--version")
	if err != nil {
		t.Fatalf("agios --version failed: %v", err)
	}

	result := parseJSON(t, stdout)
	if result["version"] != "dev" {
		t.Errorf("version = %q, want %q", result["version"], "dev")
	}
}

// TestIntegrationHelpFlags tests that --help and -h work like agios help.
func TestIntegrationHelpFlags(t *testing.T) {
	env := setupTestEnv(t)

	for _, flag := range []string{"--help", "-h"} {
		stdout, _, err := env.run(t, flag)
		if err != nil {
			t.Fatalf("agios %s failed: %v\nstdout: %s", flag, err, stdout)
		}

		result := parseJSON(t, stdout)
		if _, ok := result["usage"]; !ok {
			t.Errorf("agios %s: expected usage field in output", flag)
		}
	}
}

// TestIntegrationEmptyPeek tests home command with empty peek data.
func TestIntegrationEmptyPeek(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")
	env.run(t, "add", "mock-app")

	// Set mock-app to return empty peek
	stdout, _, err := env.runWithEnv(t, []string{"MOCK_EMPTY_PEEK=1"})
	if err != nil {
		t.Fatalf("agios (home, empty peek) failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)

	// Find mock-app — peek should be empty or absent
	apps := result["apps"].([]any)
	for _, a := range apps {
		entry := a.(map[string]any)
		if entry["name"] == "mock-app" {
			// peek should be omitted or empty
			if peekData, ok := entry["peek"].(map[string]any); ok && len(peekData) > 0 {
				t.Errorf("expected empty peek for mock-app, got: %v", peekData)
			}
			break
		}
	}
}

// TestIntegrationStatusMissingBinary tests status with a missing app binary.
func TestIntegrationStatusMissingBinary(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")

	// Manually add a non-existent binary to config
	configPath := filepath.Join(env.dir, "agios.yaml")
	os.WriteFile(configPath, []byte("apps:\n- nonexistent-binary-xyz\n- mock-app\n"), 0644)

	stdout, _, err := env.run(t, "status")
	if err != nil {
		t.Fatalf("agios status failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	apps := result["apps"].([]any)
	if len(apps) != 5 {
		t.Fatalf("expected 5 apps (including browser, terminal, and tasks), got %d", len(apps))
	}

	// Find the missing app — it should have a warning
	for _, a := range apps {
		app := a.(map[string]any)
		if app["name"] == "nonexistent-binary-xyz" {
			if app["status"] != "error" {
				t.Errorf("missing app status = %q, want %q", app["status"], "error")
			}
			if errMsg, ok := app["error"].(string); !ok || errMsg == "" {
				t.Error("expected error for missing binary")
			}
		}
		if app["name"] == "mock-app" {
			if app["status"] != "ok" {
				t.Errorf("mock-app status = %q, want %q", app["status"], "ok")
			}
		}
	}
}

// TestIntegrationJobsList tests agios jobs with no running jobs.
func TestIntegrationJobsList(t *testing.T) {
	env := setupTestEnv(t)

	stdout, _, err := env.run(t, "jobs")
	if err != nil {
		t.Fatalf("agios jobs failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	jobs, ok := result["jobs"].([]any)
	if !ok {
		t.Fatalf("expected jobs array, got: %v", result["jobs"])
	}
	// May have jobs from other tests, but at minimum should be a valid array
	_ = jobs
}

// TestIntegrationGetAppItem tests agios <app> get <id> with arguments.
func TestIntegrationGetAppItem(t *testing.T) {
	env := setupTestEnv(t)
	env.run(t, "init")
	env.run(t, "add", "mock-app")

	stdout, _, err := env.run(t, "mock-app", "get", "42")
	if err != nil {
		t.Fatalf("agios mock-app get 42 failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	if result["id"] != "42" {
		t.Errorf("id = %q, want %q", result["id"], "42")
	}
	if title := result["title"].(string); !strings.Contains(title, "42") {
		t.Errorf("title should contain '42', got: %q", title)
	}
}

// TestIntegrationFullFlow tests the complete workflow end-to-end.
func TestIntegrationFullFlow(t *testing.T) {
	env := setupTestEnv(t)

	// 1. Init
	stdout, _, err := env.run(t, "init")
	if err != nil {
		t.Fatalf("Step 1 (init): %v\n%s", err, stdout)
	}
	parseJSON(t, stdout) // validate JSON

	// 2. Add mock-app
	stdout, _, err = env.run(t, "add", "mock-app")
	if err != nil {
		t.Fatalf("Step 2 (add): %v\n%s", err, stdout)
	}
	parseJSON(t, stdout)

	// 3. Status check
	stdout, _, err = env.run(t, "status")
	if err != nil {
		t.Fatalf("Step 3 (status): %v\n%s", err, stdout)
	}
	statusResult := parseJSON(t, stdout)
	apps := statusResult["apps"].([]any)
	if len(apps) < 1 || apps[0].(map[string]any)["status"] != "ok" {
		t.Errorf("Step 3: unexpected status: %v", statusResult)
	}

	// 4. Home command — should show per-app peek data
	stdout, _, err = env.run(t)
	if err != nil {
		t.Fatalf("Step 4 (home): %v\n%s", err, stdout)
	}
	homeResult := parseJSON(t, stdout)
	homeApps := homeResult["apps"].([]any)
	var mockAppEntry map[string]any
	for _, a := range homeApps {
		entry := a.(map[string]any)
		if entry["name"] == "mock-app" {
			mockAppEntry = entry
			break
		}
	}
	if mockAppEntry == nil {
		t.Fatal("Step 4: mock-app not found in home apps")
	}
	peekData, ok := mockAppEntry["peek"].(map[string]any)
	if !ok {
		t.Fatalf("Step 4: expected inline peek data for mock-app, got: %v", mockAppEntry["peek"])
	}
	if peekData["unread_count"].(float64) != 3 {
		t.Errorf("Step 4: expected unread_count=3, got %v", peekData["unread_count"])
	}

	// 5. Run peek command directly
	stdout, _, err = env.run(t, "mock-app", "peek")
	if err != nil {
		t.Fatalf("Step 5 (peek): %v\n%s", err, stdout)
	}
	peekResult := parseJSON(t, stdout)
	if _, ok := peekResult["recent_activity"]; !ok {
		t.Error("Step 5: expected recent_activity in peek output")
	}

	// 6. Use app command
	stdout, _, err = env.run(t, "mock-app", "list")
	if err != nil {
		t.Fatalf("Step 6 (list): %v\n%s", err, stdout)
	}
	listResult := parseJSON(t, stdout)
	if _, ok := listResult["items"]; !ok {
		t.Error("Step 6: expected items in list result")
	}

	// 7. Remove app
	stdout, _, err = env.run(t, "remove", "mock-app")
	if err != nil {
		t.Fatalf("Step 7 (remove): %v\n%s", err, stdout)
	}
	parseJSON(t, stdout)

	// 8. Verify app is gone
	stdout, _, err = env.run(t, "mock-app", "status")
	if err == nil {
		t.Error("Step 8: expected error after removal")
	}
	errResult := parseJSON(t, stdout)
	if errResult["code"] != "APP_NOT_CONFIGURED" {
		t.Errorf("Step 8: expected APP_NOT_CONFIGURED, got %v", errResult["code"])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestIntegrationTerminalHelp tests agios terminal help returns valid AIP JSON.
func TestIntegrationTerminalHelp(t *testing.T) {
	env := setupTestEnv(t)

	stdout, _, err := env.run(t, "terminal", "help")
	if err != nil {
		t.Fatalf("agios terminal help failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	if result["name"] != "terminal" {
		t.Errorf("name = %q, want %q", result["name"], "terminal")
	}
	if _, ok := result["commands"]; !ok {
		t.Error("expected commands in help output")
	}
}

// TestIntegrationTerminalStatus tests agios terminal status returns valid AIP JSON.
func TestIntegrationTerminalStatus(t *testing.T) {
	env := setupTestEnv(t)

	stdout, _, err := env.run(t, "terminal", "status")
	if err != nil {
		t.Fatalf("agios terminal status failed: %v\nstdout: %s", err, stdout)
	}

	result := parseJSON(t, stdout)
	if result["name"] != "terminal" {
		t.Errorf("name = %q, want %q", result["name"], "terminal")
	}
	if result["version"] != "1.0.0" {
		t.Errorf("version = %q, want %q", result["version"], "1.0.0")
	}
}

// TestIntegrationTerminalLifecycle tests the full terminal lifecycle:
// start → send → read → list → kill → quit.
func TestIntegrationTerminalLifecycle(t *testing.T) {
	env := setupTestEnv(t)

	// Ensure clean state
	env.run(t, "terminal", "quit")

	// 1. Start a session
	stdout, _, err := env.run(t, "terminal", "start", "--shell", "/bin/sh")
	if err != nil {
		t.Fatalf("terminal start failed: %v\nstdout: %s", err, stdout)
	}
	result := parseJSON(t, stdout)
	if result["message"] != "Session started" {
		t.Errorf("start message = %q, want %q", result["message"], "Session started")
	}
	sessionID := result["session_id"].(float64)
	if sessionID < 1 {
		t.Errorf("session_id = %v, want >= 1", sessionID)
	}

	// 2. Send a command
	stdout, _, err = env.run(t, "terminal", "send", "echo hello-test-123")
	if err != nil {
		t.Fatalf("terminal send failed: %v\nstdout: %s", err, stdout)
	}
	result = parseJSON(t, stdout)
	output, ok := result["output"].(string)
	if !ok {
		t.Fatalf("expected output string, got: %v", result["output"])
	}
	if !strings.Contains(output, "hello-test-123") {
		t.Errorf("output should contain 'hello-test-123', got: %q", output)
	}

	// 3. Send pwd to verify directory
	stdout, _, err = env.run(t, "terminal", "send", "pwd")
	if err != nil {
		t.Fatalf("terminal send pwd failed: %v\nstdout: %s", err, stdout)
	}
	result = parseJSON(t, stdout)
	output = result["output"].(string)
	// Output should contain a path (could be anything, just verify non-empty)
	if len(output) == 0 {
		t.Error("pwd output should not be empty")
	}

	// 4. Dock view — should show sessions
	stdout, _, err = env.run(t, "terminal")
	if err != nil {
		t.Fatalf("terminal (dock) failed: %v\nstdout: %s", err, stdout)
	}
	result = parseJSON(t, stdout)
	sessions, ok := result["sessions"].([]any)
	if !ok || len(sessions) == 0 {
		t.Fatalf("expected at least 1 session, got: %v", result["sessions"])
	}

	// 5. Kill the session
	stdout, _, err = env.run(t, "terminal", "kill")
	if err != nil {
		t.Fatalf("terminal kill failed: %v\nstdout: %s", err, stdout)
	}
	result = parseJSON(t, stdout)
	if result["message"] != "Session killed" {
		t.Errorf("kill message = %q, want %q", result["message"], "Session killed")
	}

	// 6. Dock view should show no sessions
	stdout, _, err = env.run(t, "terminal")
	if err != nil {
		t.Fatalf("terminal (dock, after kill) failed: %v\nstdout: %s", err, stdout)
	}
	result = parseJSON(t, stdout)
	sessions = result["sessions"].([]any)
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after kill, got %d", len(sessions))
	}

	// 7. Quit the server
	stdout, _, err = env.run(t, "terminal", "quit")
	if err != nil {
		t.Fatalf("terminal quit failed: %v\nstdout: %s", err, stdout)
	}
	result = parseJSON(t, stdout)
	if msg := result["message"].(string); !strings.Contains(msg, "stopped") {
		t.Errorf("quit message = %q, want something with 'stopped'", msg)
	}
}

// TestIntegrationTerminalIdempotent tests idempotency:
// quit when stopped, start when already running.
func TestIntegrationTerminalIdempotent(t *testing.T) {
	env := setupTestEnv(t)

	// Quit when not running should succeed
	stdout, _, err := env.run(t, "terminal", "quit")
	if err != nil {
		t.Fatalf("terminal quit (not running) failed: %v\nstdout: %s", err, stdout)
	}
	result := parseJSON(t, stdout)
	if _, ok := result["error"]; ok {
		t.Errorf("quit when not running should not error: %v", result)
	}

	// Dock view when not running — should show empty sessions
	stdout, _, err = env.run(t, "terminal")
	if err != nil {
		t.Fatalf("terminal (dock, not running) failed: %v\nstdout: %s", err, stdout)
	}
	result = parseJSON(t, stdout)
	sessions, ok := result["sessions"].([]any)
	if !ok || len(sessions) != 0 {
		t.Errorf("expected empty sessions list, got: %v", result["sessions"])
	}

	// Clean up
	env.run(t, "terminal", "quit")
}

// TestIntegrationTerminalAutoCreate tests that send auto-creates a session
// if none exists (prefer success over errors).
func TestIntegrationTerminalAutoCreate(t *testing.T) {
	env := setupTestEnv(t)

	// Ensure clean state
	env.run(t, "terminal", "quit")

	// Send without start — should auto-create session
	stdout, _, err := env.run(t, "terminal", "send", "echo auto-created")
	if err != nil {
		t.Fatalf("terminal send (auto-create) failed: %v\nstdout: %s", err, stdout)
	}
	result := parseJSON(t, stdout)
	output, ok := result["output"].(string)
	if !ok {
		t.Fatalf("expected output, got: %v", result)
	}
	if !strings.Contains(output, "auto-created") {
		t.Errorf("output should contain 'auto-created', got: %q", output)
	}

	// Clean up
	env.run(t, "terminal", "quit")
}

// Silence the "declared and not used" warning for the fmt package which is
// used in string formatting within test Fatalf/Errorf calls.
var _ = fmt.Sprintf
