package rtp

import (
	"time"

	pionrtp "github.com/pion/rtp"
)

// Stream sends pre-encoded RTP payloads from payloads[] at 20ms intervals.
// It stops when either all payloads are exhausted or <-stop is signalled.
//
// payloads must be PCMU-encoded (one byte per sample, 160 bytes per frame).
// stats.PacketsSent is incremented for each successfully sent packet.
func Stream(sess *Session, payloads [][]byte, pt uint8, stats *SendStats, stop <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for _, payload := range payloads {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}

		seq, ts := sess.NextSeqTS(160)

		pkt := &pionrtp.Packet{
			Header: pionrtp.Header{
				Version:        2,
				PayloadType:    pt,
				SequenceNumber: seq,
				Timestamp:      ts,
				SSRC:           sess.SSRC,
			},
			Payload: payload,
		}

		raw, err := pkt.Marshal()
		if err != nil {
			continue
		}

		if err := sess.Send(raw); err == nil {
			stats.PacketsSent++
		}
	}
}
