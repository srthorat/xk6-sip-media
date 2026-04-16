package session

import (
	"sync"

	sipcall "xk6-sip-media/sip"
)

// Manager tracks all active and completed call sessions.
// In k6, each VU creates its own CallSession and calls Run() directly —
// the Manager is optional and useful for standalone CLI tooling or dashboards.
type Manager struct {
	mu       sync.Mutex
	sessions []*CallSession
}

// NewManager returns an empty session Manager.
func NewManager() *Manager {
	return &Manager{}
}

// Run creates a new CallSession, registers it, executes the call, and returns
// the session when done.
func (m *Manager) Run(cfg sipcall.CallConfig) *CallSession {
	sess := NewCallSession(cfg)

	m.mu.Lock()
	m.sessions = append(m.sessions, sess)
	m.mu.Unlock()

	sess.Run()
	return sess
}

// Sessions returns a snapshot of all sessions (active + completed).
func (m *Manager) Sessions() []*CallSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*CallSession, len(m.sessions))
	copy(out, m.sessions)
	return out
}

// ActiveCount returns the number of currently active calls.
func (m *Manager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, s := range m.sessions {
		if s.State == StateActive {
			count++
		}
	}
	return count
}
