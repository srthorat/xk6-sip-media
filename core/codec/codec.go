// Package codec provides audio codec abstractions and implementations.
package codec

// Codec is a stateless audio codec that encodes PCM16 frames to bytes.
type Codec interface {
	// Name returns the codec name (e.g., "PCMU", "PCMA").
	Name() string
	// PayloadType returns the RTP payload type number (RFC 3551).
	PayloadType() uint8
	// Encode encodes one frame of PCM16 samples into codec bytes.
	Encode(frame []int16) []byte
	// Decode decodes codec bytes back to PCM16 samples.
	Decode(payload []byte) []int16
}

// New returns a Codec by name. Supported: "PCMU", "PCMA", "G722".
// Returns nil if the codec name is unknown.
func New(name string) Codec {
	switch name {
	case "PCMU":
		return &PCMUCodec{}
	case "PCMA":
		return &PCMACodec{}
	case "G722":
		return &G722Codec{}
	default:
		return nil
	}
}

// G722Codec implements the Codec interface for G.722 wideband audio.
type G722Codec struct{}

func (c *G722Codec) Name() string                  { return "G722" }
func (c *G722Codec) PayloadType() uint8            { return G722PayloadType }
func (c *G722Codec) Encode(frame []int16) []byte   { return G722Frame(frame) }
func (c *G722Codec) Decode(payload []byte) []int16 { return G722Decode(payload) }
