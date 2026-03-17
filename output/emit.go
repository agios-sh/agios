package output

import (
	"encoding/json"
	"os"
)

// EmitResult writes a JSON value through the output pipeline to stdout.
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

// EmitError writes an AIP error to stdout. Uses defaultHelp if no help strings given.
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
