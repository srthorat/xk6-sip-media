// Package rtp provides RTP echo mode: receives packets and immediately reflects
// them back to the sender. Used to test echo cancellation and loopback quality.
package rtp

import (
	"net"
	"time"

	pionrtp "github.com/pion/rtp"
)

// Echo listens for RTP packets on conn and immediately reflects each packet
// back to remoteAddr. Useful for testing echo cancellation, MOS loopback
// measurement, and validating round-trip jitter.
//
// The reflected packet has:
//   - Same SSRC as the received packet (so the remote sees its own stream)
//   - Same payload type, sequence number, and timestamp as received
//   - Same payload bytes — no decoding performed
//
// It returns when <-stop is signalled.
func Echo(conn *net.UDPConn, remoteAddr *net.UDPAddr, stats *RTPStats, stop <-chan struct{}) {
	buf := make([]byte, 1500)

	for {
		select {
		case <-stop:
			return
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		n, srcAddr, err := conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			continue
		}

		var pkt pionrtp.Packet
		if err := pkt.Unmarshal(buf[:n]); err != nil {
			continue
		}

		stats.update(pkt.SequenceNumber, time.Now())

		// Reflect the raw packet back unchanged
		target := remoteAddr
		if srcAddr != nil {
			if udpSrc, ok := srcAddr.(*net.UDPAddr); ok {
				target = udpSrc
			}
		}

		_, _ = conn.WriteTo(buf[:n], target)
	}
}
