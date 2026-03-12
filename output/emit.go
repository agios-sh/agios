package output

import (
	"encoding/json"
	"os"
)

// EmitResult writes a JSON value through the output pipeline to stdout.
// Falls back to plain JSON encoding if the pipeline fails.
func EmitResult(v map[string]any) {
	data, err := Process(v)
	if err != nil {
		enc := json.NewEncoder(os.Stdout)
		enc.Encode(v)
		return
	}
	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
}

// EmitError writes an AIP-compliant error response to stdout.
// If no help strings are provided, defaultHelp is used as the fallback.
func EmitError(msg, code, defaultHelp string, help ...string) {
	result := map[string]any{
		"error": msg,
		"code":  code,
	}
	if len(help) > 0 {
		result["help"] = help
	} else {
		result["help"] = []string{defaultHelp}
	}
	EmitResult(result)
}
