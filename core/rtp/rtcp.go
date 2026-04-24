// Package rtp provides RTCP (RTP Control Protocol, RFC 3550) sender and
// receiver report support.
//
// RTCP runs on a separate port (RTP port + 1) and provides:
//   - Sender Reports (SR): sent by the RTP sender, contains NTP timestamp,
//     RTP timestamp, packet count, and octet count.
//   - Receiver Reports (RR): sent by the RTP receiver, contains fraction lost,
//     cumulative lost, jitter, and LSR/DLSR for RTT calculation.
//
// The RTCP goroutine fires every 5 seconds (the minimum compound interval
// defined in RFC 3550 §6.2).
package rtp

import (
	"encoding/binary"
	"math"
	"net"
	"sync"
	"time"
)

// RTCPStats holds aggregate RTCP statistics for a single stream.
// It is a plain value type — no mutex. The owning RTCPSession protects it
// with its own statsMu field.
type RTCPStats struct {
	// Sender side
	PacketsSentTotal uint32
	OctetsSentTotal  uint32
	SendSSRC         uint32

	// Receiver side (populated from received SR)
	LastSRNTPCompact uint32 // compact NTP (middle 32 bits)
	LastSRReceived   time.Time
	FractionLost     uint8
	CumulativeLost   uint32
	HighestSeqRecv   uint32
	Jitter           float64 // estimated in RTP timestamp units

	// Round-trip time (calculated from DLSR)
	RTTMs float64
}

// RTCPSession manages a bidirectional RTCP flow.
type RTCPSession struct {
	conn       *net.UDPConn
	remoteAddr *net.UDPAddr
	ssrc       uint32
	sampleRate uint32 // RTP clock rate (Hz); used to convert jitter ms → RTP ts units
	stats      *RTCPStats
	statsMu    sync.Mutex // protects stats
	rtpStats   *RTPStats
	sendStats  *SendStats
	stop       <-chan struct{}
}

// NewRTCPSession creates an RTCPSession bound to rtcpPort (rtpPort + 1).
// sampleRate is the codec clock rate in Hz (e.g. 8000 for PCMU/PCMA/G722/G729,
// 48000 for Opus). It is used to convert jitter from milliseconds to RTP
// timestamp units as required by RFC 3550 §6.4.1.
func NewRTCPSession(
	localAddr *net.UDPAddr,
	remoteAddr *net.UDPAddr,
	ssrc uint32,
	sampleRate uint32,
	rtpStats *RTPStats,
	sendStats *SendStats,
) (*RTCPSession, error) {
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, err
	}
	if sampleRate == 0 {
		sampleRate = 8000 // safe default: covers PCMU/PCMA/G722/G729
	}

	return &RTCPSession{
		conn:       conn,
		remoteAddr: remoteAddr,
		ssrc:       ssrc,
		sampleRate: sampleRate,
		stats:      &RTCPStats{SendSSRC: ssrc},
		rtpStats:   rtpStats,
		sendStats:  sendStats,
	}, nil
}

// Run starts the RTCP sender (SR every 5s) and receiver goroutines.
// Blocks until stop is closed.
func (s *RTCPSession) Run(stop <-chan struct{}) {
	s.stop = stop
	done := make(chan struct{})

	// Receiver goroutine
	go func() {
		defer close(done)
		s.receiveLoop()
	}()

	// Sender ticker
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			<-done
			return
		case <-ticker.C:
			sr := s.buildSR()
			_, _ = s.conn.WriteTo(sr, s.remoteAddr)
		}
	}
}

// Stats returns a snapshot copy of RTCP statistics collected so far.
func (s *RTCPSession) Stats() RTCPStats {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	return *s.stats
}

// Close releases the RTCP UDP socket.
func (s *RTCPSession) Close() { _ = s.conn.Close() }

// ── RTCP Sender Report (SR) ──────────────────────────────────────────────

// buildSR constructs a compound RTCP packet: SR + SDES.
func (s *RTCPSession) buildSR() []byte {
	now := time.Now()
	ntpSec, ntpFrac := toNTP(now)
	ntpCompact := uint32(ntpSec<<16) | uint32(ntpFrac>>16)

	s.statsMu.Lock()
	sent := uint32(s.sendStats.PacketsSent.Load())
	octets := uint32(s.sendStats.OctetsSent.Load())
	s.stats.LastSRNTPCompact = ntpCompact
	s.stats.LastSRReceived = now
	s.statsMu.Unlock()

	buf := make([]byte, 28) // SR fixed = 28 bytes

	// RTCP common header: V=2, P=0, RC=0, PT=200 (SR)
	buf[0] = 0x80                           // V=2, P=0, RC=0
	buf[1] = 200                            // PT = SR
	binary.BigEndian.PutUint16(buf[2:4], 6) // length in 32-bit words minus 1

	binary.BigEndian.PutUint32(buf[4:8], s.ssrc)
	binary.BigEndian.PutUint32(buf[8:12], ntpSec)
	binary.BigEndian.PutUint32(buf[12:16], ntpFrac)
	binary.BigEndian.PutUint32(buf[16:20], uint32(now.UnixNano()/125000)) // RTP ts approx
	binary.BigEndian.PutUint32(buf[20:24], sent)
	binary.BigEndian.PutUint32(buf[24:28], octets)

	return buf
}

// ── RTCP Receiver Report (RR) ────────────────────────────────────────────

// buildRR constructs an RTCP Receiver Report from current RTP receive stats.
func (s *RTCPSession) buildRR() []byte {
	if s.rtpStats == nil {
		return nil
	}

	// Use Snapshot() — RTP receive stats are written concurrently by the receiver goroutine.
	snap := s.rtpStats.Snapshot()

	buf := make([]byte, 32) // RR header (8) + one report block (24)

	// RTCP header: V=2, P=0, RC=1, PT=201 (RR)
	buf[0] = 0x81                           // RC=1
	buf[1] = 201                            // PT = RR
	binary.BigEndian.PutUint16(buf[2:4], 7) // 8 words - 1

	binary.BigEndian.PutUint32(buf[4:8], s.ssrc)

	// Report block
	rb := buf[8:]
	binary.BigEndian.PutUint32(rb[0:4], s.stats.SendSSRC)

	// RFC 3550 §6.4.1: fraction lost is floor(loss% × 256/100), clamped to [0,255].
	fractionLost := snap.PacketLossPct * 256 / 100
	if fractionLost < 0 {
		fractionLost = 0
	}
	frac := uint32(math.Floor(fractionLost))
	if frac > 255 {
		frac = 255
	}
	rb[4] = uint8(frac)

	cumLost := snap.PacketsLost
	rb[5] = byte(cumLost >> 16)
	rb[6] = byte(cumLost >> 8)
	rb[7] = byte(cumLost)

	// Extended highest sequence number received (RFC 3550 §6.4.1 field 5).
	binary.BigEndian.PutUint32(rb[8:12], snap.HighestSeqExtended)
	// Interarrival jitter in RTP timestamp units (RFC 3550 §6.4.1).
	// RTPStats stores jitter in milliseconds; convert: ts_units = ms × (sampleRate/1000).
	jitterTS := uint32(math.Round(snap.Jitter * float64(s.sampleRate) / 1000))
	binary.BigEndian.PutUint32(rb[12:16], jitterTS)

	// LSR / DLSR (last SR + delay since last SR)
	s.statsMu.Lock()
	lsr := s.stats.LastSRNTPCompact
	var dlsr uint32
	if !s.stats.LastSRReceived.IsZero() {
		delay := time.Since(s.stats.LastSRReceived)
		dlsr = uint32(delay.Seconds() * 65536) // in 1/65536 second units
	}
	s.statsMu.Unlock()

	binary.BigEndian.PutUint32(rb[16:20], lsr)
	binary.BigEndian.PutUint32(rb[20:24], dlsr)

	return buf
}

// receiveLoop reads incoming RTCP packets and parses SR/RR.
func (s *RTCPSession) receiveLoop() {
	buf := make([]byte, 1500)
	for {
		select {
		case <-s.stop:
			return
		default:
		}

		_ = s.conn.SetReadDeadline(time.Now().Add(time.Second))
		n, _, err := s.conn.ReadFrom(buf)
		if err != nil {
			continue
		}
		s.parseRTCP(buf[:n])
	}
}

// parseRTCP handles an incoming RTCP compound packet.
func (s *RTCPSession) parseRTCP(data []byte) {
	for len(data) >= 4 {
		// pt byte
		pt := data[1]
		length := int(binary.BigEndian.Uint16(data[2:4]))*4 + 4
		if length > len(data) {
			break
		}
		switch pt {
		case 200: // SR
			s.parseSR(data[:length])
		case 201: // RR — remote is reporting back to us; calculate RTT
			s.parseRR(data[:length])
		}
		data = data[length:]
	}
}

func (s *RTCPSession) parseSR(data []byte) {
	if len(data) < 28 {
		return
	}
	// Compact NTP: middle 32 bits of the 8-byte NTP timestamp in the SR.
	// NTP timestamp starts at byte 8 of the SR; bytes 10-13 are the compact form.
	ntpCompact := binary.BigEndian.Uint32(data[10:14])

	s.statsMu.Lock()
	s.stats.LastSRNTPCompact = ntpCompact
	s.stats.LastSRReceived = time.Now()
	s.statsMu.Unlock()

	// Immediately send RR to allow sender RTT calculation
	rr := s.buildRR()
	if rr != nil {
		_, _ = s.conn.WriteTo(rr, s.remoteAddr)
	}
}

func (s *RTCPSession) parseRR(data []byte) {
	if len(data) < 32 {
		return
	}
	lsr := binary.BigEndian.Uint32(data[20:24])
	dlsr := binary.BigEndian.Uint32(data[24:28])

	if lsr == 0 {
		return
	}

	// RTT = now - LSR - DLSR (RFC 3550 §6.4.1)
	now := time.Now()
	nowSec, nowFrac := toNTP(now)
	// Compact NTP: lower 16 bits of seconds | upper 16 bits of fraction.
	nowCompact := (nowSec&0xFFFF)<<16 | nowFrac>>16
	rttUnits := nowCompact - lsr - dlsr
	rttMs := float64(rttUnits) / 65.536 // convert 1/65536 s → ms

	s.statsMu.Lock()
	s.stats.RTTMs = rttMs
	s.statsMu.Unlock()
}

// toNTP converts a time.Time to NTP epoch seconds and fraction.
// NTP epoch is January 1, 1900.
func toNTP(t time.Time) (sec, frac uint32) {
	const ntpEpoch = 2208988800 // seconds between NTP epoch (1900) and Unix epoch (1970)
	unix := t.Unix()
	nano := t.Nanosecond()
	sec = uint32(unix + ntpEpoch)
	frac = uint32(float64(nano) * (1 << 32) / 1e9)
	return
}
