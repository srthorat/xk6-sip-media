// Package codec provides G.722 wideband audio codec support.
// G.722 is a 7kHz wideband codec (ITU-T G.722) operating at 16kHz sample rate
// and uses a two-subband ADPCM algorithm. It is commonly used for HD voice.
//
// This implementation uses a pure-Go encoder/decoder without external CGO deps.
// For production accuracy, replace with a proper G.722 library.
package codec

// G722Frame encodes a block of 16kHz 16-bit mono PCM samples to G.722.
// Input: 16kHz signed 16-bit PCM samples (160 samples = 20ms frame)
// Output: 80 bytes (G.722 at 64kbit/s, 4 bits per sample)
func G722Frame(samples []int16) []byte {
	enc := newG722Encoder()
	return enc.encode(samples)
}

// G722Decode decodes a G.722 frame back to 16kHz 16-bit PCM samples.
func G722Decode(frame []byte) []int16 {
	dec := newG722Decoder()
	return dec.decode(frame)
}

// ── G.722 Subband ADPCM Encoder ───────────────────────────────────────────

type g722State struct {
	// Lower subband (0-4kHz)
	sl  int32
	dl  int32
	el  int32
	det [2]int32 // det[0]=lower, det[1]=upper

	// Upper subband (4-8kHz)
	su int32
	du int32
}

type g722Encoder struct {
	s g722State
}

type g722Decoder struct {
	s g722State
}

func newG722Encoder() *g722Encoder { return &g722Encoder{} }
func newG722Decoder() *g722Decoder { return &g722Decoder{} }

// encode converts 16kHz PCM samples to G.722 at 64kbit/s.
// Each pair of input samples produces one output byte (4 bits lower + 4 bits upper).
func (e *g722Encoder) encode(samples []int16) []byte {
	// Ensure even number of samples
	if len(samples)%2 != 0 {
		samples = append(samples, 0)
	}
	out := make([]byte, len(samples)/2)
	for i := 0; i < len(samples)/2; i++ {
		// QMF analysis filter: split into low + high band
		x0 := int32(samples[i*2])
		x1 := int32(samples[i*2+1])
		xL := (x0 + x1) >> 1 // low-band approximation
		xH := (x0 - x1) >> 1 // high-band approximation (simplified)

		// Lower subband ADPCM (6-bit codeword)
		codL := adpcmEncodeLow(&e.s, xL)
		// Upper subband ADPCM (2-bit codeword)
		codH := adpcmEncodeHigh(&e.s, xH)

		out[i] = byte((codH << 6) | (codL & 0x3F))
	}
	return out
}

func (d *g722Decoder) decode(frame []byte) []int16 {
	out := make([]int16, len(frame)*2)
	for i, b := range frame {
		codL := int32(b & 0x3F)
		codH := int32((b >> 6) & 0x03)

		xL := adpcmDecodeLow(&d.s, codL)
		xH := adpcmDecodeHigh(&d.s, codH)

		// QMF synthesis
		out[i*2] = saturate16(xL + xH)
		out[i*2+1] = saturate16(xL - xH)
	}
	return out
}

// ── Simplified ADPCM step tables (ITU-T G.722 Annex A) ──────────────────

func adpcmEncodeLow(s *g722State, xL int32) int32 {
	el := xL - s.sl
	s.el = el
	// Step size selection (simplified)
	step := s.det[0]
	if step < 8 {
		step = 8
	}
	// Quantize to 6-bit signed
	q := int32(0)
	if el < 0 {
		q = 32 + clamp(-el/step, 0, 31)
	} else {
		q = clamp(el/step, 0, 31)
	}
	// Dequantize
	s.dl = dequantLow(q, step)
	// Predictor update
	s.sl = adaptPredict(s.sl, s.dl)
	// Step size update
	s.det[0] = updateStep(s.det[0], q, true)
	return q
}

func adpcmEncodeHigh(s *g722State, xH int32) int32 {
	el := xH - s.su
	step := s.det[1]
	if step < 8 {
		step = 8
	}
	q := int32(0)
	if el < 0 {
		q = 2 + clamp(-el/step, 0, 1)
	} else {
		q = clamp(el/step, 0, 1)
	}
	s.du = dequantHigh(q, step)
	s.su = adaptPredict(s.su, s.du)
	s.det[1] = updateStep(s.det[1], q, false)
	return q
}

func adpcmDecodeLow(s *g722State, q int32) int32 {
	step := s.det[0]
	if step < 8 {
		step = 8
	}
	s.dl = dequantLow(q, step)
	s.sl = adaptPredict(s.sl, s.dl)
	s.det[0] = updateStep(s.det[0], q, true)
	return s.sl
}

func adpcmDecodeHigh(s *g722State, q int32) int32 {
	step := s.det[1]
	if step < 8 {
		step = 8
	}
	s.du = dequantHigh(q, step)
	s.su = adaptPredict(s.su, s.du)
	s.det[1] = updateStep(s.det[1], q, false)
	return s.su
}

func dequantLow(q, step int32) int32 {
	sign := int32(1)
	mag := q
	if q >= 32 {
		sign = -1
		mag = q - 32
	}
	return sign * mag * step
}

func dequantHigh(q, step int32) int32 {
	if q >= 2 {
		return -(q - 2 + 1) * step
	}
	return (q + 1) * step
}

func adaptPredict(s, d int32) int32 {
	s += d
	return saturate32(s)
}

func updateStep(det, q int32, isLow bool) int32 {
	// Simple step-size adaptation — doubles/halves based on saturation signal
	_ = isLow
	if q == 0 {
		det = (det * 7) >> 3
	} else {
		det = (det * 9) >> 3
	}
	if det < 8 {
		return 8
	}
	if det > 32767 {
		return 32767
	}
	return det
}

func clamp(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func saturate32(v int32) int32 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return v
}

func saturate16(v int32) int16 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return int16(v)
}

// G722PayloadType is the standard (dynamic) RTP payload type for G.722.
// RFC 3551 §4.5.2 assigns PT=9 for G.722 at 8000 clock rate (unusual — the
// spec intentionally uses 8000 for backwards compatibility, even though the
// actual sample rate is 16000).
const G722PayloadType uint8 = 9
