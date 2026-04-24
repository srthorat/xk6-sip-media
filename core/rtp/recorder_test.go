package rtp

import (
	"os"
	"sync"
	"testing"
)

// newSmallRecorder creates an AudioRecorder backed by a temp file with a
// writeCh of the specified capacity. Used to trigger channel-full drops in
// tests without needing to flood a 4096-element channel.
func newSmallRecorder(t *testing.T, chCap int) *AudioRecorder {
	t.Helper()
	f, err := os.CreateTemp("", "recorder_test_*.raw")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	r := &AudioRecorder{
		file:       f,
		writeCh:    make(chan []byte, chCap),
		writerDone: make(chan struct{}),
	}
	go r.fileWriter()
	return r
}

// ── AudioRecorder.DroppedFrames (fix 6.6) ─────────────────────────────────────

// TestRecorder_DroppedFrames_ChannelFull verifies that Write() increments
// DroppedFrames when writeCh is full (unbuffered channel → every write drops).
func TestRecorder_DroppedFrames_ChannelFull(t *testing.T) {
	// Unbuffered channel: the fileWriter goroutine is blocked on receive while we
	// send, so every select takes the default branch and increments DroppedFrames.
	r := newSmallRecorder(t, 0)

	payload := make([]byte, 160)
	const writes = 10
	for i := 0; i < writes; i++ {
		r.Write(payload)
	}
	r.Close()

	drops := r.DroppedFrames.Load()
	if drops == 0 {
		t.Error("expected DroppedFrames > 0 with unbuffered writeCh, got 0")
	}
	if drops > int64(writes) {
		t.Errorf("DroppedFrames %d > writes %d (counter over-incremented)", drops, writes)
	}
}

// TestRecorder_DroppedFrames_NilCh verifies that buffer-only mode (no file,
// writeCh == nil) never increments DroppedFrames.
func TestRecorder_DroppedFrames_NilCh(t *testing.T) {
	r, err := NewRecorder("") // buffer-only: no file, no writeCh
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	defer r.Close()

	for i := 0; i < 100; i++ {
		r.Write(make([]byte, 160))
	}

	if d := r.DroppedFrames.Load(); d != 0 {
		t.Errorf("buffer-only recorder: expected 0 drops, got %d", d)
	}
}

// TestRecorder_DroppedFrames_ConcurrentWrite verifies concurrent Write() calls
// are race-free under the -race flag. With an unbuffered channel all frames
// drop, but the DroppedFrames counter must be updated without a data race.
func TestRecorder_DroppedFrames_ConcurrentWrite(t *testing.T) {
	r := newSmallRecorder(t, 0)
	defer r.Close()

	var wg sync.WaitGroup
	payload := make([]byte, 160)
	const goroutines = 8
	const writesEach = 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < writesEach; j++ {
				r.Write(payload)
			}
		}()
	}
	wg.Wait()

	// Some frames may have been received by the fileWriter goroutine (unbuffered
	// channel: a send succeeds if the goroutine happens to be in its receive).
	// What matters is that drops + successes == total and the counter didn't race.
	got := r.DroppedFrames.Load()
	total := int64(goroutines * writesEach)
	if got > total {
		t.Errorf("DroppedFrames %d > total writes %d (counter over-incremented)", got, total)
	}
}

// TestRecorder_DroppedFrames_BufferedNoDrops verifies that with a sufficiently
// large channel no frames are dropped when writes are slower than the writer.
func TestRecorder_DroppedFrames_BufferedNoDrops(t *testing.T) {
	// Channel large enough for all writes with margin.
	const writes = 10
	r := newSmallRecorder(t, writes+1)

	payload := make([]byte, 160)
	for i := 0; i < writes; i++ {
		r.Write(payload)
	}
	r.Close() // waits for fileWriter to drain

	if d := r.DroppedFrames.Load(); d != 0 {
		t.Errorf("large-buffered recorder: expected 0 drops, got %d", d)
	}
}
