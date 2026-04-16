package rtp

import (
	"net"
	"sync"
)

// Session holds the shared state for a single RTP send/receive flow.
// It is safe for concurrent use (sender + DTMF goroutines).
type Session struct {
	Conn       *net.UDPConn
	RemoteAddr *net.UDPAddr

	SSRC uint32

	mu  sync.Mutex
	seq uint16
	ts  uint32
}

// NewSession creates a new RTPSession with a random-ish SSRC.
func NewSession(conn *net.UDPConn, remote *net.UDPAddr, ssrc uint32) *Session {
	return &Session{
		Conn:       conn,
		RemoteAddr: remote,
		SSRC:       ssrc,
	}
}

// NextSeqTS atomically increments and returns the next sequence number and
// timestamp for outbound packets, keeping the send stream consistent even
// when the audio sender and DTMF sender interleave.
func (s *Session) NextSeqTS(tsIncrement uint32) (seq uint16, ts uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seq = s.seq
	ts = s.ts
	s.seq++
	s.ts += tsIncrement
	return
}

// Send marshals and sends a raw byte payload using the session connection.
func (s *Session) Send(raw []byte) error {
	_, err := s.Conn.WriteTo(raw, s.RemoteAddr)
	return err
}
