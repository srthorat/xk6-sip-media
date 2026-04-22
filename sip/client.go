// Package sip wraps sipgo to provide a simple SIP UAC (User Agent Client)
// with support for UDP, TCP, and TLS transports.
package sip

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"

	"github.com/emiago/sipgo"
	sipmsg "github.com/emiago/sipgo/sip"
)

// TransportUDP is the default, plain UDP SIP transport.
const TransportUDP = "udp"

// TransportTCP is plain TCP SIP transport (more reliable than UDP for long
// connections; mandatory for large SIP messages > 1300 bytes).
const TransportTCP = "tcp"

// TransportTLS is TLS-encrypted SIP signaling (SIPS, RFC 3261 §26).
// Uses port 5061 by default.
const TransportTLS = "tls"

// TLSConfig holds parameters for TLS-encrypted SIP signaling.
type TLSConfig struct {
	// CertFile is the path to the PEM-encoded client certificate.
	// Required for mutual TLS; optional for server-only TLS.
	CertFile string

	// KeyFile is the path to the PEM-encoded private key.
	KeyFile string

	// CAFile is the path to the PEM-encoded CA certificate for
	// verifying the server certificate. If empty, the system CA pool is used.
	CAFile string

	// InsecureSkipVerify skips server certificate validation.
	// Use only in non-production test environments.
	InsecureSkipVerify bool

	// ServerName overrides the hostname used for SNI and certificate
	// verification. Useful when dialling an IP address.
	ServerName string
}

// Client wraps a sipgo UA, Client, and a DialogClientCache for sending SIP requests.
type Client struct {
	ua        *sipgo.UserAgent
	client    *sipgo.Client
	cache     *sipgo.DialogClientCache
	transport string // "udp", "tcp", "tls"
}

// NewClient creates a new SIP UAC with UDP transport (default behaviour).
func NewClient(localHost string) (*Client, error) {
	return NewClientWithTransport(localHost, TransportUDP, nil)
}

// NewClientWithTransport creates a new SIP UAC with the specified transport.
//
//   - transport: "udp", "tcp", or "tls"
//   - tlsCfg:   TLS parameters (required when transport == "tls", ignored otherwise)
func NewClientWithTransport(localHost, transport string, tlsCfg *TLSConfig) (*Client, error) {
	var uaOpts []sipgo.UserAgentOption
	uaOpts = append(uaOpts, sipgo.WithUserAgent("xk6-sip-media/1.0"))
	uaOpts = append(uaOpts, sipgo.WithUserAgentHostname(localHost))

	if transport == TransportTLS {
		tlsConf, err := buildTLSConfig(tlsCfg)
		if err != nil {
			return nil, fmt.Errorf("sip: build TLS config: %w", err)
		}
		uaOpts = append(uaOpts, sipgo.WithUserAgenTLSConfig(tlsConf))
	}

	ua, err := sipgo.NewUA(uaOpts...)
	if err != nil {
		return nil, fmt.Errorf("sip: create UA: %w", err)
	}

	client, err := sipgo.NewClient(ua,
		sipgo.WithClientHostname(localHost),
	)
	if err != nil {
		_ = ua.Close()
		return nil, fmt.Errorf("sip: create client: %w", err)
	}

	contactHDR := sipmsg.ContactHeader{
		Address: sipmsg.Uri{
			User:      "k6load",
			Host:      localHost,
			UriParams: sipmsg.NewParams(),
		},
	}

	// For TLS, mark the Contact URI as sips:
	if transport == TransportTLS {
		contactHDR.Address.Scheme = "sips"
	}

	cache := sipgo.NewDialogClientCache(client, contactHDR)

	return &Client{
		ua:        ua,
		client:    client,
		cache:     cache,
		transport: transport,
	}, nil
}

// Transport returns the transport name ("udp", "tcp", "tls").
func (c *Client) Transport() string { return c.transport }

// Close shuts down the underlying UA and all open connections.
func (c *Client) Close() error {
	return c.ua.Close()
}

// ── helpers ──────────────────────────────────────────────────────────────────

// buildTLSConfig constructs a *tls.Config from TLSConfig parameters.
func buildTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	tlsConf := &tls.Config{}

	if cfg == nil {
		// Permissive defaults for load testing (no mutual TLS, skip verify)
		tlsConf.InsecureSkipVerify = true //nolint:gosec // load-test tool
		return tlsConf, nil
	}

	tlsConf.InsecureSkipVerify = cfg.InsecureSkipVerify //nolint:gosec
	if cfg.ServerName != "" {
		tlsConf.ServerName = cfg.ServerName
	}

	// Client certificate (mutual TLS)
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client cert/key: %w", err)
		}
		tlsConf.Certificates = []tls.Certificate{cert}
	}

	// Custom CA pool
	if cfg.CAFile != "" {
		caPEM, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file %q: %w", cfg.CAFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("no valid certificates found in CA file %q", cfg.CAFile)
		}
		tlsConf.RootCAs = pool
	}

	return tlsConf, nil
}

// localOutboundIP returns the preferred local outbound IP address by
// dialling a UDP socket (no packets sent) to determine the routing interface.
func localOutboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1", nil
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil
}

// resolveLocalIP returns localIP if non-empty and not a wildcard,
// otherwise auto-detects the outbound interface IP (IPv4 only).
// It delegates to resolveLocalIPAuto which handles both IPv4 and IPv6 wildcards.
func resolveLocalIP(localIP string) string {
	return resolveLocalIPAuto(localIP, false)
}

// makeFromURI constructs a SIP from URI for the given local host.
func makeFromURI(localHost string) sipmsg.Uri {
	return sipmsg.Uri{User: "k6load", Host: localHost}
}
