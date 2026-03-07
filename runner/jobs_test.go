package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStartJobCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	jobsDir := jobsDirAt(tmpDir)

	id, outputPath, err := StartJobAt(jobsDir, "test-app", []string{"list"})
	if err != nil {
		t.Fatalf("StartJobAt returned error: %v", err)
	}

	if id == "" {
		t.Fatal("expected non-empty job ID")
	}

	if len(id) < 3 || id[:2] != "j_" {
		t.Errorf("expected job ID starting with 'j_', got %q", id)
	}

	// Verify job directory was created
	jobDir := filepath.Join(jobsDir, id)
	if _, err := os.Stat(jobDir); os.IsNotExist(err) {
		t.Error("expected job directory to exist")
	}

	// Verify meta.json was created
	metaPath := filepath.Join(jobDir, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("expected meta.json to exist: %v", err)
	}

	var meta JobMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("invalid meta.json: %v", err)
	}

	if meta.ID != id {
		t.Errorf("expected meta ID %q, got %q", id, meta.ID)
	}
	if meta.App != "test-app" {
		t.Errorf("expected app 'test-app', got %q", meta.App)
	}
	if meta.Status != "running" {
		t.Errorf("expected status 'running', got %q", meta.Status)
	}

	// Verify output path is inside the job directory
	expectedOutput := filepath.Join(jobDir, "output.jsonl")
	if outputPath != expectedOutput {
		t.Errorf("expected output path %q, got %q", expectedOutput, outputPath)
	}
}

func TestCompleteJobUpdatesStatus(t *testing.T) {
	tmpDir := t.TempDir()
	jobsDir := jobsDirAt(tmpDir)

	id, _, err := StartJobAt(jobsDir, "test-app", []string{"build"})
	if err != nil {
		t.Fatalf("StartJobAt returned error: %v", err)
	}

	if err := CompleteJobAt(jobsDir, id); err != nil {
		t.Fatalf("CompleteJobAt returned error: %v", err)
	}

	// Read back and verify
	jobDir := filepath.Join(jobsDir, id)
	meta, err := readJobMeta(jobDir)
	if err != nil {
		t.Fatalf("readJobMeta returned error: %v", err)
	}

	if meta.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", meta.Status)
	}
}

func TestListJobsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	jobsDir := jobsDirAt(tmpDir)

	jobs, err := ListJobsAt(jobsDir)
	if err != nil {
		t.Fatalf("ListJobsAt returned error: %v", err)
	}

	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestListJobsNonexistentDir(t *testing.T) {
	jobs, err := ListJobsAt("/nonexistent/path/jobs")
	if err != nil {
		t.Fatalf("ListJobsAt returned error: %v", err)
	}

	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestListJobsMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	jobsDir := jobsDirAt(tmpDir)

	// Create two jobs
	id1, _, _ := StartJobAt(jobsDir, "app1", []string{"list"})
	time.Sleep(10 * time.Millisecond) // ensure different start times
	id2, _, _ := StartJobAt(jobsDir, "app2", []string{"build"})

	jobs, err := ListJobsAt(jobsDir)
	if err != nil {
		t.Fatalf("ListJobsAt returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Most recent first
	if jobs[0].ID != id2 {
		t.Errorf("expected first job ID %q, got %q", id2, jobs[0].ID)
	}
	if jobs[1].ID != id1 {
		t.Errorf("expected second job ID %q, got %q", id1, jobs[1].ID)
	}
}

func TestGetJobOutputNoOutput(t *testing.T) {
	tmpDir := t.TempDir()
	jobsDir := jobsDirAt(tmpDir)

	id, _, _ := StartJobAt(jobsDir, "test-app", []string{"list"})

	meta, parsed, err := GetJobOutputAt(jobsDir, id)
	if err != nil {
		t.Fatalf("GetJobOutputAt returned error: %v", err)
	}

	if meta.ID != id {
		t.Errorf("expected meta ID %q, got %q", id, meta.ID)
	}

	if parsed != nil {
		t.Error("expected nil parsed output for job with no output file")
	}
}

func TestGetJobOutputWithProgress(t *testing.T) {
	tmpDir := t.TempDir()
	jobsDir := jobsDirAt(tmpDir)

	id, outputPath, _ := StartJobAt(jobsDir, "test-app", []string{"build"})

	// Write progress lines to the output file
	output := `{"progress": {"message": "Compiling...", "percent": 50}}
{"progress": {"message": "Running tests...", "percent": 75}}
{"result": "ok", "items": [1, 2, 3]}
`
	os.WriteFile(outputPath, []byte(output), 0644)

	meta, parsed, err := GetJobOutputAt(jobsDir, id)
	if err != nil {
		t.Fatalf("GetJobOutputAt returned error: %v", err)
	}

	if meta.Status != "running" {
		t.Errorf("expected status 'running', got %q", meta.Status)
	}

	if parsed == nil {
		t.Fatal("expected non-nil parsed output")
	}

	if len(parsed.Progress) != 2 {
		t.Errorf("expected 2 progress lines, got %d", len(parsed.Progress))
	}

	if parsed.Result == nil {
		t.Fatal("expected non-nil result")
	}

	if parsed.Result["result"] != "ok" {
		t.Errorf("expected result 'ok', got %v", parsed.Result["result"])
	}
}

func TestGetJobOutputNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	jobsDir := jobsDirAt(tmpDir)

	_, _, err := GetJobOutputAt(jobsDir, "j_nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
}

func TestCleanupCompletedJobs(t *testing.T) {
	tmpDir := t.TempDir()
	jobsDir := jobsDirAt(tmpDir)

	// Create a completed job with an old start time
	id1, _, _ := StartJobAt(jobsDir, "old-app", []string{"list"})
	CompleteJobAt(jobsDir, id1)

	// Manually backdate the meta to make it appear old
	jobDir := filepath.Join(jobsDir, id1)
	meta, _ := readJobMeta(jobDir)
	meta.StartTime = time.Now().Add(-25 * time.Hour) // 25 hours ago
	writeJobMeta(jobDir, meta)

	// Create a running job (should not be cleaned up)
	id2, _, _ := StartJobAt(jobsDir, "running-app", []string{"build"})

	// Create a recently completed job (should not be cleaned up)
	id3, _, _ := StartJobAt(jobsDir, "recent-app", []string{"test"})
	CompleteJobAt(jobsDir, id3)

	// Run cleanup
	err := CleanupCompletedJobsAt(jobsDir, time.Now())
	if err != nil {
		t.Fatalf("CleanupCompletedJobsAt returned error: %v", err)
	}

	// Old completed job should be gone
	if _, err := os.Stat(filepath.Join(jobsDir, id1)); !os.IsNotExist(err) {
		t.Error("expected old completed job to be cleaned up")
	}

	// Running job should still exist
	if _, err := os.Stat(filepath.Join(jobsDir, id2)); os.IsNotExist(err) {
		t.Error("expected running job to still exist")
	}

	// Recent completed job should still exist
	if _, err := os.Stat(filepath.Join(jobsDir, id3)); os.IsNotExist(err) {
		t.Error("expected recent completed job to still exist")
	}
}

func TestCleanupNonexistentDir(t *testing.T) {
	err := CleanupCompletedJobsAt("/nonexistent/path/jobs", time.Now())
	if err != nil {
		t.Fatalf("expected no error for nonexistent dir, got: %v", err)
	}
}

func TestGenerateJobID(t *testing.T) {
	id1 := generateJobID()
	id2 := generateJobID()

	if id1 == id2 {
		t.Error("expected unique job IDs")
	}

	if len(id1) < 3 || id1[:2] != "j_" {
		t.Errorf("expected ID starting with 'j_', got %q", id1)
	}
}

func TestJobMetaCommand(t *testing.T) {
	tmpDir := t.TempDir()
	jobsDir := jobsDirAt(tmpDir)

	command := []string{"build", "--verbose", "--output", "dist"}
	id, _, _ := StartJobAt(jobsDir, "test-app", command)

	jobs, _ := ListJobsAt(jobsDir)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	if len(jobs[0].Command) != len(command) {
		t.Errorf("expected command %v, got %v", command, jobs[0].Command)
	}
	for i, arg := range command {
		if jobs[0].Command[i] != arg {
			t.Errorf("expected command[%d] = %q, got %q", i, arg, jobs[0].Command[i])
		}
	}

	_ = id // suppress unused
}
