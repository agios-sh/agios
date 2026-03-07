package cmd

import (
	"fmt"
	"os"

	"github.com/agios-sh/agios/config"
)

// RunRemove implements the "agios remove <app>" command.
// It removes an app from agios.yaml. Errors if the app is not listed.
func RunRemove(args []string) {
	if len(args) == 0 {
		writeError("Usage: agios remove <app>", "INVALID_ARGS", nil,
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

	if err := removeApp(cfg, appName); err != nil {
		if re, ok := err.(*removeError); ok {
			writeError(re.msg, re.code, re.cause,
				"Run `agios status` to see configured apps",
			)
		} else {
			writeError(err.Error(), "REMOVE_ERROR", err,
				"Run `agios help` for usage information",
			)
		}
		os.Exit(1)
	}

	writePipelinedJSON(map[string]any{
		"message": fmt.Sprintf("Removed %q from agios.yaml", appName),
		"help": []string{
			"Run `agios add <app>` to register an app",
			"Run `agios status` to check the health of all configured apps",
		},
	})
}

type removeError struct {
	msg   string
	code  string
	cause error
}

func (e *removeError) Error() string { return e.msg }

// removeApp removes an app from the config. Returns nil on success.
func removeApp(cfg *config.Config, appName string) error {
	if !cfg.HasApp(appName) {
		return &removeError{
			msg:  fmt.Sprintf("App %q is not configured.", appName),
			code: "NOT_CONFIGURED",
		}
	}

	var filtered []string
	for _, a := range cfg.Apps {
		if a != appName {
			filtered = append(filtered, a)
		}
	}
	cfg.Apps = filtered

	if err := cfg.Save(); err != nil {
		return &removeError{
			msg:   "Failed to save config",
			code:  "REMOVE_ERROR",
			cause: err,
		}
	}

	return nil
}
