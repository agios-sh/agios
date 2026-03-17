package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/agios-sh/agios/config"
	"github.com/agios-sh/agios/runner"
)

// RunAdd validates an app binary and adds it to agios.yaml.
func RunAdd(args []string) {
	if len(args) == 0 {
		emitError("Usage: agios add <name>", "INVALID_ARGS",
			"Run `agios help` for usage information",
		)
		os.Exit(1)
	}
	appName := args[0]

	cfg := loadConfig()

	if err := addApp(cfg, appName); err != nil {
		if ce, ok := err.(*cmdError); ok {
			emitError(ce.msg, ce.code,
				"Ensure the app binary is installed and on your PATH",
			)
		} else {
			emitError(err.Error(), "ADD_ERROR",
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

func addApp(cfg *config.Config, appName string) error {
	if cfg.HasApp(appName) {
		return &cmdError{
			msg:  fmt.Sprintf("App %q is already configured.", appName),
			code: "ALREADY_ADDED",
		}
	}

	binPath, err := runner.Resolve(appName)
	if err != nil {
		return &cmdError{
			msg:   fmt.Sprintf("Binary %q not found on PATH.", appName),
			code:  "BINARY_NOT_FOUND",
			cause: err,
		}
	}

	if err := validateAIP(binPath, appName); err != nil {
		return err
	}

	cfg.Apps = append(cfg.Apps, appName)
	if err := cfg.Save(); err != nil {
		return &cmdError{
			msg:   "Failed to save config",
			code:  "ADD_ERROR",
			cause: err,
		}
	}

	return nil
}

func validateAIP(binPath, appName string) error {
	result, execErr := runner.Exec(binPath, []string{"status"}, runner.DefaultTimeout)
	if execErr != nil && (result == nil || len(result.Stdout) == 0) {
		return &cmdError{
			msg:   fmt.Sprintf("App %q failed AIP validation: `%s status` returned an error.", appName, appName),
			code:  "AIP_VALIDATION_FAILED",
			cause: execErr,
		}
	}

	if result != nil && len(result.Stdout) > 0 {
		var obj map[string]any
		if err := json.Unmarshal(result.Stdout, &obj); err != nil {
			if _, parseErr := runner.ParseJSONL(result.Stdout); parseErr != nil {
				return &cmdError{
					msg:   fmt.Sprintf("App %q failed AIP validation: `%s status` returned invalid JSON.", appName, appName),
					code:  "AIP_VALIDATION_FAILED",
					cause: parseErr,
				}
			}
		}
	}

	return nil
}
