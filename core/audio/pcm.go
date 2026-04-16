package audio

// IntToInt16 converts a slice of int (as returned by go-audio PCM buffers)
// to []int16, clamping values to the int16 range.
func IntToInt16(in []int) []int16 {
	out := make([]int16, len(in))
	for i, v := range in {
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		out[i] = int16(v)
	}
	return out
}
