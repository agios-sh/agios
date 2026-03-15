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

// latestProgress extracts the most recent progress value from parsed JSONL
// progress lines. ParseJSONL stores the full line object {"progress": {...}},
// so this unwraps the inner value when present. Returns nil if there are no
// progress lines.
func latestProgress(progress []map[string]any) any {
	if len(progress) == 0 {
		return nil
	}
	last := progress[len(progress)-1]
	if inner, ok := last["progress"]; ok {
		return inner
	}
	return last
}

// writeError writes an error response to stdout per the AIP output standard
// (section 4.3). Every error includes a help array so agents always know what
// to do next. Delegates to output.EmitError for consistent error formatting
// across built-in and external app output paths.
func writeError(msg, code string, help ...string) {
	output.EmitError(msg, code, "Run `agios help` for usage information", help...)
}
