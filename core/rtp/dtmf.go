package rtp

import (
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
