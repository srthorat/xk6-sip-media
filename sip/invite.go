package sip

import (
	"context"
	"fmt"

	"github.com/emiago/sipgo"
	sipmsg "github.com/emiago/sipgo/sip"
)

// INVITEResult holds the important output of a successful INVITE exchange.
type INVITEResult struct {
	Dialog     *sipgo.DialogClientSession
	RemoteIP   string
	RemotePort int
}

// SendINVITE sends a SIP INVITE request via the DialogClientCache, waits for
// 200 OK, sends ACK, and returns the parsed remote RTP address.
//
// extraHeaders are appended to the INVITE (e.g. Content-Type, custom headers).
func SendINVITE(
	ctx context.Context,
	cache *sipgo.DialogClientCache,
	toURI sipmsg.Uri,
	sdpBody string,
	extraHeaders ...sipmsg.Header,
) (*INVITEResult, error) {

	// Pass the first header as the required header arg; rest via variadic
	var firstHdr sipmsg.Header
	var rest []sipmsg.Header
	if len(extraHeaders) > 0 {
		firstHdr = extraHeaders[0]
		rest = extraHeaders[1:]
	}
	_ = rest // sipgo Invite() takes one header; extras added below

	dialog, err := cache.Invite(ctx, toURI, []byte(sdpBody), firstHdr)
	if err != nil {
		return nil, fmt.Errorf("sip invite: %w", err)
	}

	// WaitAnswer blocks until a final response arrives.
	// The response is stored in dialog.InviteResponse on success.
	if err := dialog.WaitAnswer(ctx, sipgo.AnswerOptions{}); err != nil {
		_ = dialog.Close()
		return nil, fmt.Errorf("sip invite wait: %w", err)
	}

	// At this point dialog.InviteResponse is the 200 OK.
	resp := dialog.InviteResponse
	if resp == nil || resp.StatusCode != 200 {
		code := 0
		if resp != nil {
			code = int(resp.StatusCode)
		}
		_ = dialog.Close()
		return nil, fmt.Errorf("sip invite: unexpected status %d", code)
	}

	// ACK the 200 OK
	if err := dialog.Ack(ctx); err != nil {
		return nil, fmt.Errorf("sip ack: %w", err)
	}

	// Parse remote SDP from response body
	remoteIP, remotePort := ParseSDP(string(resp.Body()))
	if remoteIP == "" || remotePort == 0 {
		return nil, fmt.Errorf("sip invite: could not parse remote RTP address from SDP answer:\n%s", resp.Body())
	}

	return &INVITEResult{
		Dialog:     dialog,
		RemoteIP:   remoteIP,
		RemotePort: remotePort,
	}, nil
}
