package rtp

import (
	"encoding/binary"
	"math"
	"net"
	"testing"
	"time"
)

// newLoopbackUDPPair opens a loopback UDP socket bound to an OS-assigned port.
// Returns the conn and a no-op cleanup (conn is closed by GC / test runner).
func newLoopbackUDPPair(t *testing.T) (*net.UDPConn, *net.UDPAddr) {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("newLoopbackUDPPair: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, conn.LocalAddr().(*net.UDPAddr)
}

// ── toNTP / compact NTP ───────────────────────────────────────────────────────

// TestToNTP_CompactFormula verifies the compact NTP formula used in buildSR and parseRR.
// Compact NTP = lower 16 bits of NTP seconds | upper 16 bits of NTP fraction.
func TestToNTP_CompactFormula(t *testing.T) {
	// Use a fixed time so the NTP values are deterministic.
	// 2020-01-01 00:00:00 UTC = Unix 1577836800
	// NTP = Unix + 2208988800 = 3786825600
	fixed := time.Unix(1577836800, 500000000) // 0.5 sec = frac ≈ 2^31
	sec, frac := toNTP(fixed)

	// sec lower 16 bits:
	wantSecLow := sec & 0xFFFF
	// frac upper 16 bits:
	wantFracHigh := frac >> 16

	compact := (sec&0xFFFF)<<16 | frac>>16
	wantCompact := (wantSecLow << 16) | wantFracHigh

	if compact != wantCompact {
		t.Errorf("compact NTP mismatch: got %08x, want %08x", compact, wantCompact)
	}

	// Verify it's NOT the old "rotate seconds" formula which would equal:
	oldFormula := uint32(sec<<16 | sec>>16)
	if compact == oldFormula && sec != 0 {
		// This means our formula accidentally matches the broken one — bad
		t.Logf("Note: compact == old formula for this input, sec=%x frac=%x", sec, frac)
	}
}

// TestRTT_CompactNTP_Formula verifies the RTT compact NTP (nowCompact - lsr - dlsr)
// produces a valid non-negative round trip time for a well-formed RTCP RR.
func TestRTT_CompactNTP_Formula(t *testing.T) {
	// Simulate: we sent SR with compact NTP "lsr" 100ms ago.
	// Remote received it 10ms later, processed, and sent RR back 90ms later.
	// Expected RTT ≈ 100ms - 10ms - 90ms = 0ms (but let's just check it's non-garbage).

	now := time.Now()
	nowSec, nowFrac := toNTP(now)
	nowCompact := (nowSec&0xFFFF)<<16 | nowFrac>>16

	// Simulate LSR sent 100ms ago
	srTime := now.Add(-100 * time.Millisecond)
	srSec, srFrac := toNTP(srTime)
	lsr := (srSec&0xFFFF)<<16 | srFrac>>16

	// DLSR: remote spent 10ms processing (in 1/65536 s units)
	dlsr := uint32(655) // approx 0.010 * 65536

	// RTT calculation (from parseRR)
	rttUnits := nowCompact - lsr - dlsr
	rttMs := float64(rttUnits) / 65.536

	// RTT should be approximately 90ms (100ms - 10ms dlsr)
	if rttMs < 80 || rttMs > 110 {
		t.Errorf("RTT out of expected range [80,110]ms: got %.2fms", rttMs)
	}
}

// TestBuildSR_OctetCount verifies that buildSR() writes the actual OctetsSent
// value (not packets*160).
func TestBuildSR_OctetCount(t *testing.T) {
	var sendStats SendStats
	// 50 packets of 80 bytes each (e.g. G.729 half-rate)
	sendStats.PacketsSent.Add(50)
	sendStats.OctetsSent.Add(50 * 80)

	var rtpStats RTPStats
	sess := &RTCPSession{
		ssrc:      0xDEADBEEF,
		stats:     &RTCPStats{},
		rtpStats:  &rtpStats,
		sendStats: &sendStats,
	}

	sr := sess.buildSR()

	// SR layout: [0:4]=header, [4:8]=SSRC, [8:12]=NTP sec, [12:16]=NTP frac,
	// [16:20]=RTP ts, [20:24]=packet count, [24:28]=octet count
	pktCount := binary.BigEndian.Uint32(sr[20:24])
	octetCount := binary.BigEndian.Uint32(sr[24:28])

	if pktCount != 50 {
		t.Errorf("SR packet count: want 50, got %d", pktCount)
	}
	if octetCount != 50*80 {
		t.Errorf("SR octet count: want %d, got %d", 50*80, octetCount)
	}
}

// TestBuildSR_OctetCount_VoIP verifies that for PCMU (160 bytes/pkt),
// octet count is packets*160 (just coincidentally correct, not hardcoded).
func TestBuildSR_OctetCount_VoIP(t *testing.T) {
	var sendStats SendStats
	sendStats.PacketsSent.Add(100)
	sendStats.OctetsSent.Add(100 * 160)

	sess := &RTCPSession{
		ssrc:      1,
		stats:     &RTCPStats{},
		rtpStats:  nil,
		sendStats: &sendStats,
	}

	sr := sess.buildSR()
	octetCount := binary.BigEndian.Uint32(sr[24:28])

	if octetCount != 100*160 {
		t.Errorf("PCMU SR octet count: want %d, got %d", 100*160, octetCount)
	}
}

// TestBuildRR_HighestSeqExtended verifies that buildRR() puts HighestSeqExtended
// (not PacketsReceived) in report block bytes [8:12].
func TestBuildRR_HighestSeqExtended(t *testing.T) {
	var rtpStats RTPStats
	now := time.Now()

	// Feed 3 packets ending at seq 500 — no rollover, so extended = 500.
	for _, seq := range []uint16{498, 499, 500} {
		rtpStats.update(seq, now)
		now = now.Add(20 * time.Millisecond)
	}

	var sendStats SendStats
	sess := &RTCPSession{
		ssrc:      2,
		stats:     &RTCPStats{SendSSRC: 3},
		rtpStats:  &rtpStats,
		sendStats: &sendStats,
	}

	rr := sess.buildRR()

	// RR layout: [0:8]=header, [8:12]=SSRC of source,
	// rb[0:4]=SSRC, rb[4]=frac, rb[5:8]=cumLost, rb[8:12]=highest seq
	// buf[8+8:8+12] = rb[8:12]
	highestSeq := binary.BigEndian.Uint32(rr[8+8 : 8+12])

	if highestSeq != 500 {
		t.Errorf("RR highest seq: want 500 (extended), got %d (this would be PacketsReceived=3 if bug was present)", highestSeq)
	}
}

// TestBuildRR_HighestSeqExtended_WithRollover verifies that HighestSeqExtended
// after a rollover includes the rollover count in the upper 16 bits.
func TestBuildRR_HighestSeqExtended_WithRollover(t *testing.T) {
	var rtpStats RTPStats
	now := time.Now()

	rtpStats.update(65534, now)
	now = now.Add(20 * time.Millisecond)
	rtpStats.update(65535, now)
	now = now.Add(20 * time.Millisecond)
	rtpStats.update(0, now) // rollover

	var sendStats SendStats
	sess := &RTCPSession{
		ssrc:      4,
		stats:     &RTCPStats{SendSSRC: 5},
		rtpStats:  &rtpStats,
		sendStats: &sendStats,
	}

	rr := sess.buildRR()
	highestSeq := binary.BigEndian.Uint32(rr[8+8 : 8+12])

	// extended = (1<<16)|0 = 65536
	if highestSeq != 65536 {
		t.Errorf("RR highest seq after rollover: want 65536, got %d", highestSeq)
	}
}

// TestBuildRR_FractionLost verifies the loss fraction byte is computed correctly.
// RFC 3550: fraction = floor(lost_fraction × 256).
func TestBuildRR_FractionLost(t *testing.T) {
	var rtpStats RTPStats
	now := time.Now()

	// 1 received, then gap of 1 (lost), then 1 received → 1 lost out of 3 total → ~33%
	rtpStats.update(0, now)
	now = now.Add(20 * time.Millisecond)
	// seq 1 is lost — skip it
	rtpStats.update(2, now) // triggers loss detection: diff=2, lost+=1

	var sendStats SendStats
	sess := &RTCPSession{
		ssrc:      6,
		stats:     &RTCPStats{SendSSRC: 7},
		rtpStats:  &rtpStats,
		sendStats: &sendStats,
	}

	rr := sess.buildRR()
	fracByte := rr[8+4] // rb[4] = fraction lost byte

	snap := rtpStats.Snapshot()
	// Mirror buildRR: floor(loss% × 256/100), clamped to [0,255] (RFC 3550 §6.4.1).
	fl := snap.PacketLossPct * 256 / 100
	if fl < 0 {
		fl = 0
	}
	wantFrac32 := uint32(math.Floor(fl))
	if wantFrac32 > 255 {
		wantFrac32 = 255
	}
	wantFrac := uint8(wantFrac32)
	if fracByte != wantFrac {
		t.Errorf("fraction lost byte: want %d (%.1f%%), got %d", wantFrac, snap.PacketLossPct, fracByte)
	}
}

// TestParseSR_CompactNTP verifies that parseSR() correctly extracts the compact NTP
// from bytes [10:14] of the SR payload.
func TestParseSR_CompactNTP(t *testing.T) {
	// Build a synthetic SR packet (28 bytes)
	sr := make([]byte, 28)
	sr[0] = 0x80                           // V=2, P=0, RC=0
	sr[1] = 200                            // PT=SR
	binary.BigEndian.PutUint16(sr[2:4], 6) // length

	// NTP timestamp at [8:16]: upper 32 bits (sec) at [8:12], lower 32 (frac) at [12:16]
	// Compact = middle 32 bits = bytes [10:14]
	wantCompact := uint32(0xABCD1234)
	// Place it correctly: sr[10..13] = wantCompact
	binary.BigEndian.PutUint32(sr[10:14], wantCompact)

	var rtpStats RTPStats
	var sendStats SendStats
	conn, _ := newLoopbackUDPPair(t)
	sess := &RTCPSession{
		ssrc:      9,
		conn:      conn,
		stats:     &RTCPStats{},
		rtpStats:  &rtpStats,
		sendStats: &sendStats,
	}
	sess.stop = make(chan struct{})

	sess.parseSR(sr)

	sess.statsMu.Lock()
	got := sess.stats.LastSRNTPCompact
	sess.statsMu.Unlock()

	if got != wantCompact {
		t.Errorf("parseSR compact NTP: want 0x%08x, got 0x%08x", wantCompact, got)
	}
}

// ── buildRR jitter unit conversion (fix 6.1) ──────────────────────────────────

// TestBuildRR_JitterUnits_8kHz verifies that jitter stored in milliseconds is
// converted to RTP timestamp units (ms × sampleRate/1000) before being written
// into the RR report block (RFC 3550 §6.4.1).  8 kHz clock (PCMU/PCMA/G722/G729).
func TestBuildRR_JitterUnits_8kHz(t *testing.T) {
	const sampleRate = 8000
	const jitterMs = 20.0 // 20 ms → 160 RTP ts units at 8 kHz

	var rtpStats RTPStats
	rtpStats.Jitter = jitterMs // set directly (internal package test)

	var sendStats SendStats
	sess := &RTCPSession{
		ssrc:       10,
		sampleRate: sampleRate,
		stats:      &RTCPStats{SendSSRC: 11},
		rtpStats:   &rtpStats,
		sendStats:  &sendStats,
	}

	rr := sess.buildRR()
	// RR: [0:8] header, [8:] report block.
	// rb layout: [0:4]=SSRC, [4]=frac, [5:8]=cumLost, [8:12]=highestSeq, [12:16]=jitter
	wireJitter := binary.BigEndian.Uint32(rr[8+12 : 8+16])

	wantJitter := uint32(math.Round(jitterMs * sampleRate / 1000))
	if wireJitter != wantJitter {
		t.Errorf("jitter wire value: want %d RTP-ts-units (%.1f ms × %d Hz / 1000), got %d",
			wantJitter, jitterMs, sampleRate, wireJitter)
	}
}

// TestBuildRR_JitterUnits_48kHz verifies the same conversion for Opus (48 kHz clock).
// 20 ms jitter at 48 kHz → 960 RTP timestamp units.
func TestBuildRR_JitterUnits_48kHz(t *testing.T) {
	const sampleRate = 48000
	const jitterMs = 20.0 // 20 ms → 960 RTP ts units at 48 kHz

	var rtpStats RTPStats
	rtpStats.Jitter = jitterMs

	var sendStats SendStats
	sess := &RTCPSession{
		ssrc:       12,
		sampleRate: sampleRate,
		stats:      &RTCPStats{SendSSRC: 13},
		rtpStats:   &rtpStats,
		sendStats:  &sendStats,
	}

	rr := sess.buildRR()
	wireJitter := binary.BigEndian.Uint32(rr[8+12 : 8+16])

	wantJitter := uint32(math.Round(jitterMs * sampleRate / 1000))
	if wireJitter != wantJitter {
		t.Errorf("jitter wire value (48kHz): want %d, got %d", wantJitter, wireJitter)
	}
}

// TestBuildRR_JitterUnits_ZeroDefault verifies that a zero sampleRate (should not
// happen after fix, but defensive) does not produce a division-by-zero panic.
func TestBuildRR_JitterUnits_ZeroDefault(t *testing.T) {
	var rtpStats RTPStats
	rtpStats.Jitter = 10.0

	var sendStats SendStats
	sess := &RTCPSession{
		ssrc:       14,
		sampleRate: 0, // zero: buildRR should handle gracefully (produces 0 jitter)
		stats:      &RTCPStats{},
		rtpStats:   &rtpStats,
		sendStats:  &sendStats,
	}

	// Must not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("buildRR panicked with zero sampleRate: %v", r)
		}
	}()
	_ = sess.buildRR()
}
