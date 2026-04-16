package k6ext

import (
	sipcall "xk6-sip-media/sip"
)

// K6CallHandle wraps a *sip.CallHandle and exposes its methods to the k6
// JavaScript runtime. All methods are safe to call from the VU goroutine.
type K6CallHandle struct {
	handle *sipcall.CallHandle
}

// Hold puts the call on hold (re-INVITE with a=inactive SDP).
func (k *K6CallHandle) Hold() error { return k.handle.Hold() }

// Unhold resumes a held call (re-INVITE with a=sendrecv SDP).
func (k *K6CallHandle) Unhold() error { return k.handle.Unhold() }

// SendDTMF sends a single DTMF digit via RFC 2833.
func (k *K6CallHandle) SendDTMF(digit string) { k.handle.SendDTMF(digit) }

// SendInfo sends a SIP INFO request with the given body and content type.
// Used for Cisco/Avaya style SIP INFO DTMF and application signalling.
func (k *K6CallHandle) SendInfo(body, contentType string) error {
	return k.handle.SendInfo(body, contentType)
}

// SendDTMFInfo sends DTMF via SIP INFO (application/dtmf-relay).
// Supported by Cisco, Avaya, and other legacy PBX systems.
//
//	call.sendDTMFInfo("5", 160)  // digit, duration ms
func (k *K6CallHandle) SendDTMFInfo(digit string, durationMs int) error {
	return k.handle.SendDTMFInfo(digit, durationMs)
}

// BlindTransfer sends REFER to the remote party to transfer the call to targetURI.
func (k *K6CallHandle) BlindTransfer(targetURI string) error {
	return k.handle.BlindTransfer(targetURI)
}

// AttendedTransfer performs an attended transfer.
func (k *K6CallHandle) AttendedTransfer(other *K6CallHandle) error {
	return k.handle.AttendedTransfer(other.handle)
}

// Hangup sends BYE and ends the call.
func (k *K6CallHandle) Hangup() error { return k.handle.Hangup() }

// WaitDone blocks until the call ends.
func (k *K6CallHandle) WaitDone() { k.handle.WaitDone() }

// IsActive returns true if the call is still connected.
func (k *K6CallHandle) IsActive() bool { return k.handle.IsActive() }

// ── Variable Extraction (SIPp <ereg> parity) ─────────────────────────────────

// ResponseHeader returns a SIP header value from the INVITE 200 OK.
//
//	const token = call.responseHeader("X-Session-Id");
func (k *K6CallHandle) ResponseHeader(name string) string {
	return k.handle.ResponseHeader(name)
}

// CallID returns the SIP Call-ID of this dialog.
func (k *K6CallHandle) CallID() string { return k.handle.CallID() }

// ToTag returns the To tag from the 200 OK.
func (k *K6CallHandle) ToTag() string { return k.handle.ToTag() }

// RemoteContactURI returns the Contact URI from the 200 OK.
func (k *K6CallHandle) RemoteContactURI() string { return k.handle.RemoteContactURI() }

// ResponseBody returns the raw SDP body of the 200 OK.
func (k *K6CallHandle) ResponseBody() string { return k.handle.ResponseBody() }

// RequestHeader returns a header value from our own INVITE request.
func (k *K6CallHandle) RequestHeader(name string) string { return k.handle.RequestHeader(name) }

// Result returns call quality metrics. Blocks until the call is done.
func (k *K6CallHandle) Result() map[string]interface{} {
	r := k.handle.Result()
	return map[string]interface{}{
		"success":     true,
		"sent":        r.PacketsSent,
		"received":    r.PacketsReceived,
		"lost":        r.PacketsLost,
		"jitter":      r.Jitter,
		"mos":         r.MOS,
		"pesq_mos":    r.PESQScore,
		"ivr_ok":      r.IVRValid,
		"transfer_ok": r.TransferOK,
	}
}
