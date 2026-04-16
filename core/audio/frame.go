package audio

// FrameSize is the number of PCM samples per 20ms frame at 8kHz.
// 8000 samples/sec × 0.020 sec = 160 samples.
const FrameSize = 160

// SamplesPerSecond is the expected audio sample rate (8kHz telephony).
const SamplesPerSecond = 8000

// FrameDurationMs is the RTP packetization interval in milliseconds.
const FrameDurationMs = 20

// SplitFrames splits a PCM16 sample slice into equal-sized frames.
// Any trailing samples that don't fill a complete frame are discarded.
func SplitFrames(samples []int16, frameSize int) [][]int16 {
	count := len(samples) / frameSize
	frames := make([][]int16, count)
	for i := range frames {
		start := i * frameSize
		frames[i] = samples[start : start+frameSize]
	}
	return frames
}
