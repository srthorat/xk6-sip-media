package k6ext

import (
	"os"
	"testing"
)

func writeCSV(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "creds-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

// TestCSVPool_Sequential verifies round-robin distribution by VU id.
func TestCSVPool_Sequential(t *testing.T) {
	path := writeCSV(t, "SEQUENTIAL\nusername,password,domain\nalice,pass1,pbx.example.com\nbob,pass2,pbx.example.com\n")
	pool, err := loadCSVFile(path, nil)
	if err != nil {
		t.Fatalf("loadCSVFile: %v", err)
	}
	if pool.Len() != 2 {
		t.Fatalf("want 2 rows, got %d", pool.Len())
	}

	r1 := pool.Pick(1)
	if r1["username"] != "alice" {
		t.Errorf("VU 1 should get alice, got %v", r1["username"])
	}
	r2 := pool.Pick(2)
	if r2["username"] != "bob" {
		t.Errorf("VU 2 should get bob, got %v", r2["username"])
	}
	// Wrap-around: VU 3 → row 0 (alice)
	r3 := pool.Pick(3)
	if r3["username"] != "alice" {
		t.Errorf("VU 3 (wrap) should get alice, got %v", r3["username"])
	}
}

// TestCSVPool_SemicolonSeparator verifies SIPp-style semicolon separator.
func TestCSVPool_SemicolonSeparator(t *testing.T) {
	path := writeCSV(t, "username;password;domain\nalice;pass1;pbx.example.com\n")
	pool, err := loadCSVFile(path, nil)
	if err != nil {
		t.Fatalf("loadCSVFile: %v", err)
	}
	r := pool.Pick(1)
	if r["password"] != "pass1" {
		t.Errorf("semicolon separator: want pass1, got %v", r["password"])
	}
}

// TestCSVPool_NoHeader verifies SIPp-style headerless CSV (numeric field names).
func TestCSVPool_NoHeader(t *testing.T) {
	// No header — first line is data (contains '@' so isHeader = false)
	path := writeCSV(t, "alice@pbx;pass1;pbx.example.com\n")
	pool, err := loadCSVFile(path, nil)
	if err != nil {
		t.Fatalf("loadCSVFile: %v", err)
	}
	r := pool.Pick(1)
	if r["field0"] != "alice@pbx" {
		t.Errorf("headerless CSV: want field0=alice@pbx, got %v", r["field0"])
	}
	if r["field1"] != "pass1" {
		t.Errorf("headerless CSV: want field1=pass1, got %v", r["field1"])
	}
}

// TestCSVPool_CommentAndBlankLines verifies that # comments and blank lines are skipped.
func TestCSVPool_CommentAndBlankLines(t *testing.T) {
	path := writeCSV(t, "# this is a comment\n\nusername,password\nalice,pass1\n\n# another comment\nbob,pass2\n")
	pool, err := loadCSVFile(path, nil)
	if err != nil {
		t.Fatalf("loadCSVFile: %v", err)
	}
	if pool.Len() != 2 {
		t.Fatalf("want 2 rows (comments/blanks stripped), got %d", pool.Len())
	}
}

// TestCSVPool_ModeOverrideOpts verifies that opts["mode"] overrides the CSV mode line.
func TestCSVPool_ModeOverrideOpts(t *testing.T) {
	path := writeCSV(t, "SEQUENTIAL\nusername,password\nalice,pass1\n")
	pool, err := loadCSVFile(path, map[string]interface{}{"mode": "random"})
	if err != nil {
		t.Fatalf("loadCSVFile: %v", err)
	}
	if pool.mode != "random" {
		t.Errorf("opts mode override: want random, got %s", pool.mode)
	}
}

// TestCSVPool_RoundRobin verifies the atomic counter increments correctly.
func TestCSVPool_RoundRobin(t *testing.T) {
	path := writeCSV(t, "username,password\nalice,pass1\nbob,pass2\n")
	pool, err := loadCSVFile(path, nil)
	if err != nil {
		t.Fatalf("loadCSVFile: %v", err)
	}
	r1 := pool.PickRoundRobin()
	r2 := pool.PickRoundRobin()
	r3 := pool.PickRoundRobin()
	if r1["username"] == r2["username"] {
		t.Error("consecutive PickRoundRobin should return different rows")
	}
	// Third call wraps back to first
	if r3["username"] != r1["username"] {
		t.Errorf("third PickRoundRobin should wrap to first row; got %v want %v", r3["username"], r1["username"])
	}
}

// TestCSVPool_Empty verifies graceful error on empty file.
func TestCSVPool_Empty(t *testing.T) {
	path := writeCSV(t, "")
	_, err := loadCSVFile(path, nil)
	if err == nil {
		t.Error("expected error for empty CSV, got nil")
	}
}
