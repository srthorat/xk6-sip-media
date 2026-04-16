// Package sip is the top-level call orchestrator. It wires SIP signaling,
// RTP streaming, DTMF, media recording, and quality analysis into a single
// StartCall() function callable from the k6 binding.
package sip

import (
	"time"

	corertp "xk6-sip-media/core/rtp"
)

// CallConfig holds all parameters for a single SIP call.
type CallConfig struct {
	// Target is the SIP URI to call, e.g. "sip:ivr@192.168.1.1".
	Target string

	// AudioFile is the path to the WAV file to stream (8kHz mono PCM16).
	AudioFile string

	// Codec is the audio codec name: "PCMU" (default) or "PCMA".
	Codec string

	// Direction is the SDP media direction attribute (RFC 3264).
	// Defaults to DirSendRecv ("sendrecv"). Use DirInactive for hold.
	Direction string

	// Duration caps the call. 0 = no auto-hangup (call Hangup() manually).
	Duration time.Duration

	// DTMFSequence is an ordered list of DTMF digits sent after connection.
	// Each digit waits 2 seconds before being sent (configurable in Dial).
	DTMFSequence []string

	// LocalIP overrides the auto-detected outbound IP address.
	LocalIP string

	// RTPPort is the local UDP port for RTP. 0 = random in 20000–40000.
	RTPPort int

	// EnablePESQ runs PESQ scoring after the call (requires pesq binary).
	EnablePESQ bool

	// Username / Password for Digest Auth (401) challenges.
	Username string
	Password string

	// Transport selects the SIP signaling transport.
	// Valid values: "udp" (default), "tcp", "tls".
	// Use "tls" for encrypted SIPS signaling (port 5061).
	Transport string

	// TLSConfig holds TLS certificate and CA parameters.
	// Required when Transport == "tls" and mutual TLS is needed.
	// If nil with Transport == "tls", InsecureSkipVerify is used (load-test default).
	TLSConfig *TLSConfig

	// SIPPort overrides the remote SIP port.
	// 0 = use transport default (UDP/TCP: 5060, TLS: 5061).
	SIPPort int

	// AudioMode selects how media is played:
	//   ""       — stream AudioFile (default)
	//   "echo"   — reflect received RTP back to sender
	//   "pcap"   — replay the PCAPFile byte-for-byte (codec-agnostic)
	//   "silent" — send silence (comfort noise)
	AudioMode string

	// PCAPFile is the PCAP file to replay when AudioMode == "pcap".
	PCAPFile string

	// IPv6 forces IPv6 local address auto-detection when LocalIP is empty or "::"
	IPv6 bool

	// RetransmitConfig controls SIP signaling retransmission behaviour.
	// Nil = use sipgo defaults.
	RetransmitConfig *RetransmitConfig

	// CustomHeaders is a map of extra SIP headers injected into the INVITE.
	// Example: {"X-Tenant-ID": "acme", "P-Preferred-Identity": "sip:alice@acme.com"}
	CustomHeaders map[string]string

	// SRTP enables encrypted media (RTP/SAVP). xk6 generates the keying material
	// and advertises it in SDP a=crypto; both legs are then encrypted.
	// Pair with Transport=="tls" for fully-secured signaling + media (SIPS+SRTP).
	SRTP bool

	// RTCP enables RTCP sender/receiver reports on port rtpPort+1.
	// Provides standard quality metrics (RTT, fraction lost, jitter) compatible
	// with any RTCP-capable SBC or media server.
	RTCP bool

	// EarlyMedia enables playback of audio during the 183 Session Progress phase
	// (before the call is answered). When true, the RTP sender starts streaming
	// toward the provisional SDP remote address as soon as 183 arrives.
	EarlyMedia bool

	// CancelAfter, if > 0, cancels the call exactly this duration after the INVITE
	// starts. If the call is in the ringing phase (e.g., 180), this natively
	// triggers a SIP CANCEL request instead of a BYE.
	CancelAfter time.Duration
}

// StartCall executes a complete SIP call: INVITE → RTP stream → BYE.
// It is a blocking wrapper around Dial() + WaitDone() + Result().
// Each k6 VU can call it concurrently — there is no shared mutable state.
//
// For mid-call operations (hold, transfer, conference) use Dial() instead.
func StartCall(cfg CallConfig) (corertp.CallResult, error) {
	handle, err := Dial(cfg)
	if err != nil {
		return corertp.CallResult{}, err
	}
	handle.WaitDone()
	return handle.Result(), nil
}

// loopPayloads repeats payloads enough times to fill the given duration.
func loopPayloads(payloads [][]byte, dur time.Duration) [][]byte {
	if len(payloads) == 0 {
		return nil
	}
	if dur == 0 {
		return payloads // no duration cap: play once
	}
	const frameMs = 20 * time.Millisecond
	needed := int(dur/frameMs) + 10
	if needed <= len(payloads) {
		return payloads[:needed]
	}
	out := make([][]byte, 0, needed)
	for len(out) < needed {
		out = append(out, payloads...)
	}
	return out[:needed]
}
