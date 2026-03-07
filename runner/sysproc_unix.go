//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr configures the subprocess to run in a new session,
// detaching it from the parent process on Unix systems.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}
