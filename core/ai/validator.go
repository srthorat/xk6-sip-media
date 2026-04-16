// Package ai provides AI-based IVR transcript validation.
// Whisper STT integration is guarded by the WHISPER_ENABLED environment variable.
package ai

import (
	"os"
	"os/exec"
	"strings"
)

// TranscriptValidator validates IVR prompts using STT output.
type TranscriptValidator struct {
	// WhisperModel is the Whisper model to use (e.g. "base", "small", "medium").
	WhisperModel string
}

// NewValidator returns a TranscriptValidator.
func NewValidator(model string) *TranscriptValidator {
	if model == "" {
		model = "base"
	}
	return &TranscriptValidator{WhisperModel: model}
}

// IsEnabled returns true when the WHISPER_ENABLED env var is set to "1".
func IsEnabled() bool {
	return os.Getenv("WHISPER_ENABLED") == "1"
}

// Transcribe runs the `whisper` CLI on an audio file and returns the transcript.
// Returns empty string if Whisper is not enabled or the binary is not in PATH.
func (v *TranscriptValidator) Transcribe(audioPath string) string {
	if !IsEnabled() {
		return ""
	}
	_, err := exec.LookPath("whisper")
	if err != nil {
		return ""
	}

	cmd := exec.Command(
		"whisper", audioPath,
		"--model", v.WhisperModel,
		"--language", "en",
		"--output_format", "txt",
		"--output_dir", os.TempDir(),
	)
	out, _ := cmd.CombinedOutput()
	return strings.TrimSpace(string(out))
}

// ValidateTranscript checks whether the transcript contains the expected phrase.
// Comparison is case-insensitive and uses substring matching.
func ValidateTranscript(transcript, expected string) bool {
	return strings.Contains(
		strings.ToLower(transcript),
		strings.ToLower(expected),
	)
}

// ValidateIVRPrompt transcribes the audio file and checks for the expected phrase.
// Returns (matched bool, transcript string).
func (v *TranscriptValidator) ValidateIVRPrompt(audioPath, expected string) (bool, string) {
	transcript := v.Transcribe(audioPath)
	if transcript == "" {
		return false, ""
	}
	return ValidateTranscript(transcript, expected), transcript
}
