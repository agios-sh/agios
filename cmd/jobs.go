package cmd

import (
	"fmt"
	"os"

	"github.com/agios-sh/agios/runner"
)

// RunJobs lists all jobs or shows a specific job's status.
func RunJobs(args []string) {
	_ = runner.CleanupCompletedJobs()

	if len(args) == 0 {
		listJobs()
		return
	}

	getJob(args[0])
}

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

	if meta.Status == "running" && parsed != nil && parsed.Result != nil {
		_ = runner.CompleteJob(jobID)
		meta.Status = "completed"
	}

	result := map[string]any{
		"job":    meta.ID,
		"app":    meta.App,
		"status": meta.Status,
	}

	if parsed != nil {
		if p := latestProgress(parsed.Progress); p != nil {
			result["progress"] = p
		}
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
