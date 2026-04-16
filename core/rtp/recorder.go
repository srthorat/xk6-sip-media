package rtp

import (
	"os"
	"sync"
)

// AudioRecorder writes raw RTP payloads to a file for post-call analysis
// (PESQ scoring, silence detection on received audio, etc.).
// It is safe for concurrent use.
type AudioRecorder struct {
	file *os.File
	mu   sync.Mutex
	buf  []byte // accumulated raw payload bytes
}

// NewRecorder creates an AudioRecorder that writes to the given path.
// Pass an empty path to run in buffer-only mode (no disk write).
func NewRecorder(path string) (*AudioRecorder, error) {
	r := &AudioRecorder{}
	if path != "" {
		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		r.file = f
	}
	return r, nil
}

// Write appends a received RTP payload slice to the recorder buffer (and file).
func (r *AudioRecorder) Write(payload []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, payload...)
	if r.file != nil {
		_, _ = r.file.Write(payload)
	}
}

// Bytes returns all accumulated payload bytes (PCMU encoded audio).
func (r *AudioRecorder) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]byte, len(r.buf))
	copy(out, r.buf)
	return out
}

// Path returns the file path, or empty string if no file is used.
func (r *AudioRecorder) Path() string {
	if r.file == nil {
		return ""
	}
	return r.file.Name()
}

// Close flushes and closes the underlying file (no-op if no file).
func (r *AudioRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}
