package runner

import (
	"fmt"
	"os"
	"os/exec"
)

// ExecBackground runs a detached subprocess that survives the parent, for background job execution.
func ExecBackground(binPath string, args []string, outputPath string) (*os.Process, error) {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("creating output file: %w", err)
	}

	cmd := exec.Command(binPath, args...)
	cmd.Stdout = outFile
	cmd.Stderr = outFile // capture stderr too for debugging
	cmd.Stdin = nil      // no stdin for background jobs

	// Build env: inherit parent env and ensure AGIOS_* vars are passed through
	cmd.Env = os.Environ()
	for _, key := range passEnvVars {
		if val, ok := os.LookupEnv(key); ok {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
		}
	}

	// Set platform-specific process attributes for detaching
	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		outFile.Close()
		return nil, fmt.Errorf("starting background process: %w", err)
	}

	// Start a goroutine to wait for the process and close the file.
	// This prevents zombie processes and ensures the output file is closed.
	go func() {
		cmd.Wait()
		outFile.Close()
	}()

	return cmd.Process, nil
}
