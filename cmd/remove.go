package cmd

import (
	"fmt"
	"os"

	"github.com/agios-sh/agios/config"
)

// RunRemove removes an app from agios.yaml.
func RunRemove(args []string) {
	if len(args) == 0 {
		writeError("Usage: agios remove <name>", "INVALID_ARGS",
			"Run `agios help` for usage information",
		)
		os.Exit(1)
	}
	appName := args[0]

	cfg := loadConfig()

	if err := removeApp(cfg, appName); err != nil {
		if ce, ok := err.(*cmdError); ok {
			writeError(ce.msg, ce.code,
				"Run `agios status` to see configured apps",
			)
		} else {
			writeError(err.Error(), "REMOVE_ERROR",
				"Run `agios help` for usage information",
			)
		}
		os.Exit(1)
	}

	writePipelinedJSON(map[string]any{
		"message": fmt.Sprintf("Removed %q from agios.yaml", appName),
		"help": []string{
			"Run `agios add <name>` to register an app",
			"Run `agios status` to check the health of all configured apps",
		},
	})
}

func removeApp(cfg *config.Config, appName string) error {
	if !cfg.HasApp(appName) {
		return &cmdError{
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
		return &cmdError{
			msg:   "Failed to save config",
			code:  "REMOVE_ERROR",
			cause: err,
		}
	}

	return nil
}
