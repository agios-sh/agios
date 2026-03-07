//go:build windows

package browser

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr configures the Chrome process to run as a detached process
// on Windows systems.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
