package terminal

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/vito/midterm"
)

const (
	// DefaultRows and DefaultCols define the default PTY dimensions.
	// Sized for agent consumers — larger than a standard 24×80 terminal
	// so AI tools can read more output without truncation.
	DefaultRows = 80
	DefaultCols = 120
)

// ScreenBuffer wraps a virtual terminal emulator that maintains true screen state.
// PTY output is fed in as raw bytes; reads return the current screen content as
// plain text with all ANSI escape sequences already interpreted.
type ScreenBuffer struct {
	mu     sync.Mutex
	vt     *midterm.Terminal
	writes uint64 // monotonic counter incremented on each Write call
	notify chan struct{}
}

func NewScreenBuffer(rows, cols int) *ScreenBuffer {
	return &ScreenBuffer{
		vt:     midterm.NewTerminal(rows, cols),
		notify: make(chan struct{}, 1),
	}
}

func (sb *ScreenBuffer) Write(p []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	n, err := sb.vt.Write(p)
	sb.writes++

	// Signal waiters
	select {
	case sb.notify <- struct{}{}:
	default:
	}

	return n, err
}

func (sb *ScreenBuffer) Screen() screenState {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	return screenToState(sb.vt)
}

func (sb *ScreenBuffer) Writes() uint64 {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.writes
}

func (sb *ScreenBuffer) WaitForData(timeout time.Duration) bool {
	select {
	case <-sb.notify:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (sb *ScreenBuffer) Resize(rows, cols int) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.vt.Resize(rows, cols)
}

// screenState holds the screen text and cursor position.
type screenState struct {
	Text      string
	CursorRow int
	CursorCol int
}

// screenToState extracts the screen content and cursor position from a
// midterm.Terminal. It trims trailing whitespace per line and trailing blank lines.
func screenToState(vt *midterm.Terminal) screenState {
	height := vt.Height
	if height == 0 {
		return screenState{}
	}

	lines := make([]string, height)
	for y := 0; y < height; y++ {
		if y < len(vt.Content) {
			lines[y] = strings.TrimRight(string(vt.Content[y]), " ")
		}
	}

	// Trim trailing blank lines
	last := len(lines) - 1
	for last >= 0 && lines[last] == "" {
		last--
	}
	lines = lines[:last+1]

	return screenState{
		Text:      strings.Join(lines, "\n"),
		CursorRow: vt.Cursor.Y,
		CursorCol: vt.Cursor.X,
	}
}

// PTYSession wraps a PTY file descriptor, shell process, and screen buffer.
type PTYSession struct {
	ID        int
	Name      string
	Shell     string
	Dir       string
	StartedAt time.Time
	Exited    bool
	ExitErr   string

	pty    ptyHandle
	screen *ScreenBuffer
	mu     sync.Mutex
}

// SessionInfo is the serializable session info returned to clients.
type SessionInfo struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Shell     string `json:"shell"`
	Dir       string `json:"dir"`
	StartedAt string `json:"started_at"`
	Exited    bool   `json:"exited"`
}

func (s *PTYSession) Info() SessionInfo {
	return SessionInfo{
		ID:        s.ID,
		Name:      s.Name,
		Shell:     s.Shell,
		Dir:       s.Dir,
		StartedAt: s.StartedAt.Format(time.RFC3339),
		Exited:    s.Exited,
	}
}

func (s *PTYSession) ReadScreen() screenState {
	return s.screen.Screen()
}

// SendAndWait writes input to the PTY, then waits for the screen to stabilize
// (no new writes for 100ms), capped at timeoutMs total. Returns the full
// current screen state.
func (s *PTYSession) SendAndWait(input []byte, timeoutMs int) (screenState, error) {
	s.mu.Lock()
	exited := s.Exited
	s.mu.Unlock()
	if exited {
		return screenState{}, fmt.Errorf("session has exited")
	}

	writesBefore := s.screen.Writes()

	if err := s.pty.Write(input); err != nil {
		return screenState{}, fmt.Errorf("writing to PTY: %w", err)
	}

	// Wait for output to stabilize. Strategy:
	// 1. Wait for new data or a quiet period (100ms with no writes).
	// 2. If we got output and it's been quiet, we're done.
	// 3. If no output yet, check if the process exited.
	// 4. Give up when the overall deadline expires.
	deadline := time.After(time.Duration(timeoutMs) * time.Millisecond)
	quietPeriod := 100 * time.Millisecond

	for {
		select {
		case <-deadline:
			goto done
		default:
		}

		if !s.screen.WaitForData(quietPeriod) {
			// No new writes for quietPeriod
			if s.screen.Writes() > writesBefore {
				// We got output and it's been quiet — done
				goto done
			}
			// No output yet — check if process exited
			s.mu.Lock()
			exited = s.Exited
			s.mu.Unlock()
			if exited {
				goto done
			}
		}
	}

done:
	return s.screen.Screen(), nil
}

// SessionManager manages multiple PTY sessions.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[int]*PTYSession
	activeID int
	nextID   int
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[int]*PTYSession),
		nextID:   1,
	}
}

func (sm *SessionManager) Start(name, shell, dir string) (*PTYSession, error) {
	sm.mu.Lock()
	id := sm.nextID
	sm.nextID++
	sm.mu.Unlock()

	sess, err := newPTYSession(id, name, shell, dir)
	if err != nil {
		return nil, err
	}

	sm.mu.Lock()
	sm.sessions[id] = sess
	sm.activeID = id
	sm.mu.Unlock()

	return sess, nil
}

func (sm *SessionManager) Switch(id int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.sessions[id]; !ok {
		return fmt.Errorf("session %d not found", id)
	}
	sm.activeID = id
	return nil
}

// Get returns a session by ID. If id is 0, returns the active session.
func (sm *SessionManager) Get(id int) (*PTYSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if id == 0 {
		id = sm.activeID
	}
	if id == 0 {
		return nil, fmt.Errorf("no active session")
	}

	sess, ok := sm.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %d not found", id)
	}
	return sess, nil
}

func (sm *SessionManager) List() ([]SessionInfo, int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var infos []SessionInfo
	for _, sess := range sm.sessions {
		infos = append(infos, sess.Info())
	}
	return infos, sm.activeID
}

// Kill terminates a session. If id is 0, kills the active session.
func (sm *SessionManager) Kill(id int) error {
	sm.mu.Lock()
	if id == 0 {
		id = sm.activeID
	}
	sess, ok := sm.sessions[id]
	if !ok {
		sm.mu.Unlock()
		return fmt.Errorf("session %d not found", id)
	}
	delete(sm.sessions, id)
	if sm.activeID == id {
		sm.activeID = 0
		for sid := range sm.sessions {
			sm.activeID = sid
			break
		}
	}
	sm.mu.Unlock()

	sess.pty.Kill()
	return nil
}

func (sm *SessionManager) KillAll() {
	sm.mu.Lock()
	sessions := make(map[int]*PTYSession)
	for k, v := range sm.sessions {
		sessions[k] = v
	}
	sm.sessions = make(map[int]*PTYSession)
	sm.activeID = 0
	sm.mu.Unlock()

	for _, sess := range sessions {
		sess.pty.Kill()
	}
}

func (sm *SessionManager) Count() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return len(sm.sessions)
}
