package sip

import (
	"net"
	"strings"
	"testing"
)

// TestNewClientWithTransport_UDP verifies UDP client creation succeeds.
func TestNewClientWithTransport_UDP(t *testing.T) {
	c, err := NewClientWithTransport("127.0.0.1", TransportUDP, nil)
	if err != nil {
		t.Fatalf("UDP client: %v", err)
	}
	defer c.Close()
	if c.Transport() != TransportUDP {
		t.Errorf("expected transport=%q got %q", TransportUDP, c.Transport())
	}
}

// TestNewClientWithTransport_TCP verifies TCP client creation succeeds.
func TestNewClientWithTransport_TCP(t *testing.T) {
	c, err := NewClientWithTransport("127.0.0.1", TransportTCP, nil)
	if err != nil {
		t.Fatalf("TCP client: %v", err)
	}
	defer c.Close()
	if c.Transport() != TransportTCP {
		t.Errorf("expected transport=%q got %q", TransportTCP, c.Transport())
	}
}

// TestNewClientWithTransport_TLS_SkipVerify verifies TLS client creation
// with InsecureSkipVerify succeeds (no cert needed).
func TestNewClientWithTransport_TLS_SkipVerify(t *testing.T) {
	tlsCfg := &TLSConfig{InsecureSkipVerify: true}
	c, err := NewClientWithTransport("127.0.0.1", TransportTLS, tlsCfg)
	if err != nil {
		t.Fatalf("TLS client: %v", err)
	}
	defer c.Close()
	if c.Transport() != TransportTLS {
		t.Errorf("expected transport=%q got %q", TransportTLS, c.Transport())
	}
}

// TestNewClientWithTransport_TLS_BadCert verifies that a non-existent cert
// file returns an appropriate error.
func TestNewClientWithTransport_TLS_BadCert(t *testing.T) {
	tlsCfg := &TLSConfig{
		CertFile: "/nonexistent/client.pem",
		KeyFile:  "/nonexistent/client.key",
	}
	_, err := NewClientWithTransport("127.0.0.1", TransportTLS, tlsCfg)
	if err == nil {
		t.Fatal("expected error for missing cert file, got nil")
	}
	if !strings.Contains(err.Error(), "load client cert") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestNewClientWithTransport_TLS_BadCA verifies that a non-existent CA file
// returns an appropriate error.
func TestNewClientWithTransport_TLS_BadCA(t *testing.T) {
	tlsCfg := &TLSConfig{
		CAFile:             "/nonexistent/ca.pem",
		InsecureSkipVerify: false,
	}
	_, err := NewClientWithTransport("127.0.0.1", TransportTLS, tlsCfg)
	if err == nil {
		t.Fatal("expected error for missing CA file, got nil")
	}
	if !strings.Contains(err.Error(), "read CA file") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestTransportConstantValues ensures the transport constants have the
// exact string values expected by sipgo's transport layer.
func TestTransportConstantValues(t *testing.T) {
	if TransportUDP != "udp" {
		t.Errorf("TransportUDP = %q, want %q", TransportUDP, "udp")
	}
	if TransportTCP != "tcp" {
		t.Errorf("TransportTCP = %q, want %q", TransportTCP, "tcp")
	}
	if TransportTLS != "tls" {
		t.Errorf("TransportTLS = %q, want %q", TransportTLS, "tls")
	}
}

// TestBuildTLSConfig_NilInput verifies nil config returns a permissive config.
func TestBuildTLSConfig_NilInput(t *testing.T) {
	tlsConf, err := buildTLSConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tlsConf.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true for nil config")
	}
}

// TestBuildTLSConfig_ServerName verifies ServerName is propagated.
func TestBuildTLSConfig_ServerName(t *testing.T) {
	tlsConf, err := buildTLSConfig(&TLSConfig{
		ServerName:         "pbx.example.com",
		InsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tlsConf.ServerName != "pbx.example.com" {
		t.Errorf("ServerName=%q, want %q", tlsConf.ServerName, "pbx.example.com")
	}
}

// ── resolveLocalIP (fix 6.2) ──────────────────────────────────────────────────

// TestResolveLocalIP_Empty verifies that resolveLocalIP("") returns a non-empty
// valid IP address (auto-detected from the routing table).
func TestResolveLocalIP_Empty(t *testing.T) {
	got := resolveLocalIP("")
	if got == "" {
		t.Fatal("resolveLocalIP(\"\") returned empty string")
	}
	ip := net.ParseIP(got)
	if ip == nil {
		t.Errorf("resolveLocalIP returned non-IP %q", got)
	}
}

// TestResolveLocalIP_Passthrough verifies that a pre-set IP is returned unchanged.
func TestResolveLocalIP_Passthrough(t *testing.T) {
	const fixed = "192.168.1.42"
	got := resolveLocalIP(fixed)
	if got != fixed {
		t.Errorf("resolveLocalIP(%q) = %q; want passthrough", fixed, got)
	}
}

// TestResolveLocalIP_MatchesAuto verifies that resolveLocalIP and
// resolveLocalIPAuto(localIP, false) return identical results — confirming that
// resolveLocalIP is a thin delegate rather than a separate implementation.
func TestResolveLocalIP_MatchesAuto(t *testing.T) {
	for _, input := range []string{"", "10.0.0.1"} {
		fromShort := resolveLocalIP(input)
		fromAuto := resolveLocalIPAuto(input, false)
		if fromShort != fromAuto {
			t.Errorf("resolveLocalIP(%q)=%q differs from resolveLocalIPAuto(%q, false)=%q",
				input, fromShort, input, fromAuto)
		}
	}
}
