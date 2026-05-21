package rtp

import (
	"sync"
	"time"

	pionrtp "github.com/pion/rtp"
)

// DTMFPayloadType is the standard dynamic payload type for RFC 2833 telephone events.
const DTMFPayloadType = uint8(101)

// digitCode maps DTMF digit characters to RFC 2833 event codes.
var digitCode = map[string]byte{
	"0": 0, "1": 1, "2": 2, "3": 3, "4": 4,
	"5": 5, "6": 6, "7": 7, "8": 8, "9": 9,
	"*": 10, "#": 11,
	"A": 12, "B": 13, "C": 14, "D": 15,
}

// codeDigit is the reverse of digitCode: RFC 2833 event code → digit string.
var codeDigit [16]string

func init() {
	for d, c := range digitCode {
		codeDigit[c] = d
	}
}

// decodeDTMFEvent decodes a 4-byte RFC 2833 telephone-event payload.
// Returns the digit string, whether this is the end-of-event packet, and ok.
func decodeDTMFEvent(payload []byte) (digit string, end bool, ok bool) {
	if len(payload) < 4 {
		return "", false, false
	}
	code := payload[0]
	if int(code) >= len(codeDigit) || codeDigit[code] == "" {
		return "", false, false
	}
	return codeDigit[code], payload[1]&0x80 != 0, true
}

// DTMFCollector collects DTMF digits received as inbound RFC 2833 telephone-event
// packets. It deduplicates multi-packet events (only the end-of-event packet counts)
// so each user-visible digit is recorded exactly once.
// All methods are goroutine-safe.
type DTMFCollector struct {
	mu      sync.Mutex
	digits  []string
	PT      uint8  // negotiated telephone-event payload type
	lastTS  uint32 // RTP timestamp of the last accepted end event
	lastEvt byte   // RFC 2833 event code of the last accepted end event
	seeded  bool
}

// NewDTMFCollector creates a collector for inbound telephone-event packets
// with the given negotiated payload type. If telPT is 0 the standard
// dynamic PT 101 is used as the default.
func NewDTMFCollector(telPT uint8) *DTMFCollector {
	pt := telPT
	if pt == 0 {
		pt = DTMFPayloadType
	}
	return &DTMFCollector{PT: pt}
}

// accept records a single digit (called on end-of-event packet).
// Deduplication is based on (RTP timestamp, event code) — RFC 2833 allows
// multiple retransmissions of the end packet with incrementing sequence numbers
// but the same timestamp, so seq-based dedup would record false duplicates.
func (c *DTMFCollector) accept(ts uint32, eventCode byte, digit string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seeded && c.lastTS == ts && c.lastEvt == eventCode {
		return // retransmission of the same end-of-event — skip
	}
	c.lastTS = ts
	c.lastEvt = eventCode
	c.seeded = true
	c.digits = append(c.digits, digit)
}

// Digits returns a snapshot of all collected digits in order of arrival.
func (c *DTMFCollector) Digits() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]string, len(c.digits))
	copy(cp, c.digits)
	return cp
}

// MatchesExpected returns true when every digit in expected appears in the
// collected sequence in order (subsequence / prefix match).
// An empty expected slice always returns true.
func (c *DTMFCollector) MatchesExpected(expected []string) bool {
	if len(expected) == 0 {
		return true
	}
	c.mu.Lock()
	got := make([]string, len(c.digits)) // deep copy under lock to avoid race
	copy(got, c.digits)
	c.mu.Unlock()
	ei := 0
	for _, d := range got {
		if d == expected[ei] {
			ei++
			if ei == len(expected) {
				return true
			}
		}
	}
	return false
}

// SendDTMF transmits a single DTMF digit as RFC 2833 RTP telephone-event packets.
// The digit is sent as 5 intermediate packets (each 20ms) plus one end packet,
// for a total on-wire duration of ~120ms per digit.
//
// seq and ts must be the session's current counters; they are advanced via
// sess.NextSeqTS() so the DTMF stream is coherent with the audio stream.
func SendDTMF(sess *Session, digit string) {
	code, ok := digitCode[digit]
	if !ok {
		return // unknown digit — silently skip
	}

	const (
		volume   = byte(10)  // -10 dBm0
		duration = uint16(0) // will be incremented per packet (units: 8kHz timestamp ticks)
	)

	// Send 5 intermediate packets
	var dur uint16
	for i := 0; i < 5; i++ {
		dur = uint16((i + 1) * 160) // 160 ticks = 20ms at 8kHz

		seq, ts := sess.NextSeqTS(160)

		payload := buildDTMFPayload(code, volume, dur, false)
		pkt := buildDTMFPacket(sess, seq, ts, payload, i == 0)

		raw, _ := pkt.Marshal()
		_ = sess.Send(raw)
		time.Sleep(20 * time.Millisecond)
	}

	// Send end packet (marker = end bit in payload)
	seq, ts := sess.NextSeqTS(0) // timestamp does NOT advance for end event
	endPayload := buildDTMFPayload(code, volume, dur, true)
	endPkt := buildDTMFPacket(sess, seq, ts, endPayload, false)
	raw, _ := endPkt.Marshal()
	_ = sess.Send(raw)
}

// buildDTMFPayload constructs the 4-byte RFC 2833 telephone-event payload.
//
//	Byte 0: event code
//	Byte 1: E-bit (end), R-bit (reserved), volume (6 bits)
//	Bytes 2-3: duration (network byte order)
func buildDTMFPayload(code, volume byte, duration uint16, end bool) []byte {
	flags := volume & 0x3F
	if end {
		flags |= 0x80 // E-bit
	}
	return []byte{
		code,
		flags,
		byte(duration >> 8),
		byte(duration),
	}
}

func buildDTMFPacket(sess *Session, seq uint16, ts uint32, payload []byte, marker bool) *pionrtp.Packet {
	return &pionrtp.Packet{
		Header: pionrtp.Header{
			Version:        2,
			PayloadType:    DTMFPayloadType,
			SequenceNumber: seq,
			Timestamp:      ts,
			SSRC:           sess.SSRC,
			Marker:         marker, // mark first packet of each digit event
		},
		Payload: payload,
	}
}
