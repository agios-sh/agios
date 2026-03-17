package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/agios-sh/agios/runner"
)

// RunApp handles the catch-all app routing: agios <app> <command> [args].
// Timed-out apps are automatically backgrounded as jobs.
func RunApp(appName string, args []string) {
	cfg := loadConfig()

	if !cfg.HasApp(appName) {
		emitError(
			"App \""+appName+"\" is not configured. Run `agios add "+appName+"` first.", "APP_NOT_CONFIGURED",
			fmt.Sprintf("Run `agios add %s` to register the app", appName),
		)
		os.Exit(1)
	}

	binPath, err := runner.Resolve(appName)
	if err != nil {
		emitError(
			"Binary \""+appName+"\" not found on PATH.", "BINARY_NOT_FOUND",
			"Ensure the binary is installed and available on your PATH",
		)
		os.Exit(1)
	}

	result, execErr := runner.Exec(binPath, args, runner.DefaultTimeout)

	if execErr != nil && isTimeoutError(execErr) {
		backgroundJob(appName, binPath, args, result)
		return
	}

	if result != nil && len(result.Stdout) == 0 && execErr != nil {
		stderrStr := string(result.Stderr)
		if stderrStr == "" {
			stderrStr = execErr.Error()
		}
		emitError(stderrStr, "APP_ERROR",
			fmt.Sprintf("Run `agios %s help` to see available commands", appName),
			"Run `agios status` to check app health",
		)
		os.Exit(1)
	}

	if result != nil && len(result.Stdout) > 0 {
		parsed, parseErr := runner.ParseJSONL(result.Stdout)
		if parseErr != nil {
			if invErr, ok := parseErr.(*runner.InvalidOutputError); ok {
				errResult := map[string]any{
					"error": "App returned invalid output: " + invErr.Message,
					"code":  "INVALID_OUTPUT",
					"raw":   invErr.Raw,
					"help": []string{
						"The app's output must be valid JSONL (one JSON object per line).",
						"The last non-progress line is treated as the final result.",
					},
				}
				writePipelinedJSON(errResult)
				os.Exit(1)
			}
			emitError("Failed to parse app output", "INVALID_OUTPUT",
				"The app's output must be valid JSONL (one JSON object per line).",
				"Run `agios status` to check app health",
			)
			os.Exit(1)
		}

		// Non-zero exit with an "error" field in valid JSON: pass through as-is
		if execErr != nil {
			if _, hasError := parsed.Result["error"]; hasError {
				writePipelinedJSON(parsed.Result)
				os.Exit(1)
			}
			emitError("App exited with error", "APP_ERROR",
				fmt.Sprintf("Run `agios %s help` to see available commands", appName),
				"Run `agios status` to check app health",
			)
			os.Exit(1)
		}

		writePipelinedJSON(parsed.Result)
		return
	}

	if execErr != nil {
		emitError("App execution failed", "APP_ERROR",
			fmt.Sprintf("Run `agios %s help` to see available commands", appName),
			"Run `agios status` to check app health",
		)
		os.Exit(1)
	}

	emitError("App produced no output", "INVALID_OUTPUT",
		fmt.Sprintf("Run `agios %s help` to see available commands", appName),
		"Run `agios status` to check app health",
	)
	os.Exit(1)
}

func isTimeoutError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "timed out")
}

// backgroundJob converts a timed-out command into a detached job, returning
// the job ID and any progress captured before the timeout.
func backgroundJob(appName, binPath string, args []string, result *runner.ExecResult) {
	jobID, outputPath, err := runner.StartJob(appName, args)
	if err != nil {
		emitError("Failed to create background job", "INTERNAL_ERROR",
			"Run `agios jobs` to see existing background jobs",
		)
		os.Exit(1)
	}

	_, err = runner.ExecBackground(binPath, args, outputPath)
	if err != nil {
		emitError("Failed to background command", "INTERNAL_ERROR",
			"Run `agios jobs` to see existing background jobs",
		)
		os.Exit(1)
	}

	jobResult := map[string]any{
		"job":    jobID,
		"app":    appName,
		"status": "running",
		"help": []string{
			fmt.Sprintf("Run `agios jobs %s` to check status", jobID),
		},
	}

	if result != nil && len(result.Stdout) > 0 {
		parsed, parseErr := runner.ParseJSONL(result.Stdout)
		if parseErr == nil {
			if p := latestProgress(parsed.Progress); p != nil {
				jobResult["progress"] = p
			}
		}
	}

	writePipelinedJSON(jobResult)
}
