package audio

// SilenceRatio returns the fraction of samples that fall within the silence
// threshold (absolute value ≤ threshold). A ratio close to 1.0 means the
// audio is almost entirely silent.
//
// Typical thresholds:
//   - 100 for telephony silence detection (int16 range: -32768..32767)
func SilenceRatio(samples []int16) float64 {
	if len(samples) == 0 {
		return 1.0 // treat empty as silent
	}
	const threshold = int16(100)
	var silent int
	for _, s := range samples {
		if s >= -threshold && s <= threshold {
			silent++
		}
	}
	return float64(silent) / float64(len(samples))
}

// SilenceRatioBytes decodes raw PCMU bytes back to approximate PCM energy
// and returns the silence ratio.
// Note: this is an approximation — full PCMU decoding belongs in the codec package.
func SilenceRatioBytes(pcmu []byte) float64 {
	if len(pcmu) == 0 {
		return 1.0
	}
	// PCMU 0xFF encodes silence (μ-law zero = 0xFF)
	var silent int
	for _, b := range pcmu {
		if b == 0xFF || b == 0x7F {
			silent++
		}
	}
	return float64(silent) / float64(len(pcmu))
}
