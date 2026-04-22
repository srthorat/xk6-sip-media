package audio

import (
	"fmt"
	"strings"
)

// CodecEncoder is the minimal interface needed from core/codec.Codec.
// Duplicated here to avoid an import cycle (audio → codec → audio).
type CodecEncoder interface {
	Name() string
	PayloadType() uint8
	Encode(frame []int16) []byte
}

// LoadAudioForCodec loads any audio file (WAV or MP3) and encodes it as
// RTP payloads for the given codec. It automatically selects the correct
// sample rate and frame size for the codec.
//
//   - PCMU / PCMA / G729: 8 kHz,  160 samples/frame  (20ms)
//   - G722:               16 kHz, 320 samples/frame  (20ms at 16kHz)
//   - OPUS:               48 kHz, 960 samples/frame  (20ms at 48kHz)
//
// Accepts WAV and MP3 — format detected by magic bytes.
//
//	payloads, err := audio.LoadAudioForCodec("hold_music.mp3", pcmuCodec)
func LoadAudioForCodec(path string, cod CodecEncoder) ([][]byte, error) {
	if cod == nil {
		return nil, fmt.Errorf("audio: codec is nil")
	}

	targetRate, frameSize := rateAndFrameForCodec(cod.Name())

	samples, err := LoadAudio(path, targetRate)
	if err != nil {
		return nil, err
	}

	frames := SplitFrames(samples, frameSize)
	payloads := make([][]byte, len(frames))
	for i, f := range frames {
		payloads[i] = cod.Encode(f)
	}
	return payloads, nil
}

// rateAndFrameForCodec returns (sampleRate, samplesPerFrame) for a codec name.
func rateAndFrameForCodec(name string) (rate, frameSize int) {
	switch strings.ToUpper(name) {
	case "G722":
		return TargetSampleRate16k, 320 // 16kHz × 20ms = 320 samples
	case "OPUS":
		return TargetSampleRate48k, 960 // 48kHz × 20ms = 960 samples
	default:
		// PCMU, PCMA, G729, and any unknown codec → telephony 8kHz
		return TargetSampleRate8k, FrameSize // 8kHz × 20ms = 160 samples
	}
}
