// Package rtp contains types shared across the RTP send/receive/DTMF pipeline.
package rtp

import (
	"sync"
	"sync/atomic"
	"time"
)

// SendStats tracks outbound RTP packet counters.
// Fields use sync/atomic: the reactor shard writes concurrently with the
// RTCP goroutine reading.
type SendStats struct {
	PacketsSent atomic.Int64
	OctetsSent  atomic.Int64 // total unencrypted payload bytes sent
	BytesSent   atomic.Int64 // total wire bytes sent (RTP header + payload)
}

// RTPStatsSnapshot is an immutable point-in-time copy of receive stats.
// Used by RTCP and other concurrent readers that must not hold the lock.
type RTPStatsSnapshot struct {
	PacketsReceived    int
	PacketsLost        int
	Jitter             float64
	HighestSeqExtended uint32 // RFC 3550 extended seq: (rollover<<16)|highest
	PacketLossPct      float64
	RecvErrors         int   // non-timeout socket errors (e.g. EMSGSIZE, ECONNREFUSED)
	BytesReceived      int64 // total payload bytes received
}

// RTPStats tracks inbound RTP packet quality metrics.
// Protected by mu for concurrent access from receiver and RTCP goroutines.
type RTPStats struct {
	mu sync.Mutex

	PacketsReceived int
	PacketsLost     int
	Jitter          float64      // running average, milliseconds
	BytesReceived   atomic.Int64 // total payload bytes received (written atomically)

	lastSeq            uint16
	lastArrival        time.Time
	initialized        bool
	rollover           uint16 // sequence number rollover count
	highestSeqExtended uint32 // RFC 3550 extended highest sequence number

	// RecvErrors counts non-timeout socket errors (e.g. EMSGSIZE, ECONNREFUSED).
	// Written atomically from the receiver goroutine; no need to hold mu.
	RecvErrors atomic.Int64
}

// PacketLossPercent returns the fraction of expected packets that were lost.
func (s *RTPStats) PacketLossPercent() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lossPercent()
}

// lossPercent computes loss percentage. Caller must hold s.mu.
func (s *RTPStats) lossPercent() float64 {
	total := s.PacketsReceived + s.PacketsLost
	if total == 0 {
		return 0
	}
	return float64(s.PacketsLost) * 100.0 / float64(total)
}

// Snapshot returns a consistent, mutex-safe copy of receive stats.
// Safe to call from any goroutine concurrently with the receiver goroutine.
func (s *RTPStats) Snapshot() RTPStatsSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return RTPStatsSnapshot{
		PacketsReceived:    s.PacketsReceived,
		PacketsLost:        s.PacketsLost,
		Jitter:             s.Jitter,
		HighestSeqExtended: s.highestSeqExtended,
		PacketLossPct:      s.lossPercent(),
		RecvErrors:         int(s.RecvErrors.Load()),
		BytesReceived:      s.BytesReceived.Load(),
	}
}

// update ingests a newly received packet's sequence number and arrival time.
func (s *RTPStats) update(seq uint16, arrival time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		s.lastSeq = seq
		s.lastArrival = arrival
		s.initialized = true
		s.PacketsReceived++
		s.highestSeqExtended = uint32(seq)
		return
	}

	s.PacketsReceived++

	// Extended sequence number tracking (RFC 3550 §A.1): detect rollovers
	// before updating lastSeq so we compare old vs new value.
	seqDiff := int32(seq) - int32(s.lastSeq)
	if seqDiff < -0x8000 { // sequence number wrapped around
		s.rollover++
	}
	extended := uint32(s.rollover)<<16 | uint32(seq)
	if extended > s.highestSeqExtended {
		s.highestSeqExtended = extended
	}

	// Packet loss: detect gaps in sequence numbers.
	// Guard against 16-bit wraparound by checking direction.
	diff := int(seq) - int(s.lastSeq)
	if diff < 0 {
		diff += 65536
	}
	if diff > 1 {
		// Packets in the gap are considered lost.
		s.PacketsLost += diff - 1
	}
	s.lastSeq = seq

	// Jitter: RFC 3550 §A.8 simplified — exponential moving average of
	// inter-arrival deviation from the expected 20ms interval.
	if !s.lastArrival.IsZero() {
		intervalMs := float64(arrival.Sub(s.lastArrival).Milliseconds())
		const expected = float64(20)
		deviation := intervalMs - expected
		if deviation < 0 {
			deviation = -deviation
		}
		// EMA with α = 0.1
		s.Jitter = s.Jitter*0.9 + deviation*0.1
	}
	s.lastArrival = arrival
}

// Ensure atomic types are used (compile-time check).
var _ = atomic.Int64{}

// CallResult is the summary returned after a call ends.
type CallResult struct {
	PacketsSent        int
	PacketsReceived    int
	PacketsLost        int
	Jitter             float64 // average jitter, ms
	PacketLossPct      float64 // RTP packet loss percentage
	MOS                float64 // E-model estimate
	PESQScore          float64 // 0 if PESQ not run
	IVRValid           bool    // IVR validation result
	SilenceRatio       float64 // fraction of silent received audio
	RTTMs              float64 // RTCP round-trip time in milliseconds
	RTCPFractionLost   uint8   // RTCP receiver-report fraction lost
	RTCPCumulativeLost uint32  // RTCP receiver-report cumulative lost
	TransferOK         bool    // true if REFER was accepted (202)
	RecvErrors         int     // non-timeout UDP receive errors during the call
	RecorderDrops      int     // frames dropped by the async file-writer (channel full)
	BytesSent          int64   // total RTP payload bytes sent
	BytesReceived      int64   // total RTP payload bytes received
}
