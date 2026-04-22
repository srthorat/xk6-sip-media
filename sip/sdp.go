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
	clockRate := 8000
	switch payloadType {
	case 8:
		codecName = "PCMA"
	case 9:
		codecName = "G722"
	case 18:
		codecName = "G729"
	case 111:
		codecName = "OPUS"
		clockRate = 48000
	}

	return fmt.Sprintf(
		"v=0\r\n"+
			"o=k6load 0 0 IN IP4 %s\r\n"+
			"s=xk6-sip-media load test\r\n"+
			"c=IN IP4 %s\r\n"+
			"t=0 0\r\n"+
			"m=audio %d RTP/AVP %d\r\n"+
			"a=rtpmap:%d %s/%d\r\n"+
			"a=ptime:20\r\n"+
			"a=%s\r\n",
		localIP, localIP,
		rtpPort, payloadType,
		payloadType, codecName, clockRate,
		direction,
	)
}

// ParseSDP extracts the remote RTP IP, port, and dynamic codec mapping from an SDP answer.
// Returns an empty IP/0 port if parsing critically fails.
func ParseSDP(body string) (ip string, port int, ptMap map[uint8]string) {
	// Normalise line endings
	body = strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(body, "\n")

	var connectionIP string
	ptMap = make(map[uint8]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// c= line: "c=IN IP4 192.168.1.1"
		if strings.HasPrefix(line, "c=IN IP4 ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				connectionIP = parts[2]
			}
		}

		// m=audio line: "m=audio 5004 RTP/AVP 0 101"
		if strings.HasPrefix(line, "m=audio ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				p, err := strconv.Atoi(parts[1])
				if err == nil {
					port = p
				}
			}
		}

		// a=rtpmap:111 opus/48000/2
		if strings.HasPrefix(line, "a=rtpmap:") {
			parts := strings.SplitN(line[9:], " ", 2)
			if len(parts) == 2 {
				pt, err := strconv.Atoi(parts[0])
				if err == nil {
					codecParts := strings.SplitN(parts[1], "/", 2)
					ptMap[uint8(pt)] = strings.ToUpper(codecParts[0])
				}
			}
		}
	}

	// Statically inject PCMU and PCMA defaults if missing from remote answer.
	// PT 0 (PCMU) and PT 8 (PCMA) are statically assigned by RFC 3551 §6.
	// PT 9 (G722) and PT 18 (G729) are also statically assigned but should
	// only be injected if the remote advertised them in the SDP; injecting
	// them unconditionally causes codec mismatches with peers that don't support them.
	if _, ok := ptMap[0]; !ok {
		ptMap[0] = "PCMU"
	}
	if _, ok := ptMap[8]; !ok {
		ptMap[8] = "PCMA"
	}

	ip = connectionIP
	return
}
