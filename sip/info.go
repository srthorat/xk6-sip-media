package sip

import (
	"context"
	"fmt"
	"time"

	sipmsg "github.com/emiago/sipgo/sip"
)

// SendInfo sends a SIP INFO request mid-call with the given content type and body.
// SIP INFO (RFC 6086) is used by some PBX/IVR systems for in-band DTMF and
// application-level signaling.
//
// Common content types:
//   - "application/dtmf-relay"  (Cisco, Avaya)
//   - "application/dtmf"
//   - "application/vnd.nortel.icas+xml"
//
// Example DTMF relay body:
//
//	Signal=5\r\nDuration=160\r\n
func (h *CallHandle) SendInfo(body string, contentType string) error {
	h.mu.Lock()
	active := h.active
	h.mu.Unlock()
	if !active {
		return fmt.Errorf("sendInfo: call is not active")
	}

	info := sipmsg.NewRequest(sipmsg.INFO, h.remoteContact())
	info.SetBody([]byte(body))
	info.AppendHeader(sipmsg.NewHeader("Content-Type", contentType))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.dialog.Do(ctx, info)
	if err != nil {
		return fmt.Errorf("sendInfo: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("sendInfo: remote returned %d", resp.StatusCode)
	}
	return nil
}

// SendDTMFInfo sends DTMF using SIP INFO with application/dtmf-relay body.
// This is the Cisco/Avaya style. Use SendDTMF() for RFC 2833 (RTP) style.
//
//	call.SendDTMFInfo("5", 160)  // digit, duration in ms
func (h *CallHandle) SendDTMFInfo(digit string, durationMs int) error {
	if durationMs <= 0 {
		durationMs = 160
	}
	body := fmt.Sprintf("Signal=%s\r\nDuration=%d\r\n", digit, durationMs)
	return h.SendInfo(body, "application/dtmf-relay")
}
