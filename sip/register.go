package sip

import (
	"context"
	"fmt"
	"time"

	"github.com/emiago/sipgo"
	sipmsg "github.com/emiago/sipgo/sip"
)

// RegisterConfig holds parameters for a SIP REGISTER transaction.
type RegisterConfig struct {
	// Registrar is the SIP registrar URI, e.g. "sip:pbx.example.com".
	Registrar string

	// AOR is the Address of Record, e.g. "sip:alice@pbx.example.com".
	AOR string

	// Username and Password for Digest authentication (RFC 2617).
	Username string
	Password string

	// Expires is the registration lifetime in seconds (default 3600).
	Expires int

	// LocalIP is the local IP for the Contact header.
	LocalIP string

	// Transport selects the signaling transport: "udp" (default), "tcp", "tls".
	Transport string

	// TLSConfig holds TLS parameters when Transport == "tls".
	// If nil and Transport is "tls", InsecureSkipVerify is used.
	TLSConfig *TLSConfig
}

// Registration represents an active SIP registration.
type Registration struct {
	client    *Client
	cfg       RegisterConfig
	ExpiresAt time.Time
}

// Register performs a SIP REGISTER transaction, handling 401 Digest Auth
// challenge automatically.
//
// On success the returned *Registration can be used to Refresh or Unregister.
func Register(cfg RegisterConfig) (*Registration, error) {
	if cfg.Expires == 0 {
		cfg.Expires = 3600
	}
	localIP := resolveLocalIP(cfg.LocalIP)

	transport := cfg.Transport
	if transport == "" {
		transport = TransportUDP
	}
	tlsCfg := cfg.TLSConfig
	if tlsCfg == nil && transport == TransportTLS {
		tlsCfg = &TLSConfig{InsecureSkipVerify: true}
	}

	sipClient, err := NewClientWithTransport(localIP, transport, tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("register: create client: %w", err)
	}

	var registrarURI sipmsg.Uri
	if err := sipmsg.ParseUri(cfg.Registrar, &registrarURI); err != nil {
		sipClient.Close()
		return nil, fmt.Errorf("register: parse registrar URI %q: %w", cfg.Registrar, err)
	}

	req, err := buildRegisterRequest(registrarURI, cfg, localIP)
	if err != nil {
		sipClient.Close()
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := sipClient.client.Do(ctx, req, sipgo.ClientRequestRegisterBuild)
	if err != nil {
		sipClient.Close()
		return nil, fmt.Errorf("register: send REGISTER: %w", err)
	}

	// Handle 401 Unauthorized or 407 Proxy Authentication Required
	if resp.StatusCode == 401 || resp.StatusCode == 407 {
		resp, err = sipClient.client.DoDigestAuth(ctx, req, resp, sipgo.DigestAuth{
			Username: cfg.Username,
			Password: cfg.Password,
		})
		if err != nil {
			sipClient.Close()
			return nil, fmt.Errorf("register: digest auth: %w", err)
		}
	}

	if resp.StatusCode != 200 {
		sipClient.Close()
		return nil, fmt.Errorf("register: unexpected response %d", resp.StatusCode)
	}

	return &Registration{
		client:    sipClient,
		cfg:       cfg,
		ExpiresAt: time.Now().Add(time.Duration(cfg.Expires) * time.Second),
	}, nil
}

// Refresh re-sends REGISTER to renew the registration before it expires.
func (r *Registration) Refresh() error {
	refreshCfg := r.cfg
	// Reuse the same client
	localIP := resolveLocalIP(r.cfg.LocalIP)

	var registrarURI sipmsg.Uri
	if err := sipmsg.ParseUri(r.cfg.Registrar, &registrarURI); err != nil {
		return fmt.Errorf("register refresh: %w", err)
	}

	req, err := buildRegisterRequest(registrarURI, refreshCfg, localIP)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := r.client.client.Do(ctx, req, sipgo.ClientRequestRegisterBuild)
	if err != nil {
		return fmt.Errorf("register refresh: %w", err)
	}
	if resp.StatusCode == 401 || resp.StatusCode == 407 {
		resp, err = r.client.client.DoDigestAuth(ctx, req, resp, sipgo.DigestAuth{
			Username: r.cfg.Username,
			Password: r.cfg.Password,
		})
		if err != nil {
			return fmt.Errorf("register refresh: digest auth: %w", err)
		}
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("register refresh: status %d", resp.StatusCode)
	}

	r.ExpiresAt = time.Now().Add(time.Duration(r.cfg.Expires) * time.Second)
	return nil
}

// Unregister sends REGISTER with Expires: 0 to remove the registration.
func (r *Registration) Unregister() error {
	cfg := r.cfg
	cfg.Expires = 0

	localIP := resolveLocalIP(r.cfg.LocalIP)

	var registrarURI sipmsg.Uri
	if err := sipmsg.ParseUri(r.cfg.Registrar, &registrarURI); err != nil {
		return fmt.Errorf("unregister: %w", err)
	}

	req, err := buildRegisterRequest(registrarURI, cfg, localIP)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := r.client.client.Do(ctx, req, sipgo.ClientRequestRegisterBuild)
	if err != nil {
		return fmt.Errorf("unregister: %w", err)
	}
	if resp.StatusCode == 401 || resp.StatusCode == 407 {
		resp, err = r.client.client.DoDigestAuth(ctx, req, resp, sipgo.DigestAuth{
			Username: r.cfg.Username,
			Password: r.cfg.Password,
		})
		if err != nil {
			return fmt.Errorf("unregister: digest auth: %w", err)
		}
	}
	_ = r.client.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("unregister: status %d", resp.StatusCode)
	}
	return nil
}

// buildRegisterRequest constructs the REGISTER SIP request message.
func buildRegisterRequest(
	registrar sipmsg.Uri,
	cfg RegisterConfig,
	localIP string,
) (*sipmsg.Request, error) {
	req := sipmsg.NewRequest(sipmsg.REGISTER, registrar)

	var aorURI sipmsg.Uri
	if err := sipmsg.ParseUri(cfg.AOR, &aorURI); err != nil {
		return nil, fmt.Errorf("register: parse AOR %q: %w", cfg.AOR, err)
	}

	// REGISTER must present the Address of Record in both To and From.
	// Do not rely on sipgo's fallback From generation, which uses the user-agent
	// name instead of the SIP account identity.
	to := sipmsg.ToHeader{
		Address: sipmsg.Uri{
			Scheme:    aorURI.Scheme,
			User:      aorURI.User,
			Host:      aorURI.Host,
			Port:      aorURI.Port,
			UriParams: sipmsg.NewParams(),
			Headers:   sipmsg.NewParams(),
		},
		Params: sipmsg.NewParams(),
	}
	from := sipmsg.FromHeader{
		Address: sipmsg.Uri{
			Scheme:    aorURI.Scheme,
			User:      aorURI.User,
			Host:      aorURI.Host,
			Port:      aorURI.Port,
			UriParams: sipmsg.NewParams(),
			Headers:   sipmsg.NewParams(),
		},
		Params: sipmsg.NewParams(),
	}
	from.Params.Add("tag", sipmsg.GenerateTagN(16))
	req.AppendHeader(&from)
	req.AppendHeader(&to)

	// Contact: extract the username from the AOR so the Contact user matches.
	// Vonage and many carriers reject REGISTER if the Contact user differs from
	// the AOR user.
	contactUser := cfg.Username
	if contactUser == "" {
		if err := sipmsg.ParseUri(cfg.AOR, &aorURI); err == nil && aorURI.User != "" {
			contactUser = aorURI.User
		}
	}
	contactParams := sipmsg.NewParams()
	contactParams.Add("ob", "")
	contact := sipmsg.ContactHeader{
		Address: sipmsg.Uri{
			User:      contactUser,
			Host:      localIP,
			UriParams: contactParams,
		},
	}
	req.AppendHeader(&contact)

	// Expires
	exp := sipmsg.ExpiresHeader(cfg.Expires)
	req.AppendHeader(&exp)

	return req, nil
}
