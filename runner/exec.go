package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// DefaultTimeout is the default subprocess timeout.
const DefaultTimeout = 5 * time.Second

// passEnvVars are environment variables forwarded from the parent process to app subprocesses.
var passEnvVars = []string{"AGIOS_FRESH", "AGIOS_VERBOSE", "AGIOS_QUIET"}

// ExecResult holds the captured output of a subprocess.
type ExecResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Exec runs the given binary with args, capturing stdout and stderr separately.
// It forwards stdin from the parent process and passes through AGIOS_* env vars.
// The subprocess is killed if it exceeds the given timeout.
func Exec(binPath string, args []string, timeout time.Duration) (*ExecResult, error) {
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = os.Stdin

	// Build env: inherit parent env and ensure AGIOS_* vars are passed through.
	cmd.Env = os.Environ()
	for _, key := range passEnvVars {
		if val, ok := os.LookupEnv(key); ok {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
		}
	}

	err := cmd.Run()

	result := &ExecResult{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if ctx.Err() == context.DeadlineExceeded {
		return result, fmt.Errorf("command timed out after %s", timeout)
	}

	if err != nil {
		return result, err
	}

	return result, nil
}
