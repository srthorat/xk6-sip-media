package sip

import (
	"context"
	"fmt"

	"github.com/emiago/sipgo"
	sipmsg "github.com/emiago/sipgo/sip"

	corertp "xk6-sip-media/core/rtp"
)

// EarlyMedia holds a provisional media stream established on 183 Session Progress.
type EarlyMedia struct {
	RemoteIP   string
	RemotePort int
	SRTPConfig *corertp.SRTPConfig // set if 183 SDP advertises SRTP
}

// SendINVITEWithEarlyMedia extends SendINVITE to intercept 183 Session Progress
// responses. When 183 arrives with an SDP body, EarlyMedia is populated so
// the caller can begin streaming RTP toward the provisional remote address
// before the call is answered (200 OK).
func SendINVITEWithEarlyMedia(
	ctx context.Context,
	cache *sipgo.DialogClientCache,
	toURI sipmsg.Uri,
	sdpBody string,
	inviteOpts InviteOptions,
	extraHeaders ...sipmsg.Header,
) (*INVITEResult, *EarlyMedia, error) {
	req, err := buildInviteRequest(toURI, sdpBody, inviteOpts, extraHeaders...)
	if err != nil {
		return nil, nil, err
	}

	dialog, err := cache.WriteInvite(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("sip invite: %w", err)
	}

	var early *EarlyMedia

	// AnswerOptions.OnResponse is called for every response including 1xx.
	// We intercept 183 here to capture the provisional SDP.
	answerOpts := sipgo.AnswerOptions{
		Username: inviteOpts.Username,
		Password: inviteOpts.Password,
		OnResponse: func(resp *sipmsg.Response) error {
			if resp.StatusCode == 183 && len(resp.Body()) > 0 {
				ip, port, _ := ParseSDP(string(resp.Body()))
				if ip != "" && port != 0 {
					em := &EarlyMedia{
						RemoteIP:   ip,
						RemotePort: port,
					}
					// Parse SRTP config if encoded in 183
					if inlineKey := ParseSDPCrypto(string(resp.Body())); inlineKey != "" {
						cfg, e := corertp.ParseSRTPConfig(inlineKey)
						if e == nil {
							em.SRTPConfig = cfg
						}
					}
					early = em
				}
			}
			return nil // returning non-nil cancels WaitAnswer
		},
	}

	if err := dialog.WaitAnswer(ctx, answerOpts); err != nil {
		_ = dialog.Close()
		return nil, nil, fmt.Errorf("sip invite wait: %w", err)
	}

	resp := dialog.InviteResponse
	if resp == nil || resp.StatusCode != 200 {
		code := 0
		if resp != nil {
			code = int(resp.StatusCode)
		}
		_ = dialog.Close()
		return nil, nil, fmt.Errorf("sip invite: unexpected status %d", code)
	}

	if err := dialog.Ack(ctx); err != nil {
		return nil, nil, fmt.Errorf("sip ack: %w", err)
	}

	remoteIP, remotePort, ptMap := ParseSDP(string(resp.Body()))
	if remoteIP == "" || remotePort == 0 {
		return nil, nil, fmt.Errorf("sip invite: could not parse remote RTP address from 200 OK SDP")
	}

	return &INVITEResult{
		Dialog:     dialog,
		RemoteIP:   remoteIP,
		RemotePort: remotePort,
		PtMap:      ptMap,
	}, early, nil
}
