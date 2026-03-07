//go:build !windows

package browser

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr configures the Chrome process to run in a new session,
// detaching it from the parent process on Unix systems.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}
