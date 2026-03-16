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

var agiosEnvVars = []string{"AGIOS_FRESH", "AGIOS_VERBOSE", "AGIOS_QUIET"}

func buildEnv() []string {
	env := os.Environ()
	for _, key := range agiosEnvVars {
		if val, ok := os.LookupEnv(key); ok {
			env = append(env, fmt.Sprintf("%s=%s", key, val))
		}
	}
	return env
}

type ExecResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Exec runs binPath with args, capturing stdout/stderr. Killed after timeout.
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

	cmd.Env = buildEnv()

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
