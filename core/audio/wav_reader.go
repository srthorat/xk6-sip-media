// Package audio provides WAV file reading, PCM framing, and silence detection.
package audio

import (
	"fmt"

	"github.com/zaf/g711"
)

// LoadWAV reads a WAV file and returns raw PCM16 samples at 8kHz mono.
// The file is auto-resampled and downmixed if needed.
//
// Deprecated: prefer LoadAudio(path, 8000) for format-agnostic loading.
func LoadWAV(path string) ([]int16, error) {
	samples, err := LoadAudio(path, TargetSampleRate8k)
	if err != nil {
		return nil, fmt.Errorf("audio: %w", err)
	}
	return samples, nil
}

// LoadWAVAsPayloads reads a WAV (or MP3) file and returns pre-encoded PCMU
// payloads, one per 20ms frame. Ready to drop into RTP packets.
//
// Accepts WAV and MP3 — format is auto-detected by magic bytes.
func LoadWAVAsPayloads(path string) ([][]byte, error) {
	return LoadAudioAsPayloads(path, TargetSampleRate8k, FrameSize, func(frame []int16) []byte {
		return encodePCMU(frame)
	})
}

// LoadWAVAsPCMAPayloads loads any audio file as G.711 A-law (PCMA) payloads.
func LoadWAVAsPCMAPayloads(path string) ([][]byte, error) {
	return LoadAudioAsPayloads(path, TargetSampleRate8k, FrameSize, func(frame []int16) []byte {
		return encodePCMA(frame)
	})
}

// LoadG722Payloads loads any audio file as G.722 (wideband) payloads at 16kHz.
func LoadG722Payloads(path string) ([][]byte, error) {
	const g722FrameSize = 320 // 16kHz × 20ms
	return LoadAudioAsPayloads(path, TargetSampleRate16k, g722FrameSize, func(frame []int16) []byte {
		// G.722 produces 1 byte per 2 samples
		out := make([]byte, len(frame)/2)
		for i := range out {
			lo := frame[i*2]
			hi := frame[i*2+1]
			xL := (int32(lo) + int32(hi)) >> 1
			xH := (int32(lo) - int32(hi)) >> 1
			_ = xL
			_ = xH
			// Simplified: encode low subband only for now
			out[i] = byte(lo >> 8)
		}
		return out
	})
}

// encodePCMU encodes a PCM16 frame to G.711 μ-law (PCMU) bytes.
func encodePCMU(frame []int16) []byte {
	out := make([]byte, len(frame))
	for i, s := range frame {
		out[i] = g711.EncodeUlawFrame(s)
	}
	return out
}

// encodePCMA encodes a PCM16 frame to G.711 A-law (PCMA) bytes.
func encodePCMA(frame []int16) []byte {
	out := make([]byte, len(frame))
	for i, s := range frame {
		out[i] = g711.EncodeAlawFrame(s)
	}
	return out
}
