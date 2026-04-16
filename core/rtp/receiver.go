package rtp

import (
	"net"
	"time"

	pionrtp "github.com/pion/rtp"
)

// Receive reads incoming RTP packets from conn, updates stats, and feeds
// payloads into recorder.  It returns when <-stop is signalled.
//
// conn must have a read deadline set / reset periodically so the loop can
// observe stop signals even when no packets arrive.
func Receive(conn *net.UDPConn, stats *RTPStats, recorder *AudioRecorder, stop <-chan struct{}) {
	buf := make([]byte, 1500)

	for {
		select {
		case <-stop:
			return
		default:
		}

		// 1-second read deadline so we can poll the stop channel.
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))

		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			// deadline exceeded or transient error — loop to check stop
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			continue // other errors: log in production, ignore here
		}

		var pkt pionrtp.Packet
		if err := pkt.Unmarshal(buf[:n]); err != nil {
			continue // malformed packet
		}

		arrival := time.Now()
		stats.update(pkt.SequenceNumber, arrival)

		if recorder != nil && len(pkt.Payload) > 0 {
			recorder.Write(pkt.Payload)
		}
	}
}
