package audio

import (
	"math"
	"testing"
)

// generateSinePCM generates a sine wave at freq Hz, sampleRate Hz, for durationMs ms.
func generateSinePCM(freq, sampleRate, durationMs int) []int16 {
	n := sampleRate * durationMs / 1000
	samples := make([]int16, n)
	for i := range samples {
		angle := 2 * math.Pi * float64(freq) * float64(i) / float64(sampleRate)
		samples[i] = int16(10000 * math.Sin(angle))
	}
	return samples
}

func TestToMono_Stereo(t *testing.T) {
	// Interleaved stereo: L=1000, R=2000 → mono should be 1500
	stereo := []int16{1000, 2000, 1000, 2000}
	mono := toMono(stereo, 2)
	if len(mono) != 2 {
		t.Fatalf("expected 2 mono samples, got %d", len(mono))
	}
	if mono[0] != 1500 {
		t.Errorf("expected mono[0]=1500, got %d", mono[0])
	}
}

func TestToMono_AlreadyMono(t *testing.T) {
	samples := []int16{100, 200, 300}
	out := toMono(samples, 1)
	if len(out) != 3 || out[1] != 200 {
		t.Errorf("mono passthrough failed: %v", out)
	}
}

func TestResample_Upsample(t *testing.T) {
	// 8kHz → 16kHz: should double the length
	samples := generateSinePCM(440, 8000, 100) // 100ms = 800 samples
	out := resample(samples, 8000, 16000)
	expected := 1600
	if abs(len(out)-expected) > 2 {
		t.Errorf("upsample: expected ~%d samples, got %d", expected, len(out))
	}
}

func TestResample_Downsample(t *testing.T) {
	// 44100 → 8000: ratio 5.5125
	samples := generateSinePCM(440, 44100, 100) // 4410 samples
	out := resample(samples, 44100, 8000)
	expected := 800
	if abs(len(out)-expected) > 10 {
		t.Errorf("downsample: expected ~%d samples, got %d", expected, len(out))
	}
}

func TestResample_SameRate(t *testing.T) {
	samples := generateSinePCM(440, 8000, 100)
	out := resample(samples, 8000, 8000)
	if len(out) != len(samples) {
		t.Errorf("same-rate resample changed length: %d → %d", len(samples), len(out))
	}
}

func TestDetectFormat_WAV(t *testing.T) {
	// Minimal WAV magic bytes
	data := []byte("RIFF\x00\x00\x00\x00WAVEfmt ")
	format := detectFormat(data, "test.wav")
	if format != "wav" {
		t.Errorf("expected wav, got %q", format)
	}
}

func TestDetectFormat_MP3_ID3(t *testing.T) {
	data := []byte("ID3\x03\x00\x00\x00\x00\x00\x00")
	format := detectFormat(data, "test.mp3")
	if format != "mp3" {
		t.Errorf("expected mp3, got %q", format)
	}
}

func TestDetectFormat_MP3_SyncWord(t *testing.T) {
	data := []byte{0xFF, 0xFB, 0x90, 0x00} // MPEG1, Layer3 sync
	format := detectFormat(data, "audio.mp3")
	if format != "mp3" {
		t.Errorf("expected mp3, got %q", format)
	}
}

func TestDetectFormat_Unknown(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02, 0x03}
	format := detectFormat(data, "file.ogg")
	if format != "unknown" {
		t.Errorf("expected unknown, got %q", format)
	}
}

func TestLoadWAV_InvalidFile(t *testing.T) {
	_, err := LoadAudio("/nonexistent/file.wav", 8000)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadAudioForCodec_NilCodec(t *testing.T) {
	_, err := LoadAudioForCodec("anything.wav", nil)
	if err == nil {
		t.Error("expected error for nil codec")
	}
}

func TestRateAndFrame_PCMU(t *testing.T) {
	rate, frame := rateAndFrameForCodec("PCMU")
	if rate != 8000 {
		t.Errorf("PCMU rate=%d, want 8000", rate)
	}
	if frame != 160 {
		t.Errorf("PCMU frame=%d, want 160", frame)
	}
}

func TestRateAndFrame_G722(t *testing.T) {
	rate, frame := rateAndFrameForCodec("G722")
	if rate != 16000 {
		t.Errorf("G722 rate=%d, want 16000", rate)
	}
	if frame != 320 {
		t.Errorf("G722 frame=%d, want 320", frame)
	}
}

// TestRateAndFrame_Opus verifies Opus returns 48kHz / 960 samples per 20ms frame.
// This is a regression test: a missing OPUS case would fall through to 8kHz/160,
// feeding a 48kHz encoder the wrong frame size and producing silent RTP.
func TestRateAndFrame_Opus(t *testing.T) {
	rate, frame := rateAndFrameForCodec("OPUS")
	if rate != 48000 {
		t.Errorf("OPUS rate=%d, want 48000", rate)
	}
	if frame != 960 {
		t.Errorf("OPUS frame=%d, want 960 (20ms @ 48kHz)", frame)
	}
}

// TestRateAndFrame_G729 verifies G.729 uses 8kHz / 80 samples per 10ms frame.
func TestRateAndFrame_G729(t *testing.T) {
	rate, frame := rateAndFrameForCodec("G729")
	if rate != 8000 {
		t.Errorf("G729 rate=%d, want 8000", rate)
	}
	if frame != 160 {
		t.Errorf("G729 frame=%d, want 160 (20ms @ 8kHz)", frame)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
