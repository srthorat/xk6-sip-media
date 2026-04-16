package sip

import (
	"context"
	"fmt"
	"time"

	sipmsg "github.com/emiago/sipgo/sip"
)

// OptionsConfig defines the parameters for a SIP OPTIONS ping.
type OptionsConfig struct {
	Target    string
	LocalIP   string
	Transport string
	Timeout   time.Duration
	TLSConfig *TLSConfig
}

// OptionsResult holds the outcome of a SIP ping.
type OptionsResult struct {
	StatusCode int
	RTT        time.Duration
}

// SendOptions sends a SIP OPTIONS ping to the target URI to measure round-trip time
// and verify SBC/PBX health without establishing an active call.
func SendOptions(cfg OptionsConfig) (*OptionsResult, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.Transport == "" {
		cfg.Transport = TransportUDP
	}

	localIP := resolveLocalIP(cfg.LocalIP)
	tlsCfg := cfg.TLSConfig
	if tlsCfg == nil && cfg.Transport == TransportTLS {
		tlsCfg = &TLSConfig{InsecureSkipVerify: true}
	}

	sipClient, err := NewClientWithTransport(localIP, cfg.Transport, tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("options: create client: %w", err)
	}
	defer sipClient.Close()

	var toURI sipmsg.Uri
	if e := sipmsg.ParseUri(cfg.Target, &toURI); e != nil {
		return nil, fmt.Errorf("options: parse target: %w", e)
	}

	req := sipmsg.NewRequest(sipmsg.OPTIONS, toURI)
	req.AppendHeader(sipmsg.NewHeader("Content-Length", "0"))
	req.AppendHeader(sipmsg.NewHeader("Accept", "application/sdp"))

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	start := time.Now()
	tx, e := sipClient.client.TransactionRequest(ctx, req)
	if e != nil {
		return nil, fmt.Errorf("options: send request: %w", e)
	}

	select {
	case resp := <-tx.Responses():
		if resp == nil {
			return nil, fmt.Errorf("options: nil response")
		}
		return &OptionsResult{
			StatusCode: int(resp.StatusCode),
			RTT:        time.Since(start),
		}, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("options: timeout or context cancelled: %w", ctx.Err())
	}
}
