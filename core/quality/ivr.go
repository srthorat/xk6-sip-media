package quality

// IVRResult holds the outcome of an IVR validation check.
type IVRResult struct {
	Valid  bool
	Reason string
}

// IVRThresholds controls the rule-based IVR validation gates.
type IVRThresholds struct {
	// MaxSilenceRatio is the maximum fraction of silence allowed in the
	// received audio before the call is flagged as broken (default: 0.80).
	MaxSilenceRatio float64

	// MinMOS is the minimum acceptable E-model MOS score (default: 3.0).
	MinMOS float64
}

// DefaultThresholds returns reasonable production defaults.
func DefaultThresholds() IVRThresholds {
	return IVRThresholds{
		MaxSilenceRatio: 0.80,
		MinMOS:          3.0,
	}
}

// ValidateIVR applies rule-based gates to determine if a call's IVR audio
// meets quality thresholds. Checks are applied in order; the first failure wins.
func ValidateIVR(silence, mos float64, t IVRThresholds) IVRResult {
	if silence > t.MaxSilenceRatio {
		return IVRResult{
			Valid:  false,
			Reason: "no audio received (silence ratio too high)",
		}
	}
	if mos < t.MinMOS {
		return IVRResult{
			Valid:  false,
			Reason: "call quality too poor (MOS below threshold)",
		}
	}
	return IVRResult{Valid: true, Reason: "OK"}
}
