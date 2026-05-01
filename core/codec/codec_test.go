package codec_test

import (
	"testing"

	"github.com/srthorat/xk6-sip-media/core/codec"
)

// generateSine produces a simple alternating tone pattern (not real sine, but non-silent PCM).
func generatePCM(n int, val int16) []int16 {
	out := make([]int16, n)
	for i := range out {
		if i%2 == 0 {
			out[i] = val
		} else {
			out[i] = -val
		}
	}
	return out
}

// TestPCMU_RoundTrip encodes then decodes a frame and verifies shape is preserved.
func TestPCMU_RoundTrip(t *testing.T) {
	c, err := codec.New("PCMU")
	if err != nil || c == nil {
		t.Fatal("PCMU codec not found")
	}
	defer c.Close()

	frame := generatePCM(160, 4096)
	encoded := c.Encode(frame)
	if len(encoded) == 0 {
		t.Fatal("PCMU encode returned empty bytes")
	}
	if len(encoded) != 160 {
		t.Fatalf("PCMU expected 160 bytes, got %d", len(encoded))
	}

	decoded := c.Decode(encoded)
	if len(decoded) == 0 {
		t.Fatal("PCMU decode returned empty samples")
	}
}

// TestPCMA_RoundTrip checks G.711 A-law round-trip.
func TestPCMA_RoundTrip(t *testing.T) {
	c, err := codec.New("PCMA")
	if err != nil || c == nil {
		t.Fatal("PCMA codec not found")
	}
	defer c.Close()

	frame := generatePCM(160, 2048)
	encoded := c.Encode(frame)
	if len(encoded) != 160 {
		t.Fatalf("PCMA expected 160 bytes, got %d", len(encoded))
	}
	decoded := c.Decode(encoded)
	if len(decoded) != 160 {
		t.Fatalf("PCMA decode expected 160 samples, got %d", len(decoded))
	}
}

// TestG722_RoundTrip checks wideband round-trip (output is half the frame length in bytes).
func TestG722_RoundTrip(t *testing.T) {
	c, err := codec.New("G722")
	if err != nil || c == nil {
		t.Fatal("G722 codec not found")
	}
	defer c.Close()

	frame := generatePCM(160, 8000)
	encoded := c.Encode(frame)
	if len(encoded) == 0 {
		t.Fatal("G722 encode returned empty bytes")
	}
	decoded := c.Decode(encoded)
	if len(decoded) == 0 {
		t.Fatal("G722 decode returned empty samples")
	}
}

// TestCodecFactory_UnknownReturnsError verifies unknown codecs return an error gracefully.
func TestCodecFactory_UnknownReturnsError(t *testing.T) {
	c, err := codec.New("INVALID_CODEC")
	if err == nil {
		t.Errorf("expected error for unknown codec, got nil")
	}
	if c != nil {
		t.Errorf("expected nil Codec for unknown name, got %T", c)
	}
}

// TestCodecFactory_SupportedNames verifies all supported codecs construct without panic.
func TestCodecFactory_SupportedNames(t *testing.T) {
	names := []string{"PCMU", "PCMA", "G722"}
	for _, name := range names {
		c, err := codec.New(name)
		if err != nil || c == nil {
			t.Errorf("expected non-nil codec for %q, got err=%v", name, err)
			continue
		}
		if c.Name() != name {
			t.Errorf("expected Name()=%q, got %q", name, c.Name())
		}
		_ = c.Close()
	}
}

// TestPayloadTypes verifies well-known static payload type assignments.
func TestPayloadTypes(t *testing.T) {
	tests := []struct {
		name string
		pt   uint8
	}{
		{"PCMU", 0},
		{"PCMA", 8},
	}
	for _, tt := range tests {
		c, err := codec.New(tt.name)
		if err != nil || c == nil {
			t.Errorf("codec %q not found: %v", tt.name, err)
			continue
		}
		if c.PayloadType() != tt.pt {
			t.Errorf("%s: expected PT=%d, got %d", tt.name, tt.pt, c.PayloadType())
		}
		_ = c.Close()
	}
}
