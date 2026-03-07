//go:build windows

package terminal

import "fmt"

// ptyHandle is a stub on Windows.
type ptyHandle struct{}

func (h *ptyHandle) Write(data []byte) error     { return fmt.Errorf("not supported on Windows") }
func (h *ptyHandle) Resize(rows, cols int) error { return fmt.Errorf("not supported on Windows") }
func (h *ptyHandle) Kill()                       {}

func newPTYSession(id int, name, shell, dir string) (*PTYSession, error) {
	return nil, fmt.Errorf("terminal sessions are not supported on Windows")
}
