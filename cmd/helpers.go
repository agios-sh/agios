package cmd

import (
	"encoding/json"
	"os"

	"github.com/agios-sh/agios/config"
	"github.com/agios-sh/agios/output"
)

// loadConfig loads agios.yaml from the current directory, exiting with an
// AIP error if the working directory can't be determined or no config is found.
func loadConfig() *config.Config {
	cwd, err := os.Getwd()
	if err != nil {
		writeError("Failed to get working directory", "INTERNAL_ERROR",
			"Run `agios help` for usage information",
		)
		os.Exit(1)
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		writeError(
			"No agios.yaml found. Run `agios init` first.",
			"NO_CONFIG",
			"Run `agios init` to create a new agios.yaml",
			"Run `agios help` for usage information",
		)
		os.Exit(1)
	}

	return cfg
}

// writeJSON writes a JSON object to stdout.
func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// writePipelinedJSON runs the output pipeline (truncation, TOON conversion)
// on a JSON value and writes the result to stdout. Used for app output that
// may contain large values. Falls back to writeJSON if the pipeline fails.
func writePipelinedJSON(v any) {
	data, err := output.Process(v)
	if err != nil {
		// Pipeline failed — fall back to plain JSON
		writeJSON(v)
		return
	}
	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
}

// writeError writes an error response to stdout per the AIP output standard
// (section 4.3). Every error includes a help array so agents always know what
// to do next.
func writeError(msg, code string, help ...string) {
	result := map[string]any{
		"error": msg,
		"code":  code,
	}
	if len(help) > 0 {
		result["help"] = help
	} else {
		result["help"] = []string{"Run `agios help` for usage information"}
	}
	writePipelinedJSON(result)
}
