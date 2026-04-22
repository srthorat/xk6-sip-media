package codec

import "github.com/zaf/g711"

// PCMUCodec implements G.711 μ-law (PCMU) — RTP payload type 0.
type PCMUCodec struct{}

func (c *PCMUCodec) Name() string       { return "PCMU" }
func (c *PCMUCodec) PayloadType() uint8 { return 0 }
func (c *PCMUCodec) SampleRate() int    { return 8000 }

// Encode converts a frame of PCM16 samples to PCMU bytes (one byte per sample).
func (c *PCMUCodec) Encode(frame []int16) []byte {
	out := make([]byte, len(frame))
	for i, s := range frame {
		out[i] = g711.EncodeUlawFrame(s)
	}
	return out
}

// Decode converts PCMU bytes back to PCM16 samples.
func (c *PCMUCodec) Decode(payload []byte) []int16 {
	out := make([]int16, len(payload))
	for i, b := range payload {
		out[i] = g711.DecodeUlawFrame(b)
	}
	return out
}

func (c *PCMUCodec) Close() error { return nil }

// PCMACodec implements G.711 A-law (PCMA) — RTP payload type 8.
type PCMACodec struct{}

func (c *PCMACodec) Name() string       { return "PCMA" }
func (c *PCMACodec) PayloadType() uint8 { return 8 }
func (c *PCMACodec) SampleRate() int    { return 8000 }

// Encode converts a frame of PCM16 samples to PCMA bytes.
func (c *PCMACodec) Encode(frame []int16) []byte {
	out := make([]byte, len(frame))
	for i, s := range frame {
		out[i] = g711.EncodeAlawFrame(s)
	}
	return out
}

// Decode converts PCMA bytes back to PCM16 samples.
func (c *PCMACodec) Decode(payload []byte) []int16 {
	out := make([]int16, len(payload))
	for i, b := range payload {
		out[i] = g711.DecodeAlawFrame(b)
	}
	return out
}

func (c *PCMACodec) Close() error { return nil }
