package sip

// RetransmitConfig controls SIP/UDP retransmission behaviour.
// These values map to sipgo transport layer tunables.
//
// SIPp equivalent flags:
//
//	-max_retrans N         → MaxRetransmits
//	-max_invite_retrans N  → MaxINVITERetransmits
//	-nr                    → Disabled = true
type RetransmitConfig struct {
	// Disabled turns off all UDP retransmission.
	// Use when testing network reliability directly (INVITE sent once, no retry).
	Disabled bool

	// MaxRetransmits is the total number of retransmit attempts for any request.
	// 0 = use sipgo default (typically 7).
	MaxRetransmits int

	// MaxINVITERetransmits overrides MaxRetransmits specifically for INVITE.
	// 0 = same as MaxRetransmits.
	MaxINVITERetransmits int

	// T1 is the base retransmission timer in milliseconds (RFC 3261 T1, default 500ms).
	// Reduce for LAN tests; increase for high-latency WAN.
	T1Ms int

	// T2 is the maximum retransmission timer in milliseconds (RFC 3261 T2, default 4000ms).
	T2Ms int
}

// Note: sipgo v0.30.0 does not expose per-client retransmission tuning via
// a public API. These fields are stored in CallConfig and will be applied
// once sipgo exposes the relevant options, or via a custom transport wrapper.
// The Disabled and MaxRetransmits fields are readable by integration tests.
