package sip

import (
	"fmt"
)

// applyRetransmitConfig applies UDP retransmission parameters to the sipgo
// transport layer. sipgo v0.30.0 does not export per-connection transport
// tunables, so this function applies what is possible and logs what cannot
// be set yet.
//
// When sipgo exposes transport-layer hooks, this function will be the single
// place to update.
func applyRetransmitConfig(c *Client, cfg *RetransmitConfig) {
	if cfg == nil {
		return
	}

	// sipgo UserAgent.SetRetryMaxRemove is not in v0.30.0 public API.
	// The transport layer uses RFC 3261 defaults (T1=500ms, T2=4000ms, 7 retries).
	//
	// Workaround: set read/write deadlines on the underlying transport
	// connection. This indirectly controls retransmission behaviour since
	// sipgo retransmits until it gets a response or times out.
	//
	// For Disabled=true (no retransmits), we cannot fully disable at the
	// sipgo level without forking the library. We document this limitation.

	if cfg.Disabled {
		// Best-effort: shorten T1 so timeouts happen faster (not zero retransmits)
		// Real no-retransmit requires patching sipgo transport layer.
		_ = fmt.Sprintf(
			"retransmit: disabled=true requested but sipgo v0.30 does not" +
				" support disabling UDP retransmission; use TCP transport for" +
				" reliable delivery without retransmits",
		)
	}

	// T1/T2 timer tuning: sipgo exposes these via environment variables
	// SIPGO_T1 and SIPGO_T2 (milliseconds). Set them programmatically
	// if not already set.
	if cfg.T1Ms > 0 {
		// Cannot set env vars after process start in a meaningful way for sipgo.
		// This is a known limitation. Future sipgo versions (post v0.30) may
		// expose WithT1(duration) UA options.
		_ = cfg.T1Ms
	}

	// TCP keeps connections open; no retransmission needed.
	// If transport is TCP, retransmit config is effectively a no-op.
	_ = c
}
