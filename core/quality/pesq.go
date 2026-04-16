// Package quality provides post-call audio quality analysis tools.
package quality

import (
	"os/exec"
	"strconv"
	"strings"
)

// PESQResult holds the output of a PESQ analysis run.
type PESQResult struct {
	MOSScore float64
	Error    string
}

// RunPESQ runs the external `pesq` CLI tool to compute MOS-LQO between
// a reference WAV and a degraded (received) WAV file.
//
// Prerequisites:
//   - pesq binary must be in PATH (install via apt/brew or from https://www.itu.int/rec/T-REC-P.862)
//   - Both WAV files must be 8kHz, mono, 16-bit PCM
//
// Returns PESQResult with Score=0 if the binary is not found (graceful degradation).
func RunPESQ(refWav, degradedWav string) PESQResult {
	cmd := exec.Command("pesq", "+8000", refWav, degradedWav)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// pesq binary not found or failed — return 0 gracefully
		return PESQResult{Error: err.Error()}
	}

	return parsePESQOutput(string(out))
}

// parsePESQOutput extracts the MOS-LQO score from pesq CLI output.
// The output format varies by implementation; we look for "MOS-LQO" or
// a bare float on the last non-empty line.
func parsePESQOutput(output string) PESQResult {
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Format: "MOS-LQO:         3.456"  or  "Predicted MOS-LQO: 3.456"
		if strings.Contains(strings.ToUpper(line), "MOS-LQO") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				raw := parts[len(parts)-1]
				if score, err := strconv.ParseFloat(raw, 64); err == nil {
					return PESQResult{MOSScore: score}
				}
			}
		}
	}

	// Fallback: try last line as a bare float
	if len(lines) > 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if score, err := strconv.ParseFloat(last, 64); err == nil {
			return PESQResult{MOSScore: score}
		}
	}

	return PESQResult{Error: "could not parse PESQ output: " + output}
}

// IsAvailable returns true if the `pesq` binary is present in PATH.
func IsAvailable() bool {
	_, err := exec.LookPath("pesq")
	return err == nil
}
