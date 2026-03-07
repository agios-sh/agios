package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/agios-sh/agios/config"
	"github.com/agios-sh/agios/runner"
)

// RunAdd implements the "agios add <app>" command.
// It validates the binary exists on $PATH, runs "<app> status" to verify AIP compliance,
// then adds the app to agios.yaml.
func RunAdd(args []string) {
	if len(args) == 0 {
		writeError("Usage: agios add <app>", "INVALID_ARGS", nil,
			"Run `agios help` for usage information",
		)
		os.Exit(1)
	}
	appName := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		writeError("Failed to get working directory", "INTERNAL_ERROR", err,
			"Run `agios help` for usage information",
		)
		os.Exit(1)
	}

	// Load config
	cfg, err := config.Load(cwd)
	if err != nil {
		writeError("No agios.yaml found. Run `agios init` first.", "NO_CONFIG", err,
			"Run `agios init` to create a new agios.yaml",
		)
		os.Exit(1)
	}

	if err := addApp(cfg, appName); err != nil {
		if ae, ok := err.(*addError); ok {
			writeError(ae.msg, ae.code, ae.cause,
				"Ensure the app binary is installed and on your PATH",
			)
		} else {
			writeError(err.Error(), "ADD_ERROR", err,
				"Run `agios help` for usage information",
			)
		}
		os.Exit(1)
	}

	writePipelinedJSON(map[string]any{
		"message": fmt.Sprintf("Added %q to agios.yaml", appName),
		"help": []string{
			fmt.Sprintf("Run `agios %s <command>` to interact with the app", appName),
			"Run `agios status` to check the health of all configured apps",
		},
	})
}

type addError struct {
	msg   string
	code  string
	cause error
}

func (e *addError) Error() string { return e.msg }

// addApp validates and adds an app to the config. Returns nil on success.
func addApp(cfg *config.Config, appName string) error {
	// Check if app is already listed
	if cfg.HasApp(appName) {
		return &addError{
			msg:  fmt.Sprintf("App %q is already configured.", appName),
			code: "ALREADY_ADDED",
		}
	}

	// Validate binary exists on PATH
	binPath, err := runner.Resolve(appName)
	if err != nil {
		return &addError{
			msg:   fmt.Sprintf("Binary %q not found on PATH.", appName),
			code:  "BINARY_NOT_FOUND",
			cause: err,
		}
	}

	// Validate AIP compliance by running "<app> status"
	if err := validateAIP(binPath, appName); err != nil {
		return err
	}

	// Add app to config and save
	cfg.Apps = append(cfg.Apps, appName)
	if err := cfg.Save(); err != nil {
		return &addError{
			msg:   "Failed to save config",
			code:  "ADD_ERROR",
			cause: err,
		}
	}

	return nil
}

// validateAIP runs "<app> status" and checks the output is valid JSON/JSONL.
func validateAIP(binPath, appName string) error {
	result, execErr := runner.Exec(binPath, []string{"status"}, runner.DefaultTimeout)
	if execErr != nil && (result == nil || len(result.Stdout) == 0) {
		return &addError{
			msg:   fmt.Sprintf("App %q failed AIP validation: `%s status` returned an error.", appName, appName),
			code:  "AIP_VALIDATION_FAILED",
			cause: execErr,
		}
	}

	// Verify the output is valid JSON
	if result != nil && len(result.Stdout) > 0 {
		var obj map[string]any
		if err := json.Unmarshal(result.Stdout, &obj); err != nil {
			// Try parsing as JSONL (last line)
			if _, parseErr := runner.ParseJSONL(result.Stdout); parseErr != nil {
				return &addError{
					msg:   fmt.Sprintf("App %q failed AIP validation: `%s status` returned invalid JSON.", appName, appName),
					code:  "AIP_VALIDATION_FAILED",
					cause: parseErr,
				}
			}
		}
	}

	return nil
}
