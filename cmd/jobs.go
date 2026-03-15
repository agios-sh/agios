package cmd

import (
	"fmt"
	"os"

	"github.com/agios-sh/agios/runner"
)

// RunJobs implements the "agios jobs" and "agios jobs <id>" commands.
// With no arguments, it lists all jobs. With a job ID, it returns the
// job's latest progress or final result.
func RunJobs(args []string) {
	// Best-effort cleanup of old completed jobs
	_ = runner.CleanupCompletedJobs()

	if len(args) == 0 {
		listJobs()
		return
	}

	getJob(args[0])
}

// listJobs lists all active and completed jobs.
func listJobs() {
	jobs, err := runner.ListJobs()
	if err != nil {
		writeError("Failed to list jobs", "INTERNAL_ERROR",
			"Run `agios help` for usage information",
		)
		os.Exit(1)
	}

	writePipelinedJSON(map[string]any{
		"jobs": jobs,
		"help": []string{
			"Run `agios jobs <id>` to check status of a specific job",
		},
	})
}

// getJob returns the current progress or final result of a specific job.
func getJob(jobID string) {
	meta, parsed, err := runner.GetJobOutput(jobID)
	if err != nil {
		writeError(
			fmt.Sprintf("Job %q not found", jobID),
			"JOB_NOT_FOUND",
			"Run `agios jobs` to list all active and completed jobs",
		)
		os.Exit(1)
	}

	// Auto-detect completion: if the output has a final result line
	// and the job is still marked as "running", mark it as completed.
	if meta.Status == "running" && parsed != nil && parsed.Result != nil {
		_ = runner.CompleteJob(jobID) // best-effort
		meta.Status = "completed"
	}

	result := map[string]any{
		"job":    meta.ID,
		"app":    meta.App,
		"status": meta.Status,
	}

	if parsed != nil {
		// Include latest progress if available
		if p := latestProgress(parsed.Progress); p != nil {
			result["progress"] = p
		}
		// If job completed, include the final result
		if meta.Status == "completed" && parsed.Result != nil {
			result["result"] = parsed.Result
		}
	}

	if meta.Status == "running" {
		result["help"] = []string{
			fmt.Sprintf("Run `agios jobs %s` again to check for updates", meta.ID),
		}
	} else {
		result["help"] = []string{
			"Job has completed. The result is included above.",
		}
	}

	writePipelinedJSON(result)
}
