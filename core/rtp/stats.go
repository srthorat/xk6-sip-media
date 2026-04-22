// Package rtp contains types shared across the RTP send/receive/DTMF pipeline.
package rtp

import "time"

// SendStats tracks outbound RTP packet counters.
type SendStats struct {
	PacketsSent int
}

// RTPStats tracks inbound RTP packet quality metrics.
type RTPStats struct {
	PacketsReceived int
	PacketsLost     int
	Jitter          float64 // running average, milliseconds

	lastSeq     uint16
	lastArrival time.Time
	initialized bool
}

// PacketLossPercent returns the fraction of expected packets that were lost.
func (s *RTPStats) PacketLossPercent() float64 {
	total := s.PacketsReceived + s.PacketsLost
	if total == 0 {
		return 0
	}
	return float64(s.PacketsLost) * 100.0 / float64(total)
}

// update ingests a newly received packet's sequence number and arrival time.
func (s *RTPStats) update(seq uint16, arrival time.Time) {
	if !s.initialized {
		s.lastSeq = seq
		s.lastArrival = arrival
		s.initialized = true
		s.PacketsReceived++
		return
	}

	s.PacketsReceived++

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
}
