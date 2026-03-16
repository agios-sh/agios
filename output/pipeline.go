package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// Process runs the output pipeline: normalize → truncate → TOON.
// Set AGIOS_FORMAT=json to skip TOON conversion.
func Process(v any) ([]byte, error) {
	_ = CleanupTempFiles()

	normalized, err := normalize(v)
	if err != nil {
		return nil, fmt.Errorf("normalization: %w", err)
	}

	truncated, err := Truncate(normalized)
	if err != nil {
		return nil, fmt.Errorf("truncation: %w", err)
	}

	if os.Getenv("AGIOS_FORMAT") == "json" {
		return json.Marshal(truncated)
	}

	out, err := ToTOON(truncated)
	if err != nil {
		return nil, fmt.Errorf("TOON conversion: %w", err)
	}

	return out, nil
}

// normalize converts v to JSON-compatible types via a round-trip (respects struct tags).
func normalize(v any) (any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
