//go:build windows

package runner

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr configures the subprocess to run as a detached process
// on Windows systems.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
