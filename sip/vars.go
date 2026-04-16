package sip

// ResponseVars documents the variable-extraction pattern available via CallHandle methods.
// Use ResponseHeader(), CallID(), FromTag(), ToTag(), etc. on a *CallHandle directly.

// ResponseHeader returns the value of a SIP header from the INVITE 200 OK response.
// Returns "" if the call has not been answered or the header is absent.
//
//	const token = call.responseHeader("X-IVR-Session");
func (h *CallHandle) ResponseHeader(name string) string {
	if h.dialog == nil || h.dialog.InviteResponse == nil {
		return ""
	}
	hdr := h.dialog.InviteResponse.GetHeader(name)
	if hdr == nil {
		return ""
	}
	return hdr.Value()
}

// CallID returns the SIP Call-ID of this dialog.
func (h *CallHandle) CallID() string {
	if h.dialog == nil || h.dialog.InviteRequest == nil {
		return ""
	}
	if cid := h.dialog.InviteRequest.CallID(); cid != nil {
		return cid.Value()
	}
	return ""
}

// FromTag returns the From tag of the dialog (our tag, generated on INVITE).
func (h *CallHandle) FromTag() string {
	if h.dialog == nil || h.dialog.InviteRequest == nil {
		return ""
	}
	if from := h.dialog.InviteRequest.From(); from != nil {
		tag, _ := from.Params.Get("tag")
		return tag
	}
	return ""
}

// ToTag returns the To tag assigned by the remote party in the 200 OK.
func (h *CallHandle) ToTag() string {
	if h.dialog == nil || h.dialog.InviteResponse == nil {
		return ""
	}
	if to := h.dialog.InviteResponse.To(); to != nil {
		tag, _ := to.Params.Get("tag")
		return tag
	}
	return ""
}

// RemoteContact returns the Contact URI string from the 200 OK response.
// Useful for building subsequent requests to the exact remote endpoint.
func (h *CallHandle) RemoteContactURI() string {
	uri := h.remoteContact()
	return uri.String()
}

// ResponseBody returns the raw body of the INVITE 200 OK (the remote SDP answer).
func (h *CallHandle) ResponseBody() string {
	if h.dialog == nil || h.dialog.InviteResponse == nil {
		return ""
	}
	return string(h.dialog.InviteResponse.Body())
}

// RequestHeader returns a header value from our own INVITE request.
func (h *CallHandle) RequestHeader(name string) string {
	if h.dialog == nil || h.dialog.InviteRequest == nil {
		return ""
	}
	hdr := h.dialog.InviteRequest.GetHeader(name)
	if hdr == nil {
		return ""
	}
	return hdr.Value()
}
