package sip

import (
	"context"
	"fmt"
	"time"

	sipmsg "github.com/emiago/sipgo/sip"
)

// Hold puts the call on hold by sending a re-INVITE with a=inactive SDP.
// The remote party stops sending media; we also stop sending.
func (h *CallHandle) Hold() error {
	h.mu.Lock()
	if !h.active {
		h.mu.Unlock()
		return fmt.Errorf("hold: call is not active")
	}
	if h.onHold {
		h.mu.Unlock()
		return nil // already on hold
	}
	h.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.sendReINVITE(ctx, DirInactive); err != nil {
		return fmt.Errorf("hold: %w", err)
	}

	h.mu.Lock()
	h.onHold = true
	h.mu.Unlock()
	return nil
}

// Unhold resumes a held call by sending a re-INVITE with a=sendrecv SDP.
func (h *CallHandle) Unhold() error {
	h.mu.Lock()
	if !h.active {
		h.mu.Unlock()
		return fmt.Errorf("unhold: call is not active")
	}
	if !h.onHold {
		h.mu.Unlock()
		return nil // not on hold
	}
	h.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.sendReINVITE(ctx, DirSendRecv); err != nil {
		return fmt.Errorf("unhold: %w", err)
	}

	h.mu.Lock()
	h.onHold = false
	h.mu.Unlock()
	return nil
}

// sendReINVITE sends an in-dialog re-INVITE with the given SDP direction,
// waits for 200 OK, and sends the corresponding ACK.
//
// Re-INVITE is used for call hold (inactive), unhold (sendrecv), and
// mid-call codec changes.
func (h *CallHandle) sendReINVITE(ctx context.Context, direction string) error {
	sdp := BuildSDPWithDirection(h.localIP, h.rtpPort, h.cod.PayloadType(), direction)

	// Build re-INVITE — dialog.Do() fills in From/To/Call-ID/CSeq
	reinvite := sipmsg.NewRequest(sipmsg.INVITE, h.remoteContact())
	reinvite.SetBody([]byte(sdp))
	reinvite.AppendHeader(sipmsg.NewHeader("Content-Type", "application/sdp"))

	// Do() handles CSeq increment within the dialog and waits for final response
	resp, err := h.dialog.Do(ctx, reinvite)
	if err != nil {
		return fmt.Errorf("re-INVITE: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("re-INVITE: remote returned %d", resp.StatusCode)
	}

	// ACK the re-INVITE 200 OK manually (sipgo's Ack() is for initial INVITE only)
	ack := buildReINVITEAck(reinvite, resp)
	return h.dialog.WriteRequest(ack)
}

// buildReINVITEAck constructs the ACK for a re-INVITE 200 OK.
// The ACK for in-dialog re-INVITE uses the same CSeq number as the re-INVITE
// but with method ACK (RFC 3261 §17.1.1.3).
func buildReINVITEAck(reinvite *sipmsg.Request, resp *sipmsg.Response) *sipmsg.Request {
	// Target: Contact from 200 OK (or fall back to re-INVITE recipient)
	recipient := reinvite.Recipient
	if c := resp.Contact(); c != nil {
		recipient = c.Address
	}

	ack := sipmsg.NewRequest(sipmsg.ACK, *recipient.Clone())
	ack.SipVersion = reinvite.SipVersion

	// Copy Route from re-INVITE
	sipmsg.CopyHeaders("Route", reinvite, ack)

	if h := reinvite.From(); h != nil {
		ack.AppendHeader(sipmsg.HeaderClone(h))
	}
	if h := resp.To(); h != nil {
		ack.AppendHeader(sipmsg.HeaderClone(h))
	}
	if h := reinvite.CallID(); h != nil {
		ack.AppendHeader(sipmsg.HeaderClone(h))
	}

	// CSeq: same number, method = ACK
	if cseq := reinvite.CSeq(); cseq != nil {
		c := *cseq
		c.MethodName = sipmsg.ACK
		ack.AppendHeader(&c)
	}

	maxFwd := sipmsg.MaxForwardsHeader(70)
	ack.AppendHeader(&maxFwd)
	return ack
}
