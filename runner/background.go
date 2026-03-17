package runner

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// ExecBackground runs a detached subprocess that survives the parent process.
func ExecBackground(binPath string, args []string, outputPath string) (*os.Process, error) {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("creating output file: %w", err)
	}

	cmd := exec.Command(binPath, args...)
	cmd.Stdout = outFile
	cmd.Stderr = outFile
	cmd.Stdin = nil
	cmd.Env = buildEnv()
	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		outFile.Close()
		return nil, fmt.Errorf("starting background process: %w", err)
	}

	go func() {
		cmd.Wait()
		outFile.Close()
	}()

	return cmd.Process, nil
}

// GracefulKill sends SIGTERM to a process and waits up to timeout for it to
// exit. If the process does not exit in time, it is forcefully killed.
func GracefulKill(proc *os.Process, timeout time.Duration) {
	proc.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		proc.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		proc.Kill()
	}
}
