package codec

import (
	"fmt"
	"sync"
	"github.com/hraban/opus"
)

// Opus is the ITU-T Opus Audio Codec (RFC 6716).
// Highly versatile, used for WebRTC. We configure it exclusively for 48kHz Mono VoIP.
type Opus struct {
	mu      sync.Mutex
	encoder *opus.Encoder
	decoder *opus.Decoder
	// 48 kHz mono -> 960 samples per 20ms frame
	frameSize int 
}

func NewOpus() (Codec, error) {
	// Standard WebRTC Audio Profile: 48kHz, 1 Channel
	enc, err := opus.NewEncoder(48000, 1, opus.AppVoIP)
	if err != nil {
		return nil, fmt.Errorf("opus encoder init: %w", err)
	}
	
	dec, err := opus.NewDecoder(48000, 1)
	if err != nil {
		return nil, fmt.Errorf("opus decoder init: %w", err)
	}

	return &Opus{
		encoder:   enc,
		decoder:   dec,
		frameSize: 960, // 20ms at 48kHz = 960 samples
	}, nil
}

func (c *Opus) Name() string {
	return "OPUS"
}

func (c *Opus) PayloadType() uint8 {
	return 111 // standard WebRTC dynamic payload type used for Opus
}

func (c *Opus) SampleRate() int {
	return 48000
}

func (c *Opus) Encode(frame []int16) []byte {
	if len(frame) == 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]byte, 1500)
	n, err := c.encoder.Encode(frame, out)
	if err != nil {
		return nil
	}
	return out[:n]
}

func (c *Opus) Decode(payload []byte) []int16 {
	if len(payload) == 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	pcmBuffer := make([]int16, c.frameSize)
	n, err := c.decoder.Decode(payload, pcmBuffer)
	if err != nil {
		return nil
	}
	return pcmBuffer[:n]
}

func (c *Opus) Close() error {
	// hraban/opus does not require explicit C-free memory management, Go GC handles it.
	return nil
}
