// Package codec provides audio codec abstractions and implementations.
package codec

import "fmt"

// Codec is a stateless audio codec that encodes PCM16 frames to bytes.
type Codec interface {
	// Name returns the codec name (e.g., "PCMU", "PCMA").
	Name() string
	// PayloadType returns the RTP payload type number (RFC 3551).
	PayloadType() uint8
	// SampleRate returns the codec's native clock rate in Hz (e.g. 8000, 48000).
	// Used to compute the correct RTP timestamp increment per 20ms frame.
	SampleRate() int
	// Encode encodes one frame of PCM16 samples into codec bytes.
	Encode(frame []int16) []byte
	// Decode decodes codec bytes back to PCM16 samples.
	Decode(payload []byte) []int16
	// Close explicitly releases native C/C++ memory for stateful codecs (Opus/G.729).
	Close() error
}

// New returns a Codec by name. Returns (nil, error) for unknown codecs or
// when a CGO codec fails to initialize (e.g. missing libopus).
// Supported: "PCMU", "PCMA", "G722", "OPUS", "G729" (build with -tags g729).
// G729 requires building with -tags g729.
func New(name string) (Codec, error) {
	switch name {
	case "PCMU":
		return &PCMUCodec{}, nil
	case "PCMA":
		return &PCMACodec{}, nil
	case "G722":
		return &G722Codec{}, nil
	case "OPUS":
		return NewOpus() // returns (Codec, error)
	case "G729":
		return newG729() // Uses build tags for optional GPL linking
	default:
		return nil, fmt.Errorf("unknown codec %q (supported: PCMU, PCMA, G722, OPUS, G729)", name)
	}
}

// G722Codec implements the Codec interface for G.722 wideband audio.
type G722Codec struct{}

func (c *G722Codec) Name() string                  { return "G722" }
func (c *G722Codec) PayloadType() uint8            { return G722PayloadType }
func (c *G722Codec) SampleRate() int               { return 8000 }
func (c *G722Codec) Encode(frame []int16) []byte   { return G722Frame(frame) }
func (c *G722Codec) Decode(payload []byte) []int16 { return G722Decode(payload) }
func (c *G722Codec) Close() error                  { return nil }
