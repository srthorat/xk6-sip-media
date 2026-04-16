package sip

import (
	"context"
	"fmt"
	"net/url"
	"time"

	sipmsg "github.com/emiago/sipgo/sip"
)

// BlindTransfer sends a REFER to the remote party, instructing it to call
// targetURI directly. This is a "blind" (unattended) transfer — we do not
// wait for the transferred call to be answered before hanging up.
//
// Flow (RFC 3515):
//
//	A → B: REFER (Refer-To: C)
//	B → A: 202 Accepted           ← we stop here (option A: fire-and-forget)
//	B → C: INVITE                 (handled by remote, not us)
//	B → A: BYE                    (remote ends our leg after transfer)
func (h *CallHandle) BlindTransfer(targetURI string) error {
	h.mu.Lock()
	if !h.active {
		h.mu.Unlock()
		return fmt.Errorf("blind transfer: call is not active")
	}
	h.mu.Unlock()

	referTo := fmt.Sprintf("<%s>", targetURI)
	referredBy := fmt.Sprintf("<sip:k6load@%s>", h.localIP)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.sendREFER(ctx, referTo, referredBy); err != nil {
		return fmt.Errorf("blind transfer to %s: %w", targetURI, err)
	}

	// Mark transfer success in result (will be captured in finalize)
	h.mu.Lock()
	h.result.TransferOK = true
	h.mu.Unlock()

	return nil
}

// AttendedTransfer executes an attended (consultative) transfer.
//
// It tells leg A (h) to replace its dialog with leg B (consultant).
// After this call:
//   - h (leg A / original party) receives NOTIFY → BYE and is finished
//   - consultant (leg B) is connected to the transferred party on the bridge
//
// Flow (RFC 3515 + RFC 3891):
//
//	A → B: REFER (Refer-To: <C?Replaces=B-dialog>)
//	B → A: 202 Accepted
//	B → C: INVITE (Replaces: B-dialog)     (handled by remote)
//	C → B: 200 OK
//	A → C: BYE                             (we end our leg with C)
//	B → A: BYE                             (remote ends our leg with B)
func (h *CallHandle) AttendedTransfer(consultant *CallHandle) error {
	h.mu.Lock()
	active := h.active
	h.mu.Unlock()

	consultant.mu.Lock()
	consultActive := consultant.active
	consultant.mu.Unlock()

	if !active {
		return fmt.Errorf("attended transfer: primary call is not active")
	}
	if !consultActive {
		return fmt.Errorf("attended transfer: consultant call is not active")
	}

	// Build the Replaces value from leg B's (consultant's) dialog IDs
	callID, toTag, fromTag, err := consultant.dialogID()
	if err != nil {
		return fmt.Errorf("attended transfer: get consultant dialog ID: %w", err)
	}

	// The remote (C = consultant's remote party) URI
	consultantContact := consultant.remoteContact()
	consultantURI := fmt.Sprintf("sip:%s@%s", consultantContact.User, consultantContact.Host)
	if consultantContact.Port > 0 {
		consultantURI = fmt.Sprintf("sip:%s@%s:%d",
			consultantContact.User, consultantContact.Host, consultantContact.Port)
	}

	// Replaces parameter: must be URL-encoded (RFC 3891 §3)
	replaces := url.QueryEscape(
		fmt.Sprintf("%s;to-tag=%s;from-tag=%s", callID, toTag, fromTag),
	)

	referTo := fmt.Sprintf("<%s?Replaces=%s>", consultantURI, replaces)
	referredBy := fmt.Sprintf("<sip:k6load@%s>", h.localIP)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.sendREFER(ctx, referTo, referredBy); err != nil {
		return fmt.Errorf("attended transfer: REFER: %w", err)
	}

	// Mark transfer success
	h.mu.Lock()
	h.result.TransferOK = true
	h.mu.Unlock()

	// Our leg with the consultant is no longer needed — hang it up
	// (remote will also BYE us after the Replaces INVITE succeeds)
	go func() {
		time.Sleep(1 * time.Second) // brief grace period for the transfer to complete
		_ = consultant.Hangup()
	}()

	return nil
}

// sendREFER sends an in-dialog REFER request and expects 202 Accepted.
// This is a fire-and-forget send — we do not await NOTIFY subscription events.
func (h *CallHandle) sendREFER(
	ctx context.Context,
	referTo string,
	referredBy string,
) error {
	refer := sipmsg.NewRequest(sipmsg.REFER, h.remoteContact())
	refer.AppendHeader(sipmsg.NewHeader("Refer-To", referTo))
	refer.AppendHeader(sipmsg.NewHeader("Referred-By", referredBy))
	// Suppress NOTIFY subscription — saves us from implementing a UAS listener
	refer.AppendHeader(sipmsg.NewHeader("Refer-Sub", "false"))

	// dialog.Do() handles CSeq, From/To, Call-ID routing
	resp, err := h.dialog.Do(ctx, refer)
	if err != nil {
		return fmt.Errorf("REFER: %w", err)
	}

	// 202 Accepted = async transfer is in progress (most RFC-compliant servers)
	// 200 OK = server processed synchronously
	if resp.StatusCode != 202 && resp.StatusCode != 200 {
		return fmt.Errorf("REFER: unexpected status %d", resp.StatusCode)
	}
	return nil
}
