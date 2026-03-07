package runner

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
)

// ParsedOutput holds the result of parsing JSONL output from an app subprocess.
type ParsedOutput struct {
	// Progress contains all progress update lines (those with a "progress" key).
	Progress []map[string]any
	// Result is the final non-progress line (the last line without a "progress" key).
	Result map[string]any
}

// ParseJSONL parses JSONL output from an app subprocess.
// Lines containing a "progress" key are collected as progress updates.
// The last non-progress line is treated as the final result.
// Returns an error if the output is empty or contains invalid JSON.
func ParseJSONL(data []byte) (*ParsedOutput, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	output := &ParsedOutput{}

	var lineCount int
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		lineCount++

		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			return nil, &InvalidOutputError{
				Message: fmt.Sprintf("line %d: invalid JSON", lineCount),
				Raw:     string(data),
			}
		}

		if _, hasProgress := obj["progress"]; hasProgress {
			output.Progress = append(output.Progress, obj)
		} else {
			// Each non-progress line overwrites the previous; the last one wins.
			output.Result = obj
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading output: %w", err)
	}

	if lineCount == 0 {
		return nil, &InvalidOutputError{
			Message: "empty output",
			Raw:     string(data),
		}
	}

	if output.Result == nil {
		return nil, &InvalidOutputError{
			Message: "no result line found (all lines were progress updates)",
			Raw:     string(data),
		}
	}

	return output, nil
}

// InvalidOutputError indicates the app produced output that doesn't conform to protocol.
type InvalidOutputError struct {
	Message string
	Raw     string
}

func (e *InvalidOutputError) Error() string {
	return fmt.Sprintf("invalid output: %s", e.Message)
}
