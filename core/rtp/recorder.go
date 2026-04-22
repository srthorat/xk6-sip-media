package rtp

import (
	"os"
	"sync"
)

// AudioRecorder writes raw RTP payloads to a file for post-call analysis
// (PESQ scoring, silence detection on received audio, etc.).
// It is safe for concurrent use.
//
// File I/O is performed asynchronously via a background goroutine so that
// callers in the reactor hot-path (Tick) are never blocked by disk latency.
type AudioRecorder struct {
	file    *os.File
	mu      sync.Mutex
	buf     []byte    // accumulated raw payload bytes (in-memory, always synchronous)
	writeCh chan []byte // async channel to background file-writer goroutine
	writerDone chan struct{} // closed when background goroutine exits
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
		r.writeCh = make(chan []byte, 4096) // 4096 frames ≈ ~80 s of G.711 headroom
		r.writerDone = make(chan struct{})
		go r.fileWriter()
	}
	return r, nil
}

// fileWriter drains writeCh and writes payloads to disk sequentially.
// Runs as a single goroutine so file writes are always ordered.
func (r *AudioRecorder) fileWriter() {
	defer close(r.writerDone)
	for b := range r.writeCh {
		_, _ = r.file.Write(b)
	}
}

// Write appends a received RTP payload to the in-memory buffer (synchronous)
// and enqueues it for disk write (non-blocking — drops frame if channel full).
func (r *AudioRecorder) Write(payload []byte) {
	if len(payload) == 0 {
		return
	}
	r.mu.Lock()
	r.buf = append(r.buf, payload...)
	r.mu.Unlock()

	if r.writeCh != nil {
		// Copy payload so the caller can reuse its buffer immediately.
		cp := make([]byte, len(payload))
		copy(cp, payload)
		select {
		case r.writeCh <- cp:
		default:
			// Channel full: drop rather than block the reactor shard.
			// This is a best-effort write; PESQ analysis may show minor gaps.
		}
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

// Close flushes all pending writes and closes the underlying file.
func (r *AudioRecorder) Close() error {
	if r.writeCh != nil {
		close(r.writeCh)
		<-r.writerDone // wait for all enqueued writes to drain
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}
