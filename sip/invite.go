package sip

import (
	"context"
	"fmt"

	"github.com/emiago/sipgo"
	sipmsg "github.com/emiago/sipgo/sip"
)

// InviteOptions controls the SIP identity used on the initial INVITE.
type InviteOptions struct {
	LocalIP     string
	AOR         string
	Username    string
	Password    string
	DisplayName string
	Transport   string
}

// INVITEResult holds the important output of a successful INVITE exchange.
type INVITEResult struct {
	Dialog     *sipgo.DialogClientSession
	RemoteIP   string
	RemotePort int
	PtMap      map[uint8]string
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
	inviteOpts InviteOptions,
	extraHeaders ...sipmsg.Header,
) (*INVITEResult, error) {
	req, err := buildInviteRequest(toURI, sdpBody, inviteOpts, extraHeaders...)
	if err != nil {
		return nil, err
	}

	dialog, err := cache.WriteInvite(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("sip invite: %w", err)
	}

	// WaitAnswer blocks until a final response arrives.
	// The response is stored in dialog.InviteResponse on success.
	if err := dialog.WaitAnswer(ctx, sipgo.AnswerOptions{
		Username: inviteOpts.Username,
		Password: inviteOpts.Password,
	}); err != nil {
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
	remoteIP, remotePort, ptMap := ParseSDP(string(resp.Body()))
	if remoteIP == "" || remotePort == 0 {
		return nil, fmt.Errorf("sip invite: could not parse remote RTP address from SDP answer:\n%s", resp.Body())
	}

	return &INVITEResult{
		Dialog:     dialog,
		RemoteIP:   remoteIP,
		RemotePort: remotePort,
		PtMap:      ptMap,
	}, nil
}

func buildInviteRequest(
	toURI sipmsg.Uri,
	sdpBody string,
	inviteOpts InviteOptions,
	extraHeaders ...sipmsg.Header,
) (*sipmsg.Request, error) {
	req := sipmsg.NewRequest(sipmsg.INVITE, toURI)
	req.SetBody([]byte(sdpBody))

	fromURI, contactUser, err := resolveInviteIdentity(toURI, inviteOpts)
	if err != nil {
		return nil, err
	}

	to := sipmsg.ToHeader{
		Address: sipmsg.Uri{
			Scheme:    toURI.Scheme,
			User:      toURI.User,
			Host:      toURI.Host,
			Port:      toURI.Port,
			UriParams: sipmsg.NewParams(),
			Headers:   sipmsg.NewParams(),
		},
		Params: sipmsg.NewParams(),
	}
	req.AppendHeader(&to)

	if fromURI.User != "" {
		from := sipmsg.FromHeader{
			DisplayName: inviteOpts.DisplayName,
			Address: sipmsg.Uri{
				Scheme:    fromURI.Scheme,
				User:      fromURI.User,
				Host:      fromURI.Host,
				Port:      fromURI.Port,
				UriParams: sipmsg.NewParams(),
				Headers:   sipmsg.NewParams(),
			},
			Params: sipmsg.NewParams(),
		}
		from.Params.Add("tag", sipmsg.GenerateTagN(16))
		req.AppendHeader(&from)
	}

	contactParams := sipmsg.NewParams()
	contactParams.Add("ob", "")
	contact := sipmsg.ContactHeader{
		DisplayName: inviteOpts.DisplayName,
		Address: sipmsg.Uri{
			Scheme:    contactScheme(fromURI, inviteOpts.Transport),
			User:      contactUser,
			Host:      inviteOpts.LocalIP,
			UriParams: contactParams,
			Headers:   sipmsg.NewParams(),
		},
	}
	req.AppendHeader(&contact)

	appendHeaderIfMissing(req, "Allow", "PRACK, INVITE, ACK, BYE, CANCEL, UPDATE, INFO, SUBSCRIBE, NOTIFY, REFER, MESSAGE, OPTIONS")
	appendHeaderIfMissing(req, "Supported", "replaces, 100rel, norefersub")
	appendHeaderIfMissing(req, "User-Agent", "xk6-sip-media/1.0")

	for _, hdr := range extraHeaders {
		req.AppendHeader(hdr)
	}

	return req, nil
}

func resolveInviteIdentity(toURI sipmsg.Uri, inviteOpts InviteOptions) (sipmsg.Uri, string, error) {
	if inviteOpts.AOR != "" {
		var aorURI sipmsg.Uri
		if err := sipmsg.ParseUri(inviteOpts.AOR, &aorURI); err != nil {
			return sipmsg.Uri{}, "", fmt.Errorf("sip invite: parse AOR %q: %w", inviteOpts.AOR, err)
		}
		return sipmsg.Uri{
			Scheme:    aorURI.Scheme,
			User:      aorURI.User,
			Host:      aorURI.Host,
			Port:      aorURI.Port,
			UriParams: sipmsg.NewParams(),
			Headers:   sipmsg.NewParams(),
		}, aorURI.User, nil
	}

	if inviteOpts.Username == "" {
		return sipmsg.Uri{}, "k6load", nil
	}

	scheme := toURI.Scheme
	if scheme == "" {
		scheme = "sip"
	}

	return sipmsg.Uri{
		Scheme:    scheme,
		User:      inviteOpts.Username,
		Host:      toURI.Host,
		UriParams: sipmsg.NewParams(),
		Headers:   sipmsg.NewParams(),
	}, inviteOpts.Username, nil
}

func contactScheme(fromURI sipmsg.Uri, transport string) string {
	if transport == TransportTLS {
		return "sips"
	}
	if fromURI.Scheme != "" {
		return fromURI.Scheme
	}
	return "sip"
}

func appendHeaderIfMissing(req *sipmsg.Request, name, value string) {
	if req.GetHeader(name) != nil {
		return
	}
	req.AppendHeader(sipmsg.NewHeader(name, value))
}
