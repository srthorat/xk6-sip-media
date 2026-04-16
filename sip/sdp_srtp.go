package sip

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	corertp "xk6-sip-media/core/rtp"
)

// SRTP cipher suites supported in SDP a=crypto (RFC 4568).
const (
	SRTPProfileAES128HMAC80 = "AES_CM_128_HMAC_SHA1_80"
	SRTPProfileAES128HMAC32 = "AES_CM_128_HMAC_SHA1_32"
)

// BuildSDPWithSRTP constructs an SDP offer that advertises SRTP (RTP/SAVP)
// with an a=crypto attribute containing a freshly-generated master key.
//
// Returns the SDP string and the local SRTPConfig (to use for outbound encryption).
//
// SDP format:
//
//	m=audio <port> RTP/SAVP <pt>
//	a=crypto:1 AES_CM_128_HMAC_SHA1_80 inline:<base64-30-bytes>
//	a=rtpmap:...
//	a=ptime:20
func BuildSDPWithSRTP(localIP string, rtpPort int, payloadType uint8, direction string) (sdp string, cfg *corertp.SRTPConfig, err error) {
	// Generate 30 random bytes: 16 (key) + 14 (salt)
	keyBytes := make([]byte, 30)
	if _, err = rand.Read(keyBytes); err != nil {
		return "", nil, fmt.Errorf("srtp: generate key: %w", err)
	}

	cfg = &corertp.SRTPConfig{
		MasterKey:  keyBytes[:16],
		MasterSalt: keyBytes[16:],
		Profile:    SRTPProfileAES128HMAC80,
	}

	inline := "inline:" + base64.StdEncoding.EncodeToString(keyBytes)

	codecName := payloadTypeName(payloadType)
	clockRate := payloadTypeClockRate(payloadType)

	sdp = fmt.Sprintf(
		"v=0\r\n"+
			"o=k6load 0 0 IN IP4 %s\r\n"+
			"s=xk6-sip-media load test\r\n"+
			"c=IN IP4 %s\r\n"+
			"t=0 0\r\n"+
			"m=audio %d RTP/SAVP %d\r\n"+
			"a=crypto:1 %s %s\r\n"+
			"a=rtpmap:%d %s/%d\r\n"+
			"a=ptime:20\r\n"+
			"a=%s\r\n",
		localIP, localIP,
		rtpPort, payloadType,
		SRTPProfileAES128HMAC80, inline,
		payloadType, codecName, clockRate,
		direction,
	)
	return sdp, cfg, nil
}

// ParseSDPCrypto extracts the first a=crypto inline key from an SDP body.
// Returns the inline: string (e.g. "inline:WVNfX...") or "" if not found.
//
// Used to extract the remote party's keying material from their SDP answer.
func ParseSDPCrypto(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "a=crypto:") {
			continue
		}
		// a=crypto:<tag> <suite> <key-params>
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}
		// parts[3] contains inline:...
		keyParam := parts[3]
		if strings.HasPrefix(keyParam, "inline:") {
			return keyParam
		}
	}
	return ""
}

// ParseSDPSRTPProfile extracts the cipher suite name from an SDP a=crypto line.
func ParseSDPSRTPProfile(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "a=crypto:") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			return parts[2] // e.g. "AES_CM_128_HMAC_SHA1_80"
		}
	}
	return ""
}

// ParseSDPMediaPort extracts the numeric port from the first m= line.
func ParseSDPMediaPort(body string) int {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "m=audio ") || strings.HasPrefix(line, "m=") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				p, err := strconv.Atoi(parts[1])
				if err == nil {
					return p
				}
			}
		}
	}
	return 0
}

// IsSRTPOffer returns true if the SDP contains RTP/SAVP in its m= line.
func IsSRTPOffer(body string) bool {
	return strings.Contains(body, "RTP/SAVP")
}

// ── helpers ───────────────────────────────────────────────────────────────

func payloadTypeName(pt uint8) string {
	switch pt {
	case 0:
		return "PCMU"
	case 8:
		return "PCMA"
	case 9:
		return "G722"
	default:
		return fmt.Sprintf("unknown%d", pt)
	}
}

func payloadTypeClockRate(pt uint8) int {
	switch pt {
	case 9: // G.722 — intentionally uses 8000 per RFC 3551 §4.5.2
		return 8000
	default:
		return 8000
	}
}
