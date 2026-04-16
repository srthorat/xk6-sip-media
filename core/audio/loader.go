// Package audio provides format-agnostic audio loading for xk6-sip-media.
//
// Supported input formats:
//   - WAV  (.wav)  — any sample rate, mono or stereo, 8/16/24/32-bit PCM
//   - MP3  (.mp3)  — any bitrate, mono or stereo (pure Go, no CGO)
//
// All formats are internally converted to:
//   - 8 kHz, mono, int16  for G.711 (PCMU/PCMA)
//   - 16 kHz, mono, int16 for G.722
//
// The entry point is LoadAudio(path, targetSampleRate) which detects the
// format by magic bytes (not just extension) and returns ready-to-encode
// PCM16 samples at the requested sample rate.
package audio

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	goaudiowav "github.com/go-audio/wav"
	gomp3 "github.com/hajimehoshi/go-mp3"
)

// TargetSampleRate8k is the telephony standard (G.711, PCMU, PCMA).
const TargetSampleRate8k = 8000

// TargetSampleRate16k is the wideband standard (G.722).
const TargetSampleRate16k = 16000

// AudioFile holds decoded + normalised PCM audio ready for RTP encoding.
type AudioFile struct {
	Samples    []int16
	SampleRate int
	Channels   int
}

// LoadAudio detects format by magic bytes, decodes WAV or MP3,
// downmixes to mono, and resamples to targetRate (8000 or 16000).
//
//	samples, err := LoadAudio("./hold_music.mp3", 8000)
//	samples, err := LoadAudio("./announcement.wav", 8000)
func LoadAudio(path string, targetRate int) ([]int16, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("audio: read %q: %w", path, err)
	}

	var af *AudioFile
	switch detectFormat(data, path) {
	case "wav":
		af, err = decodeWAV(data)
	case "mp3":
		af, err = decodeMP3(data)
	default:
		return nil, fmt.Errorf("audio: unsupported format for %q (supported: .wav .mp3)", path)
	}
	if err != nil {
		return nil, err
	}

	// Stereo → mono
	mono := toMono(af.Samples, af.Channels)

	// Resample to target rate
	if af.SampleRate != targetRate {
		mono = resample(mono, af.SampleRate, targetRate)
	}

	return mono, nil
}

// LoadAudioAsPayloads loads any audio file and returns pre-encoded RTP payloads.
// codec is the encoder function (e.g. g711.EncodeUlawFrame).
//
//	payloads, err := LoadAudioAsPayloads("hold.mp3", 8000, 160, func(s []int16) []byte {
//	    return encodePCMU(s)
//	})
func LoadAudioAsPayloads(path string, targetRate, frameSize int, encode func([]int16) []byte) ([][]byte, error) {
	samples, err := LoadAudio(path, targetRate)
	if err != nil {
		return nil, err
	}
	frames := SplitFrames(samples, frameSize)
	payloads := make([][]byte, len(frames))
	for i, f := range frames {
		payloads[i] = encode(f)
	}
	return payloads, nil
}

// ── Format detection ─────────────────────────────────────────────────────────

func detectFormat(data []byte, path string) string {
	// Magic byte detection is authoritative
	if len(data) >= 12 {
		// RIFF....WAVE
		if bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WAVE")) {
			return "wav"
		}
	}
	if len(data) >= 3 {
		// ID3 tag (MP3 with metadata) or raw MP3 sync word
		if bytes.Equal(data[0:3], []byte("ID3")) {
			return "mp3"
		}
		// Raw MPEG sync: 0xFF 0xFB/0xFA/0xF3/0xF2 (MPEG1/2 Layer3)
		if data[0] == 0xFF && (data[1]&0xE0) == 0xE0 {
			return "mp3"
		}
	}

	// Fallback: extension
	switch strings.ToLower(fileExt(path)) {
	case ".wav":
		return "wav"
	case ".mp3":
		return "mp3"
	}
	return "unknown"
}

func fileExt(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
	}
	return ""
}

// ── WAV decoder ──────────────────────────────────────────────────────────────

func decodeWAV(data []byte) (*AudioFile, error) {
	dec := goaudiowav.NewDecoder(bytes.NewReader(data))
	if !dec.IsValidFile() {
		return nil, fmt.Errorf("audio: not a valid WAV file")
	}

	// Read WAV metadata before full decode
	if err := dec.FwdToPCM(); err != nil {
		return nil, fmt.Errorf("audio: WAV seek to PCM: %w", err)
	}

	sampleRate := int(dec.SampleRate)
	channels := int(dec.NumChans)

	buf, err := dec.FullPCMBuffer()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("audio: WAV decode: %w", err)
	}

	// go-audio returns samples interleaved by channel
	samples := IntToInt16(buf.Data)

	return &AudioFile{
		Samples:    samples,
		SampleRate: sampleRate,
		Channels:   channels,
	}, nil
}

// ── MP3 decoder ──────────────────────────────────────────────────────────────

func decodeMP3(data []byte) (*AudioFile, error) {
	dec, err := gomp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("audio: MP3 decoder init: %w", err)
	}

	sampleRate := dec.SampleRate()
	// go-mp3 always outputs stereo int16 little-endian
	const channels = 2

	// Read all PCM bytes
	raw, err := io.ReadAll(dec)
	if err != nil {
		return nil, fmt.Errorf("audio: MP3 decode: %w", err)
	}

	// Convert little-endian []byte → []int16
	samples := make([]int16, len(raw)/2)
	for i := range samples {
		lo := uint16(raw[i*2])
		hi := uint16(raw[i*2+1])
		samples[i] = int16(lo | hi<<8)
	}

	return &AudioFile{
		Samples:    samples,
		SampleRate: sampleRate,
		Channels:   channels,
	}, nil
}

// ── Signal processing ────────────────────────────────────────────────────────

// toMono downmixes interleaved multi-channel PCM to mono by averaging channels.
func toMono(samples []int16, channels int) []int16 {
	if channels <= 1 {
		return samples
	}
	mono := make([]int16, len(samples)/channels)
	for i := range mono {
		var sum int32
		for ch := 0; ch < channels; ch++ {
			idx := i*channels + ch
			if idx < len(samples) {
				sum += int32(samples[idx])
			}
		}
		mono[i] = int16(sum / int32(channels))
	}
	return mono
}

// resample converts mono PCM from srcRate to dstRate using linear interpolation.
// Accurate enough for telephony; for production use a polyphase filter.
func resample(samples []int16, srcRate, dstRate int) []int16 {
	if srcRate == dstRate || len(samples) == 0 {
		return samples
	}

	ratio := float64(srcRate) / float64(dstRate)
	outLen := int(math.Round(float64(len(samples)) / ratio))
	out := make([]int16, outLen)

	for i := range out {
		srcPos := float64(i) * ratio
		idx := int(srcPos)
		frac := srcPos - float64(idx)

		if idx >= len(samples)-1 {
			out[i] = samples[len(samples)-1]
		} else {
			s0 := float64(samples[idx])
			s1 := float64(samples[idx+1])
			out[i] = int16(s0 + frac*(s1-s0))
		}
	}
	return out
}
