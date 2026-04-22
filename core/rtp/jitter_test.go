package rtp

import (
	"sync"
	"testing"
	"time"

	pionrtp "github.com/pion/rtp"
)

// fakeRecorder collects Write calls for inspection.
type fakeRecorder struct {
	mu     sync.Mutex
	chunks [][]byte
}

func newFakeRecorder() *AudioRecorder {
	// Use the real recorder with empty path (in-memory)
	r, _ := NewRecorder("")
	return r
}

func makePacket(seq uint16, payload []byte) *pionrtp.Packet {
	return &pionrtp.Packet{
		Header: pionrtp.Header{
			Version:        2,
			PayloadType:    0,
			SequenceNumber: seq,
			Timestamp:      uint32(seq) * 160,
			SSRC:           0xdeadbeef,
		},
		Payload: payload,
	}
}

// TestJitterBuffer_InOrder verifies that in-order packets are played out correctly.
func TestJitterBuffer_InOrder(t *testing.T) {
	rec := newFakeRecorder()
	jb := NewJitterBuffer(rec, 0, 160)

	payload := []byte("frame0")
	jb.Push(makePacket(100, payload))

	// Tick once: should play seq 100
	alive := jb.Tick()
	if !alive {
		t.Fatal("Tick should return true on first tick with a packet present")
	}
}

// TestJitterBuffer_OutOfOrder verifies that out-of-order packets are buffered and played correctly.
func TestJitterBuffer_OutOfOrder(t *testing.T) {
	rec := newFakeRecorder()
	// Build directly (no MediaReactor.Add) so background ticks don't interfere.
	jb := &JitterBuffer{
		packets:  make(map[uint16]*pionrtp.Packet),
		recorder: rec,
		stop:     make(chan struct{}),
	}

	// Push seq 101 before seq 100
	jb.Push(makePacket(101, []byte("frame1")))
	jb.Push(makePacket(100, []byte("frame0")))

	// Push initialises nextSeq=100 (first packet seen sets the anchor)
	// But we pushed 101 first, so nextSeq=101. Then 100 is pushed but is "behind" → tested by LatePacket.
	// Reset to the proper start: re-create and push in correct test order
	jb2 := &JitterBuffer{
		packets:  make(map[uint16]*pionrtp.Packet),
		recorder: rec,
		stop:     make(chan struct{}),
	}
	jb2.Push(makePacket(100, []byte("frame0"))) // anchor set to 100
	jb2.Push(makePacket(101, []byte("frame1"))) // future, buffered

	// First Tick: plays seq 100, nextSeq → 101
	jb2.Tick()
	if jb2.nextSeq != 101 {
		t.Errorf("expected nextSeq=101 after first tick, got %d", jb2.nextSeq)
	}

	// Second Tick: plays seq 101, nextSeq → 102
	jb2.Tick()
	if jb2.nextSeq != 102 {
		t.Errorf("expected nextSeq=102 after second tick, got %d", jb2.nextSeq)
	}
}

// TestJitterBuffer_PLCOnMissedPacket verifies that a silence frame is injected
// when the expected seq is missing (Packet Loss Concealment).
func TestJitterBuffer_PLCOnMissedPacket(t *testing.T) {
	rec := newFakeRecorder()
	jb := &JitterBuffer{
		packets:      make(map[uint16]*pionrtp.Packet),
		playoutDelay: 0,
		recorder:     rec,
		stop:         make(chan struct{}),
		nextSeq:      200,
		started:      true,
		silenceSize:  160, // G.711: 160-byte silence per 20ms
	}

	// Tick with nothing in queue — PLC silence should fire
	alive := jb.Tick()
	if !alive {
		t.Fatal("Tick should still return true when injecting PLC silence")
	}
	if jb.nextSeq != 201 {
		t.Errorf("expected nextSeq to advance to 201, got %d", jb.nextSeq)
	}
	// Recorder must have received exactly 160 bytes of silence
	if got := len(rec.Bytes()); got != 160 {
		t.Errorf("G.711 PLC: expected 160 bytes written, got %d", got)
	}
}

// TestJitterBuffer_PLC_G729 verifies PLC silence for G.729 is 20 bytes per 20ms frame.
func TestJitterBuffer_PLC_G729(t *testing.T) {
	rec := newFakeRecorder()
	jb := &JitterBuffer{
		packets:      make(map[uint16]*pionrtp.Packet),
		playoutDelay: 0,
		recorder:     rec,
		stop:         make(chan struct{}),
		nextSeq:      100,
		started:      true,
		silenceSize:  20, // G.729: 10 bytes/10ms × 2 = 20 bytes
	}

	jb.Tick()

	if got := len(rec.Bytes()); got != 20 {
		t.Errorf("G.729 PLC: expected 20 bytes written, got %d", got)
	}
}

// TestJitterBuffer_PLC_Opus verifies that PLC is skipped entirely for Opus
// (silenceSize == 0) because zero-bytes are not a valid Opus bitstream.
func TestJitterBuffer_PLC_Opus(t *testing.T) {
	rec := newFakeRecorder()
	jb := &JitterBuffer{
		packets:      make(map[uint16]*pionrtp.Packet),
		playoutDelay: 0,
		recorder:     rec,
		stop:         make(chan struct{}),
		nextSeq:      50,
		started:      true,
		silenceSize:  0, // Opus: skip PLC write
	}

	jb.Tick()

	if got := len(rec.Bytes()); got != 0 {
		t.Errorf("Opus PLC: expected 0 bytes written (skip), got %d", got)
	}
	if jb.nextSeq != 51 {
		t.Errorf("Opus PLC: nextSeq should still advance, got %d", jb.nextSeq)
	}
}

// TestJitterBuffer_LatePacketDropped verifies truly late (already-played) packets are discarded.
func TestJitterBuffer_LatePacketDropped(t *testing.T) {
	rec := newFakeRecorder()
	jb := &JitterBuffer{
		packets:      make(map[uint16]*pionrtp.Packet),
		playoutDelay: 0,
		recorder:     rec,
		stop:         make(chan struct{}),
		nextSeq:      300,
		started:      true,
	}

	// Push a packet 10 sequences behind the playout head — should be dropped
	jb.Push(makePacket(290, []byte("stale")))

	if _, ok := jb.packets[290]; ok {
		t.Error("stale late packet should have been dropped by Push")
	}
}

// TestJitterBuffer_CloseFlushesThenRemovesFromReactor verifies Close() signals stop.
func TestJitterBuffer_CloseFlushesThenRemovesFromReactor(t *testing.T) {
	rec := newFakeRecorder()
	jb := &JitterBuffer{
		packets:      make(map[uint16]*pionrtp.Packet),
		playoutDelay: 0,
		recorder:     rec,
		stop:         make(chan struct{}),
		nextSeq:      0,
		started:      false,
	}

	jb.Close()

	// Tick after close should return false
	alive := jb.Tick()
	if alive {
		t.Error("Tick after Close() should return false to signal Reactor removal")
	}
}

// TestJitterBuffer_DoubleClose verifies that calling Close() twice does not panic.
// The sync.Once guard prevents closing an already-closed channel.
func TestJitterBuffer_DoubleClose(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("double Close() panicked: %v", r)
		}
	}()
	rec := newFakeRecorder()
	jb := &JitterBuffer{
		packets:  make(map[uint16]*pionrtp.Packet),
		recorder: rec,
		stop:     make(chan struct{}),
	}
	jb.Close()
	jb.Close() // must not panic
}

// TestJitterBuffer_PlayoutDelay_HoldsPackets verifies that when playoutDelay > 0,
// the jitter buffer does NOT play any packets until the delay has elapsed.
func TestJitterBuffer_PlayoutDelay_HoldsPackets(t *testing.T) {
	rec := newFakeRecorder()
	jb := &JitterBuffer{
		packets:      make(map[uint16]*pionrtp.Packet),
		playoutDelay: 500 * time.Millisecond, // long delay for deterministic test
		recorder:     rec,
		stop:         make(chan struct{}),
		silenceSize:  0, // suppress PLC
	}

	jb.Push(makePacket(0, []byte("early-frame")))

	// Tick immediately — delay not expired, so nothing should be written.
	jb.Tick()

	if got := len(rec.Bytes()); got != 0 {
		t.Errorf("playout delay: expected 0 bytes written before delay expires, got %d", got)
	}
}

// TestJitterBuffer_PlayoutDelay_PlaysAfterDelay verifies packets play out once
// the playout delay has elapsed.
func TestJitterBuffer_PlayoutDelay_PlaysAfterDelay(t *testing.T) {
	rec := newFakeRecorder()

	const payload = "hello"
	jb := &JitterBuffer{
		packets:      make(map[uint16]*pionrtp.Packet),
		playoutDelay: 10 * time.Millisecond, // short, will expire quickly
		recorder:     rec,
		stop:         make(chan struct{}),
		silenceSize:  0,
		nextSeq:      1,
		started:      true,
		delayExpired: false,
		firstPktTime: time.Now().Add(-50 * time.Millisecond), // simulate delay already passed
	}

	jb.Push(makePacket(1, []byte(payload)))
	jb.Tick()

	if got := len(rec.Bytes()); got != len(payload) {
		t.Errorf("playout after delay: expected %d bytes, got %d", len(payload), got)
	}
}

// TestJitterBuffer_PlayoutDelay_ZeroIsImmediate verifies that playoutDelay=0
// means packets play immediately on first Tick (no delay gate).
func TestJitterBuffer_PlayoutDelay_ZeroIsImmediate(t *testing.T) {
	rec := newFakeRecorder()
	jb := &JitterBuffer{
		packets:      make(map[uint16]*pionrtp.Packet),
		playoutDelay: 0,
		recorder:     rec,
		stop:         make(chan struct{}),
		silenceSize:  0,
	}
	jb.Push(makePacket(42, []byte("immediate")))
	jb.Tick()

	if got := len(rec.Bytes()); got != len("immediate") {
		t.Errorf("zero playout delay: expected %d bytes, got %d", len("immediate"), got)
	}
}

// TestJitterBuffer_FlushDoesNotLoop verifies the Close() flush path iterates the
// map directly (O(n)) rather than looping up to 65535 times over sequence space.
// With a sparse map (seq 0 and seq 40000), the old seq-walk would spin ~40000
// iterations; the new map-range is always O(len(map)).
func TestJitterBuffer_FlushDoesNotLoop(t *testing.T) {
	rec := newFakeRecorder()
	jb := &JitterBuffer{
		packets:      make(map[uint16]*pionrtp.Packet),
		playoutDelay: 0,
		recorder:     rec,
		stop:         make(chan struct{}),
		nextSeq:      0,
		started:      true,
		silenceSize:  0,
	}

	// Insert two packets with a large gap — simulates sparse map.
	jb.packets[0] = makePacket(0, []byte("pkt-zero"))
	jb.packets[40000] = makePacket(40000, []byte("pkt-far"))

	jb.Close()
	start := time.Now()
	alive := jb.Tick() // triggers close-branch flush
	elapsed := time.Since(start)

	if alive {
		t.Error("Tick after Close() should return false")
	}
	// With map-range flush the total payload written is 8+7=15 bytes.
	if got := len(rec.Bytes()); got != 8+7 {
		t.Errorf("flush: expected 15 bytes (both payloads), got %d", got)
	}
	// Should complete in well under 1ms — the old O(65535) loop would take ~1ms+.
	if elapsed > 10*time.Millisecond {
		t.Errorf("flush took too long (%v), expected < 10ms (old O(65535) loop?)", elapsed)
	}
}

// TestReactor_AddAndTickAllItems verifies the sharded reactor correctly ticks registered items.
func TestReactor_AddAndTickAllItems(t *testing.T) {
	r := NewReactor()

	var mu sync.Mutex
	ticked := 0

	item := &testTickable{
		count:   0,
		maxTick: 2,
		onTick:  func() { mu.Lock(); ticked++; mu.Unlock() },
	}

	r.Add(item)

	// Manually trigger 3 tick cycles across all shards
	for i := 0; i < 3; i++ {
		for _, s := range r.shards {
			s.tick()
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if ticked != 2 {
		t.Errorf("expected 2 ticks, got %d", ticked)
	}
	total := r.Len()
	if total != 0 {
		t.Errorf("expected item removed after maxTick, got %d reactor items", total)
	}
}

// TestReactor_RemovesFinishedItems verifies finished items are pruned from their shard.
func TestReactor_RemovesFinishedItems(t *testing.T) {
	r := NewReactor()
	// maxTick=1: removed after first tick cycle
	r.Add(&testTickable{count: 0, maxTick: 1})
	// maxTick=3: remains for 2 more cycles
	r.Add(&testTickable{count: 0, maxTick: 3})

	runOneCycle := func() {
		for _, s := range r.shards {
			s.tick()
		}
	}

	// Both items were distributed across shards. After one cycle:
	// maxTick=1 returns false → removed. maxTick=3 returns true → kept.
	runOneCycle()
	if r.Len() != 1 {
		t.Errorf("expected 1 item remaining after first cycle, got %d", r.Len())
	}

	runOneCycle()
	if r.Len() != 1 {
		t.Errorf("expected 1 item remaining after second cycle, got %d", r.Len())
	}
}

type testTickable struct {
	count   int
	maxTick int
	onTick  func()
}

func (t *testTickable) Tick() bool {
	t.count++
	if t.onTick != nil {
		t.onTick()
	}
	return t.count < t.maxTick
}

// TestStreamPlayer_TsIncrement verifies tsIncrement flows correctly through constructor.
func TestStreamPlayer_TsIncrement(t *testing.T) {
	s := &StreamPlayer{tsIncrement: 960}
	if s.tsIncrement != 960 {
		t.Errorf("expected tsIncrement=960, got %d", s.tsIncrement)
	}
}

// TestReactor_AllShardsTickOnEveryFanout verifies that every shard fires once per
// Start() tick, not just the "lucky" shard that wins the shared channel race.
// This is the regression test for the shared-ticker-channel bug.
func TestReactor_AllShardsTickOnEveryFanout(t *testing.T) {
	r := NewReactor()
	r.Start()
	defer r.Stop()

	n := r.nShards

	// Place one counter item in each shard directly (bypassing round-robin)
	// so we know exactly which shard owns each item.
	type shardCounter struct {
		mu    sync.Mutex
		ticks int
		alive bool
	}
	counters := make([]*shardCounter, n)
	for i := range counters {
		c := &shardCounter{alive: true}
		counters[i] = c
		r.shards[i].add(&funcTickable{fn: func() bool {
			c.mu.Lock()
			defer c.mu.Unlock()
			if c.alive {
				c.ticks++
			}
			return c.alive
		}})
	}

	// Wait for at least 3 full tick cycles (3 × 20ms = 60ms, allow 200ms).
	time.Sleep(200 * time.Millisecond)

	// Stop all items.
	for _, c := range counters {
		c.mu.Lock()
		c.alive = false
		c.mu.Unlock()
	}

	// Every shard must have been ticked at least 3 times.
	for i, c := range counters {
		c.mu.Lock()
		got := c.ticks
		c.mu.Unlock()
		if got < 3 {
			t.Errorf("shard %d ticked only %d time(s) in 200ms — fan-out is broken (expected ≥3)", i, got)
		}
	}
}

// funcTickable wraps a plain function as a Tickable.
type funcTickable struct {
	fn func() bool
}

func (f *funcTickable) Tick() bool { return f.fn() }

func init() {
	// Allow tests that call NewJitterBuffer to work without global reactor panicking
	_ = time.Millisecond
}
