package audio_test

import (
	"testing"

	"github.com/srthorat/xk6-sip-media/core/audio"
)

func TestSilenceRatio_AllSilent(t *testing.T) {
	samples := make([]int16, 160)
	ratio := audio.SilenceRatio(samples)
	if ratio != 1.0 {
		t.Errorf("expected 1.0 for all-zero samples, got %.2f", ratio)
	}
}

func TestSilenceRatio_AllActive(t *testing.T) {
	samples := make([]int16, 160)
	for i := range samples {
		samples[i] = 5000
	}
	ratio := audio.SilenceRatio(samples)
	if ratio != 0.0 {
		t.Errorf("expected 0.0 for all-active samples, got %.2f", ratio)
	}
}

func TestSilenceRatio_Empty(t *testing.T) {
	ratio := audio.SilenceRatio(nil)
	if ratio != 1.0 {
		t.Errorf("expected 1.0 for empty slice, got %.2f", ratio)
	}
}

func TestSilenceRatioBytes_PCMUSilence(t *testing.T) {
	payload := make([]byte, 160)
	for i := range payload {
		payload[i] = 0xFF
	}
	ratio := audio.SilenceRatioBytes(payload)
	if ratio != 1.0 {
		t.Errorf("expected 1.0 for PCMU silence bytes, got %.2f", ratio)
	}
}

func TestSplitFrames(t *testing.T) {
	samples := make([]int16, 500)
	frames := audio.SplitFrames(samples, 160)
	if len(frames) != 3 {
		t.Errorf("expected 3 frames, got %d", len(frames))
	}
	for i, f := range frames {
		if len(f) != 160 {
			t.Errorf("frame %d: expected 160 samples, got %d", i, len(f))
		}
	}
}
