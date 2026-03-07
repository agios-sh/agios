package runner

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// JobCleanupTTL is the duration after which completed jobs are cleaned up.
const JobCleanupTTL = 24 * time.Hour

// JobMeta holds metadata about a backgrounded job.
type JobMeta struct {
	ID        string    `json:"id"`
	App       string    `json:"app"`
	Command   []string  `json:"command"`
	StartTime time.Time `json:"start_time"`
	Status    string    `json:"status"` // "running" or "completed"
}

// JobResult represents the response returned when a job is backgrounded.
type JobResult struct {
	JobID    string         `json:"job"`
	App      string         `json:"app"`
	Status   string         `json:"status"`
	Progress map[string]any `json:"progress,omitempty"`
	Help     []string       `json:"help"`
}

// JobInfo represents a job listing entry.
type JobInfo struct {
	ID        string    `json:"id"`
	App       string    `json:"app"`
	Command   []string  `json:"command"`
	StartTime time.Time `json:"start_time"`
	Status    string    `json:"status"`
}

// jobsDir returns the path to ~/.agios/jobs/.
func jobsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".agios", "jobs"), nil
}

// jobsDirAt returns the path to the jobs directory under a custom base dir.
func jobsDirAt(baseDir string) string {
	return filepath.Join(baseDir, "jobs")
}

// generateJobID creates a new job ID in the format j_<random hex>.
func generateJobID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "j_" + hex.EncodeToString(b)
}

// StartJob creates a new job directory and writes the initial metadata.
// It returns the job ID and the path to the output file.
func StartJob(app string, command []string) (string, string, error) {
	dir, err := jobsDir()
	if err != nil {
		return "", "", err
	}
	return StartJobAt(dir, app, command)
}

// StartJobAt creates a new job under a custom jobs directory.
func StartJobAt(baseJobsDir string, app string, command []string) (string, string, error) {
	id := generateJobID()
	jobDir := filepath.Join(baseJobsDir, id)

	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return "", "", fmt.Errorf("creating job directory: %w", err)
	}

	meta := JobMeta{
		ID:        id,
		App:       app,
		Command:   command,
		StartTime: time.Now(),
		Status:    "running",
	}

	if err := writeJobMeta(jobDir, &meta); err != nil {
		return "", "", err
	}

	outputPath := filepath.Join(jobDir, "output.jsonl")
	return id, outputPath, nil
}

// CompleteJob marks a job as completed by updating its metadata.
func CompleteJob(jobID string) error {
	dir, err := jobsDir()
	if err != nil {
		return err
	}
	return CompleteJobAt(dir, jobID)
}

// CompleteJobAt marks a job as completed under a custom jobs directory.
func CompleteJobAt(baseJobsDir string, jobID string) error {
	jobDir := filepath.Join(baseJobsDir, jobID)
	meta, err := readJobMeta(jobDir)
	if err != nil {
		return err
	}
	meta.Status = "completed"
	return writeJobMeta(jobDir, meta)
}

// ListJobs returns all jobs (active and completed).
func ListJobs() ([]JobInfo, error) {
	dir, err := jobsDir()
	if err != nil {
		return nil, err
	}
	return ListJobsAt(dir)
}

// ListJobsAt returns all jobs under a custom jobs directory.
// It auto-detects completed jobs by checking if their output contains a final result.
func ListJobsAt(baseJobsDir string) ([]JobInfo, error) {
	entries, err := os.ReadDir(baseJobsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []JobInfo{}, nil
		}
		return nil, fmt.Errorf("reading jobs directory: %w", err)
	}

	var jobs []JobInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		jobDir := filepath.Join(baseJobsDir, entry.Name())
		meta, err := readJobMeta(jobDir)
		if err != nil {
			continue // skip corrupted jobs
		}

		// Auto-detect completion for running jobs
		if meta.Status == "running" {
			if autoDetectCompletion(jobDir, meta) {
				meta.Status = "completed"
			}
		}

		jobs = append(jobs, JobInfo{
			ID:        meta.ID,
			App:       meta.App,
			Command:   meta.Command,
			StartTime: meta.StartTime,
			Status:    meta.Status,
		})
	}

	// Sort by start time, most recent first
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].StartTime.After(jobs[j].StartTime)
	})

	if jobs == nil {
		jobs = []JobInfo{}
	}

	return jobs, nil
}

// GetJobOutput reads the output file of a job. It returns the latest progress
// (if any) and the final result (if completed).
func GetJobOutput(jobID string) (*JobMeta, *ParsedOutput, error) {
	dir, err := jobsDir()
	if err != nil {
		return nil, nil, err
	}
	return GetJobOutputAt(dir, jobID)
}

// GetJobOutputAt reads the output of a job under a custom jobs directory.
func GetJobOutputAt(baseJobsDir string, jobID string) (*JobMeta, *ParsedOutput, error) {
	jobDir := filepath.Join(baseJobsDir, jobID)

	meta, err := readJobMeta(jobDir)
	if err != nil {
		return nil, nil, fmt.Errorf("job %q not found", jobID)
	}

	outputPath := filepath.Join(jobDir, "output.jsonl")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Job exists but no output yet
			return meta, nil, nil
		}
		return nil, nil, fmt.Errorf("reading job output: %w", err)
	}

	if len(data) == 0 {
		return meta, nil, nil
	}

	parsed, parseErr := ParseJSONL(data)
	if parseErr != nil {
		// Return raw data as best effort
		return meta, nil, nil
	}

	return meta, parsed, nil
}

// CleanupCompletedJobs removes completed jobs older than the cleanup TTL.
func CleanupCompletedJobs() error {
	dir, err := jobsDir()
	if err != nil {
		return err
	}
	return CleanupCompletedJobsAt(dir, time.Now())
}

// CleanupCompletedJobsAt removes completed jobs older than the cleanup TTL
// under a custom jobs directory.
func CleanupCompletedJobsAt(baseJobsDir string, now time.Time) error {
	entries, err := os.ReadDir(baseJobsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading jobs directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		jobDir := filepath.Join(baseJobsDir, entry.Name())
		meta, err := readJobMeta(jobDir)
		if err != nil {
			continue
		}
		if meta.Status == "completed" && now.Sub(meta.StartTime) > JobCleanupTTL {
			os.RemoveAll(jobDir)
		}
	}

	return nil
}

// autoDetectCompletion checks if a running job's output file contains a final
// result line. If so, it marks the job as completed and returns true.
func autoDetectCompletion(jobDir string, meta *JobMeta) bool {
	outputPath := filepath.Join(jobDir, "output.jsonl")
	data, err := os.ReadFile(outputPath)
	if err != nil || len(data) == 0 {
		return false
	}
	parsed, err := ParseJSONL(data)
	if err != nil || parsed.Result == nil {
		return false
	}
	// Output has a final result — mark as completed
	meta.Status = "completed"
	_ = writeJobMeta(jobDir, meta) // best-effort persist
	return true
}

// writeJobMeta writes the job metadata to meta.json in the job directory.
func writeJobMeta(jobDir string, meta *JobMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling job metadata: %w", err)
	}
	metaPath := filepath.Join(jobDir, "meta.json")
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("writing job metadata: %w", err)
	}
	return nil
}

// readJobMeta reads the job metadata from meta.json in the job directory.
func readJobMeta(jobDir string) (*JobMeta, error) {
	metaPath := filepath.Join(jobDir, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("reading job metadata: %w", err)
	}
	var meta JobMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing job metadata: %w", err)
	}
	return &meta, nil
}
