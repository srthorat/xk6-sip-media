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
//
// silenceSize is the byte length of one encoded 20ms payload for the active
// codec, used to synthesize PLC silence on packet loss (0 = skip PLC writes).
func Receive(conn *net.UDPConn, stats *RTPStats, recorder *AudioRecorder, silenceSize int, stop <-chan struct{}) {
	buf := make([]byte, 1500)

	var jb *JitterBuffer
	if recorder != nil {
		jb = NewJitterBuffer(recorder, 40*time.Millisecond, silenceSize)
		defer jb.Close()
	}

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
			// Non-timeout error (e.g. EMSGSIZE, ECONNREFUSED): count and continue.
			stats.RecvErrors.Add(1)
			continue
		}

		var pkt pionrtp.Packet
		if err := pkt.Unmarshal(buf[:n]); err != nil {
			continue // malformed packet
		}

		arrival := time.Now()
		stats.update(pkt.SequenceNumber, arrival)

		if jb != nil && len(pkt.Payload) > 0 {
			// Push into priority queue, decoupling from synchronous IO
			jb.Push(&pkt)
		}
	}
}
