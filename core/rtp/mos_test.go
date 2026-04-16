package rtp_test

import (
	"math"
	"testing"

	corertp "xk6-sip-media/core/rtp"
)

func TestCalculateMOS_Perfect(t *testing.T) {
	mos := corertp.CalculateMOS(0, 0)
	// Zero loss, zero jitter → maximum MOS (~4.4)
	if mos < 4.3 || mos > 5.0 {
		t.Errorf("expected MOS ~4.4 for perfect network, got %.2f", mos)
	}
}

func TestCalculateMOS_HighLoss(t *testing.T) {
	mos := corertp.CalculateMOS(20, 0) // 20% packet loss
	if mos >= 3.0 {
		t.Errorf("expected MOS < 3.0 for 20%% loss, got %.2f", mos)
	}
}

func TestCalculateMOS_ClampMin(t *testing.T) {
	mos := corertp.CalculateMOS(100, 500)
	if mos < 1.0 {
		t.Errorf("MOS must not go below 1.0, got %.2f", mos)
	}
}

func TestCalculateMOS_ClampMax(t *testing.T) {
	mos := corertp.CalculateMOS(0, 0)
	if mos > 5.0 {
		t.Errorf("MOS must not exceed 5.0, got %.2f", mos)
	}
}

func TestCalculateMOS_Rounding(t *testing.T) {
	mos := corertp.CalculateMOS(5, 10)
	rounded := math.Round(mos*100) / 100
	if mos != rounded {
		t.Errorf("MOS should be rounded to 2dp: got %v", mos)
	}
}

func TestMOSGrade(t *testing.T) {
	cases := []struct {
		mos   float64
		grade string
	}{
		{4.5, "Excellent"},
		{4.1, "Good"},
		{3.7, "Fair"},
		{3.1, "Poor"},
		{2.0, "Bad"},
	}
	for _, tc := range cases {
		got := corertp.MOSGrade(tc.mos)
		if got != tc.grade {
			t.Errorf("MOSGrade(%.1f) = %q, want %q", tc.mos, got, tc.grade)
		}
	}
}
