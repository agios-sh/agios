package cmd

import (
	"encoding/json"
	"os"

	"github.com/agios-sh/agios/config"
	"github.com/agios-sh/agios/output"
)

// loadConfig loads the nearest agios.yaml or exits with an AIP error.
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

func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// writePipelinedJSON writes v through the output pipeline (truncation + TOON).
func writePipelinedJSON(v any) {
	data, err := output.Process(v)
	if err != nil {
		writeJSON(v)
		return
	}
	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
}

// latestProgress unwraps the inner "progress" value from the last progress line.
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

func writeError(msg, code string, help ...string) {
	output.EmitError(msg, code, "Run `agios help` for usage information", help...)
}
