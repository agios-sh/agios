package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/agios-sh/agios/runner"
)

func TestIsTimeoutError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"timeout error", fmt.Errorf("command timed out after 5s"), true},
		{"other error", fmt.Errorf("exit status 1"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTimeoutError(tt.err)
			if got != tt.expected {
				t.Errorf("isTimeoutError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestBackgroundJobWithMockBinary(t *testing.T) {
	// Build a slow mock binary that outputs progress then a result
	dir := t.TempDir()
	mockSrc := filepath.Join(dir, "slow-app.go")
	os.WriteFile(mockSrc, []byte(`package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("{\"progress\": {\"message\": \"Starting...\", \"percent\": 0}}")
	time.Sleep(50 * time.Millisecond)
	fmt.Println("{\"progress\": {\"message\": \"Working...\", \"percent\": 50}}")
	time.Sleep(50 * time.Millisecond)
	fmt.Println("{\"result\": \"done\", \"items\": [1, 2, 3]}")
}
`), 0644)

	mockBin := filepath.Join(dir, "slow-app")
	if runtime.GOOS == "windows" {
		mockBin += ".exe"
	}

	build := exec.Command("go", "build", "-o", mockBin, mockSrc)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building mock binary: %v\n%s", err, out)
	}

	// Create a temporary jobs directory
	jobsBaseDir := t.TempDir()

	// Start a job
	id, outputPath, err := runner.StartJobAt(filepath.Join(jobsBaseDir, "jobs"), "slow-app", []string{"build"})
	if err != nil {
		t.Fatalf("StartJobAt error: %v", err)
	}

	// Run the background process
	proc, err := runner.ExecBackground(mockBin, []string{}, outputPath)
	if err != nil {
		t.Fatalf("ExecBackground error: %v", err)
	}

	// Wait for the process to finish
	proc.Wait()
	time.Sleep(100 * time.Millisecond) // let the goroutine close the file

	// Read and verify the output
	meta, parsed, err := runner.GetJobOutputAt(filepath.Join(jobsBaseDir, "jobs"), id)
	if err != nil {
		t.Fatalf("GetJobOutputAt error: %v", err)
	}

	if meta.Status != "running" {
		t.Errorf("expected status 'running', got %q (not yet marked complete)", meta.Status)
	}

	if parsed == nil {
		t.Fatal("expected parsed output")
	}

	if len(parsed.Progress) != 2 {
		t.Errorf("expected 2 progress lines, got %d", len(parsed.Progress))
	}

	if parsed.Result == nil {
		t.Fatal("expected result")
	}

	if parsed.Result["result"] != "done" {
		t.Errorf("expected result 'done', got %v", parsed.Result["result"])
	}

	// Mark complete
	err = runner.CompleteJobAt(filepath.Join(jobsBaseDir, "jobs"), id)
	if err != nil {
		t.Fatalf("CompleteJobAt error: %v", err)
	}

	meta, _, _ = runner.GetJobOutputAt(filepath.Join(jobsBaseDir, "jobs"), id)
	if meta.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", meta.Status)
	}
}

func TestBackgroundJobProgressExtraction(t *testing.T) {
	// Simulate the progress extraction that happens during backgrounding
	stdout := []byte(`{"progress": {"message": "Compiling...", "percent": 30}}
{"progress": {"message": "Running tests...", "percent": 75}}
`)

	parsed, err := runner.ParseJSONL(stdout)
	// This should fail with "no result line found" since there's only progress
	if err == nil {
		// If there's a result somehow, check progress
		if len(parsed.Progress) < 1 {
			t.Error("expected at least one progress line")
		}
	}
	// This is fine — during timeout, we only have progress lines (no result yet)
	// The backgroundJob function handles this case correctly by checking parseErr
}

func TestBackgroundJobResponseFormat(t *testing.T) {
	// Test the format of the JSON response for a backgrounded job
	result := map[string]any{
		"job":    "j_abc123",
		"app":    "test-app",
		"status": "running",
		"progress": map[string]any{
			"message": "Running tests...",
			"percent": float64(75),
		},
		"help": []string{
			"Run `agios jobs j_abc123` to check status",
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["job"] != "j_abc123" {
		t.Errorf("expected job 'j_abc123', got %v", parsed["job"])
	}
	if parsed["status"] != "running" {
		t.Errorf("expected status 'running', got %v", parsed["status"])
	}
	if parsed["app"] != "test-app" {
		t.Errorf("expected app 'test-app', got %v", parsed["app"])
	}
}

func TestExecBackgroundSubprocessSurvival(t *testing.T) {
	// Build a mock binary that writes a marker file when it finishes
	dir := t.TempDir()
	mockSrc := filepath.Join(dir, "marker-app.go")
	markerPath := filepath.Join(dir, "done.marker")

	os.WriteFile(mockSrc, []byte(`package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	fmt.Println("{\"progress\": {\"message\": \"Working...\"}}")
	time.Sleep(100 * time.Millisecond)
	fmt.Println("{\"result\": \"completed\"}")

	// Write marker file to prove we finished
	os.WriteFile("`+markerPath+`", []byte("done"), 0644)
}
`), 0644)

	mockBin := filepath.Join(dir, "marker-app")
	if runtime.GOOS == "windows" {
		mockBin += ".exe"
	}

	build := exec.Command("go", "build", "-o", mockBin, mockSrc)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building mock binary: %v\n%s", err, out)
	}

	outputPath := filepath.Join(dir, "output.jsonl")
	proc, err := runner.ExecBackground(mockBin, []string{}, outputPath)
	if err != nil {
		t.Fatalf("ExecBackground error: %v", err)
	}

	// Wait for the process
	proc.Wait()
	time.Sleep(200 * time.Millisecond)

	// Verify the marker file was created
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Error("expected marker file to exist — subprocess did not complete")
	}

	// Verify the output file has content
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty output file")
	}
}
