package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStdout captures stdout during function execution
func captureStdout(f func() error) (string, error) {
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := f()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String(), err
}

func TestLsTree_Execute(t *testing.T) {
	tmp := t.TempDir()
	origDatadir := option.Datadir
	option.Datadir = tmp
	defer func() { option.Datadir = origDatadir }()

	// Setup test data
	ds := NewDatastore(tmp)
	reader := strings.NewReader("test content")
	if err := ds.Write("file1", reader, []byte{}, ""); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Run command and capture output
	cmd := &LsTree{}
	out, err := captureStdout(func() error { return cmd.Execute([]string{}) })
	if err != nil {
		t.Fatalf("LsTree.Execute() failed: %v", err)
	}
	if !strings.Contains(out, "file1") {
		t.Errorf("expected 'file1' in output, got: %q", out)
	}
}

func TestCat_Execute(t *testing.T) {
	tmp := t.TempDir()
	origDatadir := option.Datadir
	option.Datadir = tmp
	defer func() { option.Datadir = origDatadir }()

	// Setup test data
	ds := NewDatastore(tmp)
	content := "hello world"
	reader := strings.NewReader(content)
	if err := ds.Write("test", reader, []byte{}, ""); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Test plain read
	cmd := &Cat{JSON: false}
	out, err := captureStdout(func() error { return cmd.Execute([]string{"test"}) })
	if err != nil {
		t.Fatalf("Cat.Execute() failed: %v", err)
	}
	if out != content {
		t.Errorf("expected %q, got %q", content, out)
	}
}

func TestCat_ExecuteJSON(t *testing.T) {
	tmp := t.TempDir()
	origDatadir := option.Datadir
	option.Datadir = tmp
	defer func() { option.Datadir = origDatadir }()

	// Setup test data with JSON content
	ds := NewDatastore(tmp)
	content := `{"key":"value"}`
	reader := strings.NewReader(content)
	if err := ds.Write("test", reader, []byte{}, ""); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Test JSON read
	cmd := &Cat{JSON: true}
	out, err := captureStdout(func() error { return cmd.Execute([]string{"test"}) })
	if err != nil {
		t.Fatalf("Cat.Execute(JSON) failed: %v", err)
	}
	if !strings.Contains(out, "key") || !strings.Contains(out, "value") {
		t.Errorf("expected JSON output with key/value, got: %q", out)
	}
}

func TestCat_ExecuteNotFound(t *testing.T) {
	tmp := t.TempDir()
	origDatadir := option.Datadir
	option.Datadir = tmp
	defer func() { option.Datadir = origDatadir }()

	cmd := &Cat{JSON: false}
	err := cmd.Execute([]string{"nonexistent"})
	if err == nil {
		t.Errorf("expected error for nonexistent file")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPut_Execute(t *testing.T) {
	tmp := t.TempDir()
	origDatadir := option.Datadir
	option.Datadir = tmp
	defer func() { option.Datadir = origDatadir }()

	// Create a temporary input file
	tmpFile := filepath.Join(tmp, "input.txt")
	if err := os.WriteFile(tmpFile, []byte("test data"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cmd := &Put{Prefix: "prefix_", Lock: "", Hash: false}
	err := cmd.Execute([]string{tmpFile})
	if err != nil {
		t.Errorf("Put.Execute() failed: %v", err)
	}

	// Verify the file was written
	ds := NewDatastore(tmp)
	var buf bytes.Buffer
	if err := ds.Read("prefix_"+tmpFile, &buf); err != nil {
		t.Errorf("Read after Put failed: %v", err)
	}
	if buf.String() != "test data" {
		t.Errorf("expected 'test data', got %q", buf.String())
	}
}

func TestPut_ExecuteWithPrefix(t *testing.T) {
	tmp := t.TempDir()
	origDatadir := option.Datadir
	option.Datadir = tmp
	defer func() { option.Datadir = origDatadir }()

	tmpFile := filepath.Join(tmp, "data.txt")
	if err := os.WriteFile(tmpFile, []byte("prefix test"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cmd := &Put{Prefix: "myprefix/", Lock: "", Hash: false}
	err := cmd.Execute([]string{tmpFile})
	if err != nil {
		t.Errorf("Put.Execute() with prefix failed: %v", err)
	}
}

func TestHistory_Execute(t *testing.T) {
	tmp := t.TempDir()
	origDatadir := option.Datadir
	option.Datadir = tmp
	defer func() { option.Datadir = origDatadir }()

	// Setup test data with multiple versions
	ds := NewDatastore(tmp)
	for i := 0; i < 3; i++ {
		reader := strings.NewReader("version " + string(rune(48+i)))
		if err := ds.Write("test", reader, []byte{}, ""); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	cmd := &History{}
	out, err := captureStdout(func() error { return cmd.Execute([]string{"test"}) })
	if err != nil {
		t.Fatalf("History.Execute() failed: %v", err)
	}
	// Should contain file name and version info
	if !strings.Contains(out, "test") {
		t.Errorf("expected 'test' in output, got: %q", out)
	}
}

func TestPrune_Execute(t *testing.T) {
	tmp := t.TempDir()
	origDatadir := option.Datadir
	option.Datadir = tmp
	defer func() { option.Datadir = origDatadir }()

	// Setup test data with multiple versions
	ds := NewDatastore(tmp)
	for i := 0; i < 5; i++ {
		reader := strings.NewReader("version " + string(rune(48+i)))
		if err := ds.Write("test", reader, []byte{}, ""); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	cmd := &Prune{Keep: 2, Dry: false, All: false}
	err := cmd.Execute([]string{"test"})
	if err != nil {
		t.Errorf("Prune.Execute() failed: %v", err)
	}

	// Verify pruning
	hist := ds.History("test")
	if len(hist) > 3 { // current + keep
		t.Errorf("expected <= 3 versions after prune, got %d", len(hist))
		t.Logf("history: %+v", hist)
	}
}

func TestPrune_DryRun(t *testing.T) {
	tmp := t.TempDir()
	origDatadir := option.Datadir
	option.Datadir = tmp
	defer func() { option.Datadir = origDatadir }()

	// Setup test data
	ds := NewDatastore(tmp)
	for i := 0; i < 3; i++ {
		reader := strings.NewReader("version " + string(rune(48+i)))
		if err := ds.Write("test", reader, []byte{}, ""); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	hist := ds.History("test")
	originalCount := len(hist)

	cmd := &Prune{Keep: 1, Dry: true, All: false}
	err := cmd.Execute([]string{"test"})
	if err != nil {
		t.Errorf("Prune.Execute(dry) failed: %v", err)
	}

	// Verify nothing was deleted
	hist = ds.History("test")
	if len(hist) != originalCount {
		t.Errorf("expected %d versions after dry-run, got %d", originalCount, len(hist))
	}
}

func TestHistoryCat_Execute(t *testing.T) {
	tmp := t.TempDir()
	origDatadir := option.Datadir
	option.Datadir = tmp
	defer func() { option.Datadir = origDatadir }()

	// Setup test data
	ds := NewDatastore(tmp)
	content := "historical content"
	reader := strings.NewReader(content)
	if err := ds.Write("test", reader, []byte{}, ""); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Get the history name
	hist := ds.History("test")
	if len(hist) == 0 {
		t.Fatalf("no history found")
	}
	historyName := hist[0].Name

	cmd := &HistoryCat{File: "test"}
	out, err := captureStdout(func() error { return cmd.Execute([]string{historyName}) })
	if err != nil {
		t.Fatalf("HistoryCat.Execute() failed: %v", err)
	}
	if out != content {
		t.Errorf("expected %q, got %q", content, out)
	}
}

func TestHistoryRollback_Execute(t *testing.T) {
	tmp := t.TempDir()
	origDatadir := option.Datadir
	option.Datadir = tmp
	defer func() { option.Datadir = origDatadir }()

	// Setup test data with multiple versions
	ds := NewDatastore(tmp)

	// Write version 1
	reader := strings.NewReader("version1")
	if err := ds.Write("test", reader, []byte{}, ""); err != nil {
		t.Fatalf("Write version1 failed: %v", err)
	}

	hist := ds.History("test")
	if len(hist) == 0 {
		t.Fatalf("no history found")
	}
	version1 := hist[0].Name

	// Write version 2
	reader = strings.NewReader("version2")
	if err := ds.Write("test", reader, []byte{}, ""); err != nil {
		t.Fatalf("Write version2 failed: %v", err)
	}

	// Rollback to version 1
	cmd := &HistoryRollback{File: "test", History: version1}
	err := cmd.Execute([]string{})
	if err != nil {
		t.Errorf("HistoryRollback.Execute() failed: %v", err)
	}

	// Verify rollback
	var buf bytes.Buffer
	if err := ds.Read("test", &buf); err != nil {
		t.Errorf("Read after rollback failed: %v", err)
	}
	if buf.String() != "version1" {
		t.Errorf("expected 'version1', got %q", buf.String())
	}
}

func TestCat_ExecuteJSON_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	origDatadir := option.Datadir
	option.Datadir = tmp
	defer func() { option.Datadir = origDatadir }()

	// Setup test data with invalid JSON
	ds := NewDatastore(tmp)
	content := `not valid json`
	reader := strings.NewReader(content)
	if err := ds.Write("test", reader, []byte{}, ""); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Test JSON read with invalid JSON (should handle gracefully)
	cmd := &Cat{JSON: true}
	// This may or may not error depending on implementation, but should not panic
	_ = cmd.Execute([]string{"test"})
}

func TestPrune_All(t *testing.T) {
	tmp := t.TempDir()
	origDatadir := option.Datadir
	option.Datadir = tmp
	defer func() { option.Datadir = origDatadir }()

	// Setup test data in multiple files
	ds := NewDatastore(tmp)
	for i := 0; i < 2; i++ {
		fname := "file" + string(rune(49+i))
		for j := 0; j < 3; j++ {
			reader := strings.NewReader("v" + string(rune(49+j)))
			if err := ds.Write(fname, reader, []byte{}, ""); err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}
	}

	cmd := &Prune{Keep: 1, Dry: false, All: true}
	err := cmd.Execute([]string{})
	if err != nil {
		t.Errorf("Prune.Execute(all) failed: %v", err)
	}
}
