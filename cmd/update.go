package cmd

import (
	"fmt"
	"os"

	"github.com/agios-sh/agios/updater"
)

// RunUpdate implements the "agios update" command.
// With no subcommand: check + apply (download and install if newer version exists).
// With "check" subcommand: check only, update cache, report versions.
func RunUpdate(args []string, version string) {
	if version == "dev" {
		writePipelinedJSON(map[string]any{
			"message": "Development build — update check skipped",
			"version": version,
			"help": []string{
				"Build with a version to enable updates: make build VERSION=v1.0.0",
			},
		})
		return
	}

	// Subcommand: agios update check
	if len(args) > 0 && args[0] == "check" {
		runUpdateCheck(version)
		return
	}

	// Default: check + apply
	runUpdateApply(version)
}

func runUpdateCheck(version string) {
	result, err := updater.CheckLatest(version)
	if err != nil {
		writeError("Failed to check for updates", "UPDATE_CHECK_FAILED", err,
			"Check your internet connection and try again",
			"GitHub API: https://api.github.com/repos/agios-sh/agios/releases/latest",
		)
		os.Exit(1)
	}

	if !result.UpdateAvailable {
		writePipelinedJSON(map[string]any{
			"message":         "agios is up to date",
			"current_version": result.CurrentVersion,
			"latest_version":  result.LatestVersion,
			"help": []string{
				"Run `agios help` for usage information",
			},
		})
		return
	}

	writePipelinedJSON(map[string]any{
		"message":         fmt.Sprintf("Update available: %s → %s", result.CurrentVersion, result.LatestVersion),
		"current_version": result.CurrentVersion,
		"latest_version":  result.LatestVersion,
		"help": []string{
			"Run `agios update` to download and install the update",
		},
	})
}

func runUpdateApply(version string) {
	result, err := updater.CheckLatest(version)
	if err != nil {
		writeError("Failed to check for updates", "UPDATE_CHECK_FAILED", err,
			"Check your internet connection and try again",
		)
		os.Exit(1)
	}

	if !result.UpdateAvailable {
		writePipelinedJSON(map[string]any{
			"message":         "agios is up to date",
			"current_version": result.CurrentVersion,
			"latest_version":  result.LatestVersion,
			"help": []string{
				"Run `agios help` for usage information",
			},
		})
		return
	}

	if err := updater.Apply(result); err != nil {
		writeError(
			fmt.Sprintf("Failed to install update: %v", err),
			"UPDATE_FAILED",
			err,
			"Try downloading manually from https://github.com/agios-sh/agios/releases/latest",
			"You may need to run with elevated permissions",
		)
		os.Exit(1)
	}

	writePipelinedJSON(map[string]any{
		"message":          fmt.Sprintf("Updated agios %s → %s", result.CurrentVersion, result.LatestVersion),
		"previous_version": result.CurrentVersion,
		"current_version":  result.LatestVersion,
		"help": []string{
			"Run `agios --version` to verify the update",
		},
	})
}
