// Package runner handles app binary resolution, subprocess execution, and JSONL parsing.
package runner

import (
	"fmt"
	"os/exec"
)

// Resolve checks that the named binary exists on $PATH using exec.LookPath.
// Returns the full path to the binary or a structured error.
func Resolve(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("binary %q not found on PATH: %w", name, err)
	}
	return path, nil
}
