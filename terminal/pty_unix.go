//go:build !windows

package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
)

// ptyHandle wraps the platform-specific PTY resources.
type ptyHandle struct {
	cmd  *exec.Cmd
	ptmx *os.File
}

// Write sends bytes to the PTY.
func (h *ptyHandle) Write(data []byte) error {
	_, err := h.ptmx.Write(data)
	return err
}

// Resize changes the PTY window size.
func (h *ptyHandle) Resize(rows, cols int) error {
	return pty.Setsize(h.ptmx, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

// Kill terminates the PTY process.
func (h *ptyHandle) Kill() {
	if h.cmd.Process != nil {
		h.cmd.Process.Kill()
	}
	h.ptmx.Close()
}

// newPTYSession creates a new PTY session on Unix.
func newPTYSession(id int, name, shell, dir string) (*PTYSession, error) {
	if shell == "" {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
	}
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			dir = os.Getenv("HOME")
		}
	}
	if name == "" {
		name = fmt.Sprintf("session-%d", id)
	}

	cmd := exec.Command(shell)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("starting PTY: %w", err)
	}

	rows, cols := 80, 120
	pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})

	sess := &PTYSession{
		ID:        id,
		Name:      name,
		Shell:     shell,
		Dir:       dir,
		StartedAt: time.Now(),
		pty:       ptyHandle{cmd: cmd, ptmx: ptmx},
		screen:    NewScreenBuffer(rows, cols),
	}

	// Read PTY output into screen buffer
	go func() {
		tmp := make([]byte, 4096)
		for {
			n, err := ptmx.Read(tmp)
			if n > 0 {
				sess.screen.Write(tmp[:n])
			}
			if err != nil {
				sess.mu.Lock()
				sess.Exited = true
				if cmd.ProcessState != nil {
					sess.ExitErr = cmd.ProcessState.String()
				} else {
					sess.ExitErr = err.Error()
				}
				sess.mu.Unlock()
				return
			}
		}
	}()

	// Wait for process exit
	go func() {
		cmd.Wait()
		ptmx.Close()
	}()

	return sess, nil
}
