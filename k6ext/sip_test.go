package k6ext

import (
	"testing"
	"time"
)

// ── toInt (fix 6.3) ───────────────────────────────────────────────────────────

// TestToInt_Int64 covers the most common Goja integer emission type.
func TestToInt_Int64(t *testing.T) {
	if got := toInt(int64(42)); got != 42 {
		t.Errorf("want 42, got %d", got)
	}
}

// TestToInt_Float64 covers JS numbers emitted as float64 (e.g. 7.0 → 7).
func TestToInt_Float64(t *testing.T) {
	if got := toInt(float64(7.0)); got != 7 {
		t.Errorf("want 7, got %d", got)
	}
}

// TestToInt_Int covers native Go int values.
func TestToInt_Int(t *testing.T) {
	if got := toInt(int(99)); got != 99 {
		t.Errorf("want 99, got %d", got)
	}
}

// TestToInt_Int32 covers 32-bit integer emission.
func TestToInt_Int32(t *testing.T) {
	if got := toInt(int32(1000)); got != 1000 {
		t.Errorf("want 1000, got %d", got)
	}
}

// TestToInt_UnknownType returns 0 for types that cannot be a JS integer.
func TestToInt_UnknownType(t *testing.T) {
	if got := toInt("not-a-number"); got != 0 {
		t.Errorf("want 0 for string, got %d", got)
	}
}

// TestToInt_Nil returns 0 for nil (missing map key).
func TestToInt_Nil(t *testing.T) {
	if got := toInt(nil); got != 0 {
		t.Errorf("want 0 for nil, got %d", got)
	}
}

// TestToInt_MapMissingKey confirms that accessing a missing key from opts (which
// returns nil) produces 0 — the pattern used in parseCfg.
func TestToInt_MapMissingKey(t *testing.T) {
	opts := map[string]interface{}{}
	if got := toInt(opts["rtpPort"]); got != 0 {
		t.Errorf("missing key: want 0, got %d", got)
	}
}

// ── parseCfg DTMF delays (fix 6.4) ───────────────────────────────────────────

// TestParseCfg_DTMFInitialDelay verifies the dtmfInitialDelay option is parsed
// and stored in CallConfig.DTMFInitialDelay.
func TestParseCfg_DTMFInitialDelay(t *testing.T) {
	cfg := parseCfg(map[string]interface{}{
		"dtmfInitialDelay": "500ms",
	})
	if cfg.DTMFInitialDelay != 500*time.Millisecond {
		t.Errorf("DTMFInitialDelay: want 500ms, got %v", cfg.DTMFInitialDelay)
	}
}

// TestParseCfg_DTMFInterDigitGap verifies the dtmfInterDigitGap option is parsed
// and stored in CallConfig.DTMFInterDigitGap.
func TestParseCfg_DTMFInterDigitGap(t *testing.T) {
	cfg := parseCfg(map[string]interface{}{
		"dtmfInterDigitGap": "1s",
	})
	if cfg.DTMFInterDigitGap != time.Second {
		t.Errorf("DTMFInterDigitGap: want 1s, got %v", cfg.DTMFInterDigitGap)
	}
}

// TestParseCfg_DTMFBothFields verifies both DTMF delay fields can be set
// simultaneously via parseCfg.
func TestParseCfg_DTMFBothFields(t *testing.T) {
	cfg := parseCfg(map[string]interface{}{
		"dtmfInitialDelay":  "3s",
		"dtmfInterDigitGap": "250ms",
	})
	if cfg.DTMFInitialDelay != 3*time.Second {
		t.Errorf("DTMFInitialDelay: want 3s, got %v", cfg.DTMFInitialDelay)
	}
	if cfg.DTMFInterDigitGap != 250*time.Millisecond {
		t.Errorf("DTMFInterDigitGap: want 250ms, got %v", cfg.DTMFInterDigitGap)
	}
}

// TestParseCfg_DTMFZeroValue verifies that omitting DTMF delay options leaves
// the fields at zero (the dial.go goroutine then applies the 2s default).
func TestParseCfg_DTMFZeroValue(t *testing.T) {
	cfg := parseCfg(map[string]interface{}{})
	if cfg.DTMFInitialDelay != 0 {
		t.Errorf("zero opts: DTMFInitialDelay should be 0, got %v", cfg.DTMFInitialDelay)
	}
	if cfg.DTMFInterDigitGap != 0 {
		t.Errorf("zero opts: DTMFInterDigitGap should be 0, got %v", cfg.DTMFInterDigitGap)
	}
}

// TestParseCfg_DTMFInvalidDuration verifies that an unparseable duration string
// leaves the field at zero rather than panicking.
func TestParseCfg_DTMFInvalidDuration(t *testing.T) {
	cfg := parseCfg(map[string]interface{}{
		"dtmfInitialDelay":  "not-a-duration",
		"dtmfInterDigitGap": "also-bad",
	})
	if cfg.DTMFInitialDelay != 0 {
		t.Errorf("invalid duration: DTMFInitialDelay should be 0, got %v", cfg.DTMFInitialDelay)
	}
	if cfg.DTMFInterDigitGap != 0 {
		t.Errorf("invalid duration: DTMFInterDigitGap should be 0, got %v", cfg.DTMFInterDigitGap)
	}
}
