package sip

import (
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
