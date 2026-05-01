package k6ext

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync/atomic"
)

// ── CredentialPool ────────────────────────────────────────────────────────────
//
// Mirrors SIPp's -inf CSV injection:
//
//	SEQUENTIAL          ← optional first line (default)
//	username,password,domain,callee
//	alice,pass1,sip.example.com,443361
//	bob,pass2,sip.example.com,443361
//
// Separator: comma or semicolon (auto-detected).
// Mode line: SEQUENTIAL (round-robin by VU id) or RANDOM.
// Headers: first data row after the optional mode line.
//          If absent (SIPp-style numeric-only), fields are named field0…fieldN.

// csvRow is a single parsed credential row.
type csvRow map[string]string

// K6CredentialPool holds all parsed rows and is safe for concurrent VU access.
type K6CredentialPool struct {
	rows    []csvRow
	mode    string // "sequential" | "random"
	counter atomic.Int64
}

// loadCSVFile parses a CSV file and returns a K6CredentialPool.
// opts is an optional JS object; currently supports "mode" key.
func loadCSVFile(path string, opts map[string]interface{}) (*K6CredentialPool, error) {
	f, err := os.Open(path) //nolint:gosec // user-supplied path, load-test tool
	if err != nil {
		return nil, fmt.Errorf("loadCSV: open %q: %w", path, err)
	}
	defer f.Close()

	pool := &K6CredentialPool{mode: "sequential"}

	// Allow JS caller to force mode
	if v, ok := opts["mode"].(string); ok {
		switch strings.ToLower(v) {
		case "random":
			pool.mode = "random"
		case "sequential":
			pool.mode = "sequential"
		}
	}

	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("loadCSV: read %q: %w", path, err)
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("loadCSV: %q is empty", path)
	}

	// Consume optional mode line
	idx := 0
	first := strings.ToUpper(lines[0])
	if first == "SEQUENTIAL" || first == "RANDOM" {
		// Only override if not forced by opts
		if _, forced := opts["mode"].(string); !forced {
			pool.mode = strings.ToLower(first)
		}
		idx++
	}

	if idx >= len(lines) {
		return nil, fmt.Errorf("loadCSV: %q has no data rows", path)
	}

	// Auto-detect separator from the first remaining line
	sep := detectSeparator(lines[idx])

	// Parse header
	headers := splitRow(lines[idx], sep)
	// Determine if this line is a header (contains non-numeric, non-sip-uri tokens)
	// or a data row (SIPp-style, no header).
	// Heuristic: header if every field is a valid identifier (letters, digits, _).
	isHeader := true
	for _, h := range headers {
		for _, ch := range h {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
				(ch >= '0' && ch <= '9') || ch == '_') {
				isHeader = false
				break
			}
		}
		if !isHeader {
			break
		}
	}

	dataStart := idx
	if isHeader {
		dataStart = idx + 1
	} else {
		// No explicit header — use field0, field1, ...
		headers = make([]string, len(headers))
		for i := range headers {
			headers[i] = fmt.Sprintf("field%d", i)
		}
	}

	if dataStart >= len(lines) {
		return nil, fmt.Errorf("loadCSV: %q has headers but no data rows", path)
	}

	// Parse data rows
	for _, line := range lines[dataStart:] {
		vals := splitRow(line, sep)
		row := make(csvRow, len(headers))
		for i, h := range headers {
			if i < len(vals) {
				row[h] = vals[i]
			} else {
				row[h] = ""
			}
		}
		pool.rows = append(pool.rows, row)
	}

	return pool, nil
}

// Pick returns the row for the given VU id (1-based) using the pool's mode.
// Sequential: row = rows[(vuId-1) % len(rows)]
// Random: row = rows[rand.Intn(len(rows))]
func (p *K6CredentialPool) Pick(vuID int64) map[string]interface{} {
	if len(p.rows) == 0 {
		return nil
	}
	var idx int
	if p.mode == "random" {
		idx = rand.Intn(len(p.rows)) //nolint:gosec // not cryptographic
	} else {
		// Sequential: distribute VUs across rows; wrap around if VUs > rows.
		// Uses (vuId-1) so VU 1 → row[0], VU 2 → row[1], etc.
		idx = int((vuID - 1) % int64(len(p.rows)))
		if idx < 0 {
			idx = 0
		}
	}
	return rowToJS(p.rows[idx])
}

// PickRoundRobin returns the next row using a shared atomic counter.
// Useful when you want each *call* (not each VU) to rotate through creds.
func (p *K6CredentialPool) PickRoundRobin() map[string]interface{} {
	if len(p.rows) == 0 {
		return nil
	}
	n := p.counter.Add(1) - 1
	idx := int(n % int64(len(p.rows)))
	return rowToJS(p.rows[idx])
}

// PickRandom returns a uniformly random row.
func (p *K6CredentialPool) PickRandom() map[string]interface{} {
	if len(p.rows) == 0 {
		return nil
	}
	return rowToJS(p.rows[rand.Intn(len(p.rows))]) //nolint:gosec
}

// Len returns the number of rows in the pool.
func (p *K6CredentialPool) Len() int {
	return len(p.rows)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func detectSeparator(line string) rune {
	semicolons := strings.Count(line, ";")
	commas := strings.Count(line, ",")
	if semicolons > commas {
		return ';'
	}
	return ','
}

func splitRow(line string, sep rune) []string {
	parts := strings.Split(line, string(sep))
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

func rowToJS(row csvRow) map[string]interface{} {
	out := make(map[string]interface{}, len(row))
	for k, v := range row {
		out[k] = v
	}
	return out
}
