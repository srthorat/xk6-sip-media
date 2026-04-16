package sip

import (
	"fmt"
	"strconv"
	"strings"
)

// Direction values for SDP media stream direction attributes (RFC 3264).
const (
	DirSendRecv = "sendrecv" // default — bidirectional
	DirSendOnly = "sendonly" // we send, remote receives
	DirRecvOnly = "recvonly" // we receive, remote sends
	DirInactive = "inactive" // call hold — no media in either direction
)

// BuildSDP constructs a minimal SDP offer body for an audio-only call.
//
// Parameters:
//   - localIP:  the IP address to advertise in SDP (c= and o= lines)
//   - rtpPort:  local UDP port for RTP (m= line)
//   - payloadType: e.g. 0 for PCMU, 8 for PCMA
func BuildSDP(localIP string, rtpPort int, payloadType uint8) string {
	return BuildSDPWithDirection(localIP, rtpPort, payloadType, DirSendRecv)
}

// BuildSDPWithDirection constructs a minimal SDP offer body with a specific direction.
func BuildSDPWithDirection(localIP string, rtpPort int, payloadType uint8, direction string) string {
	codecName := "PCMU"
	if payloadType == 8 {
		codecName = "PCMA"
	}

	return fmt.Sprintf(
		"v=0\r\n"+
			"o=k6load 0 0 IN IP4 %s\r\n"+
			"s=xk6-sip-media load test\r\n"+
			"c=IN IP4 %s\r\n"+
			"t=0 0\r\n"+
			"m=audio %d RTP/AVP %d\r\n"+
			"a=rtpmap:%d %s/8000\r\n"+
			"a=ptime:20\r\n"+
			"a=%s\r\n",
		localIP, localIP,
		rtpPort, payloadType,
		payloadType, codecName,
		direction,
	)
}

// ParseSDP extracts the remote RTP IP and port from an SDP answer body.
// It handles both CRLF (\r\n) and LF (\n) line endings.
//
// Returns ("", 0) if parsing fails (caller should use fallback or error out).
func ParseSDP(body string) (ip string, port int) {
	// Normalise line endings
	body = strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(body, "\n")

	var connectionIP string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// c= line: "c=IN IP4 192.168.1.1"
		if strings.HasPrefix(line, "c=IN IP4 ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				connectionIP = parts[2]
			}
		}

		// m=audio line: "m=audio 5004 RTP/AVP 0"
		if strings.HasPrefix(line, "m=audio ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				p, err := strconv.Atoi(parts[1])
				if err == nil {
					port = p
				}
			}
		}

		// m-level c= overrides session-level c=
		// (handle simple case only — no multi-media sections)
	}

	ip = connectionIP
	return
}
