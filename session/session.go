// Package session provides call lifecycle tracking for xk6-sip-media.
package session

import (
	"sync"
	"time"

	corertp "xk6-sip-media/core/rtp"
	sipcall "xk6-sip-media/sip"
)

// State represents the lifecycle state of a call.
type State int

const (
	StateIdle State = iota
	StateActive
	StateDone
	StateError
)

// CallSession tracks a single SIP call's lifecycle and result.
type CallSession struct {
	mu      sync.RWMutex
	Config  sipcall.CallConfig
	State   State
	Started time.Time
	Ended   time.Time
	Result  corertp.CallResult
	Err     error
}

// NewCallSession creates a new idle CallSession with the given config.
func NewCallSession(cfg sipcall.CallConfig) *CallSession {
	return &CallSession{Config: cfg, State: StateIdle}
}

// Run executes the call and updates the session state accordingly.
func (s *CallSession) Run() {
	s.mu.Lock()
	s.State = StateActive
	s.Started = time.Now()
	s.mu.Unlock()

	result, err := sipcall.StartCall(s.Config)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.Ended = time.Now()
	s.Result = result
	s.Err = err
	if err != nil {
		s.State = StateError
	} else {
		s.State = StateDone
	}
}

// Duration returns the wall-clock duration of the call.
func (s *CallSession) Duration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Ended.IsZero() {
		return time.Since(s.Started)
	}
	return s.Ended.Sub(s.Started)
}
