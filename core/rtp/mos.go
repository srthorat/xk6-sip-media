package rtp

import "math"

// CalculateMOS estimates audio MOS (Mean Opinion Score) from packet loss and
// jitter using a simplified ITU-T E-model (G.107) R-factor approximation.
//
// Inputs:
//   - lossPercent: packet loss percentage (0..100)
//   - jitterMs:    average jitter in milliseconds
//
// Output: MOS in range [1.0, 5.0].
//
// The simplified E-model:
//
//	R = 94.2 − (lossPercent × 2.5) − (jitterMs × 0.1)
//	MOS = 1 + 0.035R + R(R−60)(100−R) × 7×10⁻⁶
//
// Reference: ITU-T G.107, ETSI TS 101 329-5 Annex E.
func CalculateMOS(lossPercent, jitterMs float64) float64 {
	R := 94.2 - (lossPercent * 2.5) - (jitterMs * 0.1)

	if R < 0 {
		R = 0
	}
	if R > 100 {
		R = 100
	}

	mos := 1.0 + 0.035*R + (R*(R-60)*(100-R))*7e-6

	mos = math.Max(1.0, math.Min(5.0, mos))
	return math.Round(mos*100) / 100 // round to 2 decimal places
}

// MOSGrade returns a human-readable quality label for a MOS value.
func MOSGrade(mos float64) string {
	switch {
	case mos >= 4.3:
		return "Excellent"
	case mos >= 4.0:
		return "Good"
	case mos >= 3.5:
		return "Fair"
	case mos >= 3.0:
		return "Poor"
	default:
		return "Bad"
	}
}
