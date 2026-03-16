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

const JobCleanupTTL = 24 * time.Hour

type JobMeta struct {
	ID        string    `json:"id"`
	App       string    `json:"app"`
	Command   []string  `json:"command"`
	StartTime time.Time `json:"start_time"`
	Status    string    `json:"status"` // "running" or "completed"
}

type JobInfo struct {
	ID        string    `json:"id"`
	App       string    `json:"app"`
	Command   []string  `json:"command"`
	StartTime time.Time `json:"start_time"`
	Status    string    `json:"status"`
}

func jobsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".agios", "jobs"), nil
}

func jobsDirAt(baseDir string) string {
	return filepath.Join(baseDir, "jobs")
}

func generateJobID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "j_" + hex.EncodeToString(b)
}

// StartJob creates a new job and returns the job ID and output file path.
func StartJob(app string, command []string) (string, string, error) {
	dir, err := jobsDir()
	if err != nil {
		return "", "", err
	}
	return StartJobAt(dir, app, command)
}

// StartJobAt is like StartJob but uses a custom jobs directory (for testing).
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

func CompleteJob(jobID string) error {
	dir, err := jobsDir()
	if err != nil {
		return err
	}
	return CompleteJobAt(dir, jobID)
}

func CompleteJobAt(baseJobsDir string, jobID string) error {
	jobDir := filepath.Join(baseJobsDir, jobID)
	meta, err := readJobMeta(jobDir)
	if err != nil {
		return err
	}
	meta.Status = "completed"
	return writeJobMeta(jobDir, meta)
}

func ListJobs() ([]JobInfo, error) {
	dir, err := jobsDir()
	if err != nil {
		return nil, err
	}
	return ListJobsAt(dir)
}

// ListJobsAt returns all jobs, auto-detecting completion from output files.
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

	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].StartTime.After(jobs[j].StartTime)
	})

	if jobs == nil {
		jobs = []JobInfo{}
	}

	return jobs, nil
}

func GetJobOutput(jobID string) (*JobMeta, *ParsedOutput, error) {
	dir, err := jobsDir()
	if err != nil {
		return nil, nil, err
	}
	return GetJobOutputAt(dir, jobID)
}

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
			return meta, nil, nil
		}
		return nil, nil, fmt.Errorf("reading job output: %w", err)
	}

	if len(data) == 0 {
		return meta, nil, nil
	}

	parsed, parseErr := ParseJSONL(data)
	if parseErr != nil {
			return meta, nil, nil
	}

	return meta, parsed, nil
}

func CleanupCompletedJobs() error {
	dir, err := jobsDir()
	if err != nil {
		return err
	}
	return CleanupCompletedJobsAt(dir, time.Now())
}

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

// autoDetectCompletion checks if a running job's output contains a final result,
// persists the completed status if so, and returns true.
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
	persistedMeta := *meta
	persistedMeta.Status = "completed"
	_ = writeJobMeta(jobDir, &persistedMeta) // best-effort persist
	return true
}

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
