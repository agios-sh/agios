package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// Process runs the output pipeline on a JSON value:
// 1. Normalize to map[string]any / []any via JSON round-trip (handles struct tags)
// 2. Large value truncation (strings > 4096 chars spilled to temp files)
// 3. TOON conversion (compact token-oriented encoding)
//
// Set AGIOS_FORMAT=json to skip TOON conversion and output compact JSON instead.
//
// It also performs best-effort cleanup of expired temp files.
func Process(v any) ([]byte, error) {
	// Best-effort cleanup of old temp files
	_ = CleanupTempFiles()

	// Step 1: Normalize to JSON-compatible types (map[string]any, []any, etc.).
	// This ensures struct types with json tags are properly handled.
	normalized, err := normalize(v)
	if err != nil {
		return nil, fmt.Errorf("normalization: %w", err)
	}

	// Step 2: Truncate large string values
	truncated, err := Truncate(normalized)
	if err != nil {
		return nil, fmt.Errorf("truncation: %w", err)
	}

	// Step 3: Convert to output format
	if os.Getenv("AGIOS_FORMAT") == "json" {
		return json.Marshal(truncated)
	}

	out, err := ToTOON(truncated)
	if err != nil {
		return nil, fmt.Errorf("TOON conversion: %w", err)
	}

	return out, nil
}

// normalize converts any value to JSON-compatible types (map[string]any, []any,
// string, float64, bool, nil) via a JSON round-trip. This ensures struct tags
// like `json:"name,omitempty"` are respected.
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
