package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/agios-sh/agios/config"
	"github.com/agios-sh/agios/runner"
)

// RunApp handles the catch-all app routing: agios <app> <command> [args].
// It loads the config, verifies the app is registered, resolves the binary,
// executes it, parses the JSONL output, and writes the result to stdout.
// If the app exceeds the timeout, it is automatically backgrounded as a job.
func RunApp(appName string, args []string) {
	cwd, err := os.Getwd()
	if err != nil {
		writeError("Failed to get working directory", "INTERNAL_ERROR", err,
			"Run `agios help` for usage information",
		)
		os.Exit(1)
	}

	// Load config (walks up from cwd)
	cfg, err := config.Load(cwd)
	if err != nil {
		writeError("No agios.yaml found. Run `agios init` first.", "NO_CONFIG", err,
			"Run `agios init` to create a new agios.yaml",
		)
		os.Exit(1)
	}

	// Verify app is in the config
	if !cfg.HasApp(appName) {
		writeError(
			"App \""+appName+"\" is not configured. Run `agios add "+appName+"` first.",
			"APP_NOT_CONFIGURED",
			nil,
			fmt.Sprintf("Run `agios add %s` to register the app", appName),
		)
		os.Exit(1)
	}

	// Resolve binary on PATH
	binPath, err := runner.Resolve(appName)
	if err != nil {
		writeError(
			"Binary \""+appName+"\" not found on PATH.",
			"BINARY_NOT_FOUND",
			err,
			"Ensure the binary is installed and available on your PATH",
		)
		os.Exit(1)
	}

	// Execute the subprocess
	result, execErr := runner.Exec(binPath, args, runner.DefaultTimeout)

	// Check if the command timed out — if so, background it as a job
	if execErr != nil && isTimeoutError(execErr) {
		backgroundJob(appName, binPath, args, result)
		return
	}

	// If exec produced no stdout at all and failed, synthesize error from stderr
	if result != nil && len(result.Stdout) == 0 && execErr != nil {
		stderrStr := string(result.Stderr)
		if stderrStr == "" {
			stderrStr = execErr.Error()
		}
		writeError(stderrStr, "APP_ERROR", execErr,
			fmt.Sprintf("Run `agios %s help` to see available commands", appName),
			"Run `agios status` to check app health",
		)
		os.Exit(1)
	}

	// If we got stdout, try to parse it as JSONL
	if result != nil && len(result.Stdout) > 0 {
		parsed, parseErr := runner.ParseJSONL(result.Stdout)
		if parseErr != nil {
			// Invalid output — return protocol error with raw output
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
			writeError("Failed to parse app output", "INVALID_OUTPUT", parseErr,
				"The app's output must be valid JSONL (one JSON object per line).",
				"Run `agios status` to check app health",
			)
			os.Exit(1)
		}

		// If the app exited non-zero but returned valid JSON with an "error" field, pass through
		if execErr != nil {
			if _, hasError := parsed.Result["error"]; hasError {
				writePipelinedJSON(parsed.Result)
				os.Exit(1)
			}
			// Non-zero exit without error field in output
			writeError("App exited with error", "APP_ERROR", execErr,
				fmt.Sprintf("Run `agios %s help` to see available commands", appName),
				"Run `agios status` to check app health",
			)
			os.Exit(1)
		}

		// Success — write the final result through the output pipeline
		writePipelinedJSON(parsed.Result)
		return
	}

	// No output and no error — shouldn't happen, but handle gracefully
	if execErr != nil {
		writeError("App execution failed", "APP_ERROR", execErr,
			fmt.Sprintf("Run `agios %s help` to see available commands", appName),
			"Run `agios status` to check app health",
		)
		os.Exit(1)
	}

	writeError("App produced no output", "INVALID_OUTPUT", nil,
		fmt.Sprintf("Run `agios %s help` to see available commands", appName),
		"Run `agios status` to check app health",
	)
	os.Exit(1)
}

// isTimeoutError checks if the error is a command timeout.
func isTimeoutError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "timed out")
}

// backgroundJob backgrounds a timed-out command as a job. It starts a new
// detached subprocess, creates the job metadata, and returns immediately
// with the job ID and any progress captured so far.
func backgroundJob(appName, binPath string, args []string, result *runner.ExecResult) {
	// Create a new job
	jobID, outputPath, err := runner.StartJob(appName, args)
	if err != nil {
		writeError("Failed to create background job", "INTERNAL_ERROR", err,
			"Run `agios jobs` to see existing background jobs",
		)
		os.Exit(1)
	}

	// Start the subprocess in the background, writing output to the job file
	_, err = runner.ExecBackground(binPath, args, outputPath)
	if err != nil {
		writeError("Failed to background command", "INTERNAL_ERROR", err,
			"Run `agios jobs` to see existing background jobs",
		)
		os.Exit(1)
	}

	// Build the job response
	jobResult := map[string]any{
		"job":    jobID,
		"app":    appName,
		"status": "running",
		"help": []string{
			fmt.Sprintf("Run `agios jobs %s` to check status", jobID),
		},
	}

	// Include latest progress from the timed-out execution if available
	if result != nil && len(result.Stdout) > 0 {
		parsed, parseErr := runner.ParseJSONL(result.Stdout)
		if parseErr == nil && len(parsed.Progress) > 0 {
			last := parsed.Progress[len(parsed.Progress)-1]
			// Extract the inner progress value — ParseJSONL stores the full
			// line object {"progress": {...}}, but we want just the inner value.
			if inner, ok := last["progress"]; ok {
				jobResult["progress"] = inner
			} else {
				jobResult["progress"] = last
			}
		}
	}

	writePipelinedJSON(jobResult)
}
