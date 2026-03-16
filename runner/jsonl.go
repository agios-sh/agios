package runner

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
)

type ParsedOutput struct {
	Progress []map[string]any // lines with a "progress" key
	Result   map[string]any   // last non-progress line
}

// ParseJSONL parses JSONL output, separating progress lines from the final result.
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
			output.Result = obj // last non-progress line wins
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

type InvalidOutputError struct {
	Message string
	Raw     string
}

func (e *InvalidOutputError) Error() string {
	return fmt.Sprintf("invalid output: %s", e.Message)
}
