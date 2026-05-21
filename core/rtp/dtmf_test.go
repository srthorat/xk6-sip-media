package rtp

import (
	"testing"
)

// ── decodeDTMFEvent ──────────────────────────────────────────────────────────

func TestDecodeDTMFEvent_Valid(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		digit   string
		end     bool
	}{
		{"digit 1 intermediate", []byte{1, 0x00, 0x00, 0xA0}, "1", false},
		{"digit 1 end", []byte{1, 0x80, 0x00, 0xA0}, "1", true},
		{"digit 0", []byte{0, 0x80, 0x00, 0xA0}, "0", true},
		{"digit star", []byte{10, 0x80, 0x00, 0xA0}, "*", true},
		{"digit hash", []byte{11, 0x80, 0x00, 0xA0}, "#", true},
		{"digit A", []byte{12, 0x80, 0x00, 0xA0}, "A", true},
		{"digit D", []byte{15, 0x80, 0x00, 0xA0}, "D", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			digit, end, ok := decodeDTMFEvent(tc.payload)
			if !ok {
				t.Fatal("expected ok=true")
			}
			if digit != tc.digit {
				t.Errorf("digit: got %q, want %q", digit, tc.digit)
			}
			if end != tc.end {
				t.Errorf("end: got %v, want %v", end, tc.end)
			}
		})
	}
}

func TestDecodeDTMFEvent_Invalid(t *testing.T) {
	// payload too short
	if _, _, ok := decodeDTMFEvent([]byte{1, 0x80, 0x00}); ok {
		t.Error("expected ok=false for 3-byte payload")
	}
	if _, _, ok := decodeDTMFEvent(nil); ok {
		t.Error("expected ok=false for nil payload")
	}
	// event code 16 is out of range
	if _, _, ok := decodeDTMFEvent([]byte{16, 0x80, 0x00, 0xA0}); ok {
		t.Error("expected ok=false for event code 16")
	}
}

// ── DTMFCollector ────────────────────────────────────────────────────────────

func TestNewDTMFCollector_DefaultPT(t *testing.T) {
	c := NewDTMFCollector(0) // 0 → fall back to standard 101
	if c.PT != DTMFPayloadType {
		t.Errorf("PT: got %d, want %d", c.PT, DTMFPayloadType)
	}
}

func TestNewDTMFCollector_CustomPT(t *testing.T) {
	c := NewDTMFCollector(96)
	if c.PT != 96 {
		t.Errorf("PT: got %d, want 96", c.PT)
	}
}

func TestDTMFCollector_Dedup_SameTimestampAndCode(t *testing.T) {
	c := NewDTMFCollector(101)
	// Three end-packets for the same event (same ts + code) — RFC 2833 retransmissions.
	c.accept(1000, 1, "1")
	c.accept(1000, 1, "1")
	c.accept(1000, 1, "1")
	got := c.Digits()
	if len(got) != 1 {
		t.Errorf("expected 1 digit after dedup, got %d: %v", len(got), got)
	}
}

func TestDTMFCollector_DifferentTimestamps_SameDigit(t *testing.T) {
	c := NewDTMFCollector(101)
	// Same digit pressed twice — distinct timestamps → two entries.
	c.accept(1000, 1, "1")
	c.accept(2000, 1, "1")
	got := c.Digits()
	if len(got) != 2 {
		t.Errorf("expected 2 digits for two keypresses, got %d: %v", len(got), got)
	}
}

func TestDTMFCollector_SameTimestamp_DifferentCode(t *testing.T) {
	c := NewDTMFCollector(101)
	// Unlikely in practice but two different digits at the same timestamp are distinct.
	c.accept(1000, 1, "1")
	c.accept(1000, 2, "2")
	got := c.Digits()
	if len(got) != 2 {
		t.Errorf("expected 2 digits, got %d: %v", len(got), got)
	}
}

func TestDTMFCollector_MatchesExpected_Exact(t *testing.T) {
	c := NewDTMFCollector(101)
	c.accept(1000, 1, "1")
	c.accept(2000, 3, "3")
	if !c.MatchesExpected([]string{"1", "3"}) {
		t.Error("exact match should return true")
	}
}

func TestDTMFCollector_MatchesExpected_Subsequence(t *testing.T) {
	c := NewDTMFCollector(101)
	c.accept(1000, 1, "1")
	c.accept(2000, 2, "2")
	c.accept(3000, 3, "3")
	// [1,3] is a valid subsequence of [1,2,3]
	if !c.MatchesExpected([]string{"1", "3"}) {
		t.Error("subsequence match should return true")
	}
}

func TestDTMFCollector_MatchesExpected_Mismatch(t *testing.T) {
	c := NewDTMFCollector(101)
	c.accept(1000, 1, "1")
	c.accept(2000, 2, "2")
	// "5" was never received
	if c.MatchesExpected([]string{"1", "5"}) {
		t.Error("match should fail when expected digit not present")
	}
}

func TestDTMFCollector_MatchesExpected_Empty(t *testing.T) {
	c := NewDTMFCollector(101)
	if !c.MatchesExpected(nil) {
		t.Error("nil expected should always return true")
	}
	if !c.MatchesExpected([]string{}) {
		t.Error("empty expected should always return true")
	}
}

func TestDTMFCollector_MatchesExpected_EmptyCollector(t *testing.T) {
	c := NewDTMFCollector(101)
	if c.MatchesExpected([]string{"1"}) {
		t.Error("non-empty expected against empty collector should return false")
	}
}

func TestDTMFCollector_Digits_IsolatedCopy(t *testing.T) {
	c := NewDTMFCollector(101)
	c.accept(1000, 1, "1")
	snap := c.Digits()
	// Mutating the snapshot must not affect the collector's internal state.
	snap[0] = "9"
	got := c.Digits()
	if got[0] != "1" {
		t.Errorf("internal state was mutated through Digits() snapshot: got %q", got[0])
	}
}
