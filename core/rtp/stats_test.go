package rtp

import (
	"sync"
	"testing"
	"time"
)

// ── SendStats ─────────────────────────────────────────────────────────────────

// TestSendStats_AtomicAdd verifies that Add() is reflected by Load().
func TestSendStats_AtomicAdd(t *testing.T) {
	var s SendStats
	s.PacketsSent.Add(1)
	s.PacketsSent.Add(1)
	s.OctetsSent.Add(160)

	if got := s.PacketsSent.Load(); got != 2 {
		t.Errorf("PacketsSent: want 2, got %d", got)
	}
	if got := s.OctetsSent.Load(); got != 160 {
		t.Errorf("OctetsSent: want 160, got %d", got)
	}
}

// TestSendStats_ConcurrentAdd verifies no race condition on concurrent writers
// (the -race flag will catch any actual data race).
func TestSendStats_ConcurrentAdd(t *testing.T) {
	var s SendStats
	var wg sync.WaitGroup
	const goroutines = 20
	const addsPerGoroutine = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < addsPerGoroutine; j++ {
				s.PacketsSent.Add(1)
				s.OctetsSent.Add(160)
			}
		}()
	}
	wg.Wait()

	want := int64(goroutines * addsPerGoroutine)
	if got := s.PacketsSent.Load(); got != want {
		t.Errorf("PacketsSent: want %d, got %d", want, got)
	}
	if got := s.OctetsSent.Load(); got != want*160 {
		t.Errorf("OctetsSent: want %d, got %d", want*160, got)
	}
}

// ── RTPStats.Snapshot ─────────────────────────────────────────────────────────

// TestRTPStats_Snapshot_Empty verifies Snapshot() on a zero-value RTPStats.
func TestRTPStats_Snapshot_Empty(t *testing.T) {
	var s RTPStats
	snap := s.Snapshot()
	if snap.PacketsReceived != 0 || snap.PacketsLost != 0 || snap.Jitter != 0 {
		t.Errorf("empty snapshot should be all-zero, got %+v", snap)
	}
	if snap.PacketLossPct != 0 {
		t.Errorf("empty loss pct: want 0, got %f", snap.PacketLossPct)
	}
}

// TestRTPStats_Snapshot_Consistent verifies Snapshot() returns values consistent
// with each other (PacketLossPct matches PacketsLost/total).
func TestRTPStats_Snapshot_Consistent(t *testing.T) {
	var s RTPStats
	now := time.Now()

	// Simulate 10 received, 5 gaps (lost), via update()
	for seq := uint16(100); seq < 116; seq++ {
		if seq%2 == 0 {
			s.update(seq, now)
		}
		now = now.Add(40 * time.Millisecond) // 40ms between received (1 lost each time)
	}

	snap := s.Snapshot()
	if snap.PacketsReceived == 0 {
		t.Fatal("expected some received packets")
	}
	total := snap.PacketsReceived + snap.PacketsLost
	want := float64(snap.PacketsLost) * 100 / float64(total)
	if snap.PacketLossPct != want {
		t.Errorf("PacketLossPct inconsistent: computed %f, snapshot has %f", want, snap.PacketLossPct)
	}
}

// TestRTPStats_Snapshot_ConcurrentRead verifies that Snapshot() is safe to call
// concurrently with update() — the race detector will catch any race.
func TestRTPStats_Snapshot_ConcurrentRead(t *testing.T) {
	var s RTPStats
	stop := make(chan struct{})

	// Writer goroutine: continuously updates stats
	go func() {
		seq := uint16(0)
		for {
			select {
			case <-stop:
				return
			default:
				s.update(seq, time.Now())
				seq++
			}
		}
	}()

	// Reader goroutine: takes snapshots concurrently
	for i := 0; i < 1000; i++ {
		_ = s.Snapshot()
	}
	close(stop)
}

// ── RTPStats.update — extended sequence number (rollover) ────────────────────

// TestRTPStats_HighestSeqExtended_NoRollover checks extended seq without rollover.
func TestRTPStats_HighestSeqExtended_NoRollover(t *testing.T) {
	var s RTPStats
	now := time.Now()

	s.update(100, now)
	now = now.Add(20 * time.Millisecond)
	s.update(101, now)
	now = now.Add(20 * time.Millisecond)
	s.update(105, now) // gap of 3

	snap := s.Snapshot()
	// No rollover: extended = (0<<16)|105 = 105
	if snap.HighestSeqExtended != 105 {
		t.Errorf("HighestSeqExtended: want 105, got %d", snap.HighestSeqExtended)
	}
}

// TestRTPStats_HighestSeqExtended_Rollover checks that a seq number wraparound
// increments the rollover counter, producing the correct RFC 3550 extended seq.
func TestRTPStats_HighestSeqExtended_Rollover(t *testing.T) {
	var s RTPStats
	now := time.Now()

	// Start near the 16-bit max
	s.update(65534, now)
	now = now.Add(20 * time.Millisecond)
	s.update(65535, now)
	now = now.Add(20 * time.Millisecond)

	// Rollover: seq goes from 65535 → 0
	s.update(0, now)

	snap := s.Snapshot()
	// After one rollover: rollover=1, seq=0 → extended = (1<<16)|0 = 65536
	if snap.HighestSeqExtended != 65536 {
		t.Errorf("after rollover: want HighestSeqExtended=65536, got %d", snap.HighestSeqExtended)
	}
}

// TestRTPStats_HighestSeqExtended_MultipleRollovers checks two rollovers.
func TestRTPStats_HighestSeqExtended_MultipleRollovers(t *testing.T) {
	var s RTPStats
	now := time.Now()

	// First rollover
	s.update(65535, now)
	now = now.Add(20 * time.Millisecond)
	s.update(0, now) // rollover 1

	now = now.Add(20 * time.Millisecond)
	s.update(100, now)

	// Second rollover
	now = now.Add(20 * time.Millisecond)
	s.update(65535, now) // jump back near max
	now = now.Add(20 * time.Millisecond)
	s.update(10, now) // rollover 2

	snap := s.Snapshot()
	// rollover=2, seq=10 → extended = (2<<16)|10 = 131082
	if snap.HighestSeqExtended != (2<<16)|10 {
		t.Errorf("two rollovers: want %d, got %d", (2<<16)|10, snap.HighestSeqExtended)
	}
}

// TestRTPStats_PacketLoss_WithGaps verifies loss counting on sequential gaps.
func TestRTPStats_PacketLoss_WithGaps(t *testing.T) {
	var s RTPStats
	now := time.Now()

	// seq 0 (anchor), then seq 5 (gap of 4 lost)
	s.update(0, now)
	now = now.Add(20 * time.Millisecond)
	s.update(5, now)

	snap := s.Snapshot()
	if snap.PacketsLost != 4 {
		t.Errorf("expected 4 lost, got %d", snap.PacketsLost)
	}
	if snap.PacketsReceived != 2 {
		t.Errorf("expected 2 received, got %d", snap.PacketsReceived)
	}
}

// TestRTPStats_PacketLossPercent_Zero verifies 0% loss on no-loss stream.
func TestRTPStats_PacketLossPercent_Zero(t *testing.T) {
	var s RTPStats
	now := time.Now()
	for seq := uint16(0); seq < 10; seq++ {
		s.update(seq, now)
		now = now.Add(20 * time.Millisecond)
	}
	if pct := s.PacketLossPercent(); pct != 0 {
		t.Errorf("expected 0%% loss, got %.2f", pct)
	}
}

// ── RTPStats.RecvErrors (fix 6.5) ─────────────────────────────────────────────

// TestRTPStats_RecvErrors_InSnapshot verifies that RecvErrors increments are
// reflected in the Snapshot returned to RTCP and call finalisation.
func TestRTPStats_RecvErrors_InSnapshot(t *testing.T) {
	var s RTPStats
	s.RecvErrors.Add(3)

	snap := s.Snapshot()
	if snap.RecvErrors != 3 {
		t.Errorf("Snapshot RecvErrors: want 3, got %d", snap.RecvErrors)
	}
}

// TestRTPStats_RecvErrors_ZeroByDefault verifies a fresh RTPStats has zero errors.
func TestRTPStats_RecvErrors_ZeroByDefault(t *testing.T) {
	var s RTPStats
	if snap := s.Snapshot(); snap.RecvErrors != 0 {
		t.Errorf("fresh RTPStats: RecvErrors want 0, got %d", snap.RecvErrors)
	}
}

// TestRTPStats_RecvErrors_ConcurrentAdd verifies concurrent increments are
// race-free. The -race flag will catch any actual data race.
func TestRTPStats_RecvErrors_ConcurrentAdd(t *testing.T) {
	var s RTPStats
	var wg sync.WaitGroup
	const goroutines = 20
	const addsEach = 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < addsEach; j++ {
				s.RecvErrors.Add(1)
			}
		}()
	}
	wg.Wait()

	snap := s.Snapshot()
	want := goroutines * addsEach
	if snap.RecvErrors != want {
		t.Errorf("concurrent RecvErrors: want %d, got %d", want, snap.RecvErrors)
	}
}
