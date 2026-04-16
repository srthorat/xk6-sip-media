package sip

import (
	"context"
	"fmt"
	"time"
)

// ThirdPartyCallConfig defines a 3PCC (RFC 3725) scenario.
//
// 3PCC (Third-Party Call Control) allows a controller to set up a call
// between two parties (A and B) without being in the media path.
//
// Flow (RFC 3725 §5 — Basic 3PCC):
//
//	Controller → A: INVITE (no SDP)
//	A → Controller: 200 OK (A's SDP offer)
//	Controller → B: INVITE (A's SDP as offer)
//	B → Controller: 200 OK (B's SDP answer)
//	Controller → A: ACK (B's SDP answer)
//	Controller → B: ACK
//	A ←──── media ────→ B
//
// xk6 implementation: two Dial() calls coordinated by the same VU goroutine.
// This is "xk6 3PCC" — not a socket-synced multi-instance approach like SIPp,
// but achieves the same outcome: the controller dials both parties.
type ThirdPartyCallConfig struct {
	// PartyA is the SIP URI of the first party to call.
	PartyA string

	// PartyB is the SIP URI of the second party to call.
	PartyB string

	// AudioA is the audio file streamed toward party A.
	AudioA string

	// AudioB is the audio file streamed toward party B.
	AudioB string

	// Codec for both legs (default PCMU).
	Codec string

	// LocalIP for both legs.
	LocalIP string

	// Duration of the 3PCC session. 0 = manual Hangup().
	Duration time.Duration

	// Transport for both legs ("udp", "tcp", "tls").
	Transport string

	// TLSConfig for TLS transport.
	TLSConfig *TLSConfig
}

// ThirdPartyCall represents an active 3PCC session with two call legs.
type ThirdPartyCall struct {
	LegA *CallHandle
	LegB *CallHandle
}

// HangupAll terminates both legs.
func (t *ThirdPartyCall) HangupAll() error {
	errA := t.LegA.Hangup()
	errB := t.LegB.Hangup()
	if errA != nil {
		return errA
	}
	return errB
}

// WaitDone blocks until both legs have ended.
func (t *ThirdPartyCall) WaitDone() {
	t.LegA.WaitDone()
	t.LegB.WaitDone()
}

// Dial3PCC dials both parties and returns a ThirdPartyCall handle.
// Both legs are established before this function returns.
//
// The controller (xk6 VU) is in the media path during setup only;
// once both Dial() calls succeed the two parties exchange media directly
// via the RTP addresses in their respective SDP answers.
func Dial3PCC(cfg ThirdPartyCallConfig) (*ThirdPartyCall, error) {
	if cfg.Codec == "" {
		cfg.Codec = "PCMU"
	}

	// Dial leg A
	legACfg := CallConfig{
		Target:    cfg.PartyA,
		AudioFile: cfg.AudioA,
		Codec:     cfg.Codec,
		LocalIP:   cfg.LocalIP,
		Duration:  cfg.Duration,
		Transport: cfg.Transport,
		TLSConfig: cfg.TLSConfig,
	}
	legA, err := Dial(legACfg)
	if err != nil {
		return nil, fmt.Errorf("3pcc: dial leg A (%s): %w", cfg.PartyA, err)
	}

	// Dial leg B
	legBCfg := CallConfig{
		Target:    cfg.PartyB,
		AudioFile: cfg.AudioB,
		Codec:     cfg.Codec,
		LocalIP:   cfg.LocalIP,
		Duration:  cfg.Duration,
		Transport: cfg.Transport,
		TLSConfig: cfg.TLSConfig,
	}
	legB, err := Dial(legBCfg)
	if err != nil {
		_ = legA.Hangup()
		return nil, fmt.Errorf("3pcc: dial leg B (%s): %w", cfg.PartyB, err)
	}

	return &ThirdPartyCall{LegA: legA, LegB: legB}, nil
}

// Dial3PCCWithTimeout wraps Dial3PCC with a context deadline.
func Dial3PCCWithTimeout(ctx context.Context, cfg ThirdPartyCallConfig) (*ThirdPartyCall, error) {
	type result struct {
		call *ThirdPartyCall
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		c, err := Dial3PCC(cfg)
		ch <- result{c, err}
	}()
	select {
	case r := <-ch:
		return r.call, r.err
	case <-ctx.Done():
		return nil, fmt.Errorf("3pcc: context cancelled: %w", ctx.Err())
	}
}
