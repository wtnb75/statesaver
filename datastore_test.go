package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewDatastore(t *testing.T) {
	ds := NewDatastore("/tmp/test")
	if ds.RootName != "/tmp/test" {
		t.Errorf("expected RootName to be '/tmp/test', got %s", ds.RootName)
	}
	if ds.RootDir == nil {
		t.Errorf("expected RootDir to not be nil")
	}
}

func TestParseJSON(t *testing.T) {
	ds := NewDatastore("/tmp/test")
	tests := []struct {
		name      string
		input     string
		expected  map[string]interface{}
		shouldErr bool
	}{
		{
			name:     "valid json",
			input:    `{"key":"value","id":123}`,
			expected: map[string]interface{}{"key": "value", "id": float64(123)},
		},
		{
			name:      "invalid json",
			input:     `{invalid json}`,
			shouldErr: true,
		},
		{
			name:     "empty json",
			input:    "{}",
			expected: map[string]interface{}{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ds.ParseJSON(test.input)
			if test.shouldErr {
				if result != nil {
					t.Errorf("expected nil for invalid json, got %v", result)
				}
			} else {
				if result == nil {
					t.Errorf("expected non-nil result")
				}
				for k, v := range test.expected {
					if result[k] != v {
						t.Errorf("expected %s=%v, got %v", k, v, result[k])
					}
				}
			}
		})
	}
}

func TestFile(t *testing.T) {
	tmp := t.TempDir()
	ds := NewDatastore(tmp)

	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "simple path",
			input:    []string{"foo"},
			expected: "foo",
		},
		{
			name:     "nested path",
			input:    []string{"dir", "subdir", "file"},
			expected: "dir/subdir/file",
		},
		{
			name:     "single element",
			input:    []string{"test"},
			expected: "test",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := ds.File(test.input...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != test.expected {
				t.Errorf("expected %s, got %s", test.expected, result)
			}
		})
	}
}

func TestTimestr(t *testing.T) {
	ds := NewDatastore("/tmp/test")
	timestr := ds.Timestr()

	_, err := time.Parse(time.RFC3339, timestr)
	if err != nil {
		t.Errorf("expected RFC3339 format, got parsing error: %v", err)
	}
}

func TestWrite(t *testing.T) {
	tmp := t.TempDir()
	ds := NewDatastore(tmp)

	content := "test content"
	hash := md5.Sum([]byte(content))

	tests := []struct {
		name          string
		filename      string
		content       string
		hash          []byte
		expectErr     bool
		expectErrType error
	}{
		{
			name:      "write with valid hash",
			filename:  "file1",
			content:   content,
			hash:      hash[:],
			expectErr: false,
		},
		{
			name:      "write without hash",
			filename:  "file2",
			content:   content,
			hash:      []byte{},
			expectErr: false,
		},
		{
			name:          "write with invalid hash",
			filename:      "file3",
			content:       content,
			hash:          []byte{0x00, 0x01, 0x02},
			expectErr:     true,
			expectErrType: ErrInvalidHash,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := strings.NewReader(test.content)
			err := ds.Write(test.filename, reader, test.hash, "")
			if test.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if test.expectErrType != nil && err != test.expectErrType {
					t.Errorf("expected %v, got %v", test.expectErrType, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestWriteAndRead(t *testing.T) {
	tmp := t.TempDir()
	ds := NewDatastore(tmp)

	filename := "myfile"
	content := "test content for read/write"

	reader := strings.NewReader(content)
	err := ds.Write(filename, reader, []byte{}, "")
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	var buf bytes.Buffer
	err = ds.Read(filename, &buf)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if buf.String() != content {
		t.Errorf("expected content %q, got %q", content, buf.String())
	}
}

func TestDelete(t *testing.T) {
	tmp := t.TempDir()
	ds := NewDatastore(tmp)

	filename := "myfile"
	content := "test content"

	reader := strings.NewReader(content)
	err := ds.Write(filename, reader, []byte{}, "")
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	err = ds.Delete(filename)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	var buf bytes.Buffer
	err = ds.Read(filename, &buf)
	if err == nil {
		t.Errorf("expected error after delete, got nil")
	}
}

func TestLockUnlock(t *testing.T) {
	tmp := t.TempDir()
	ds := NewDatastore(tmp)

	filename := "myfile"
	lockinfo := `{"ID":"lock123"}`

	err := ds.Lock(filename, lockinfo)
	if err != nil {
		t.Fatalf("lock failed: %v", err)
	}

	err = ds.Lock(filename, lockinfo)
	if err != ErrLocked {
		t.Errorf("expected ErrLocked, got %v", err)
	}

	content, err := ds.LockRead(filename)
	if err != nil {
		t.Fatalf("lockread failed: %v", err)
	}
	if content != lockinfo {
		t.Errorf("expected lockinfo %q, got %q", lockinfo, content)
	}

	err = ds.Unlock(filename, lockinfo)
	if err != nil {
		t.Fatalf("unlock failed: %v", err)
	}

	_, err = ds.LockRead(filename)
	if err != ErrUnlocked {
		t.Errorf("expected ErrUnlocked, got %v", err)
	}
}

func TestLockCheck(t *testing.T) {
	tmp := t.TempDir()
	ds := NewDatastore(tmp)

	filename := "myfile"
	lockinfo := `{"ID":"lock123"}`

	err := ds.LockCheck(filename, "any-id")
	if err != nil {
		t.Errorf("expected no error when file not locked, got %v", err)
	}

	err = ds.Lock(filename, lockinfo)
	if err != nil {
		t.Fatalf("lock failed: %v", err)
	}

	err = ds.LockCheck(filename, "lock123")
	if err != nil {
		t.Errorf("expected no error with correct ID, got %v", err)
	}

	err = ds.LockCheck(filename, "wrong-id")
	if err != ErrLocked {
		t.Errorf("expected ErrLocked with wrong ID, got %v", err)
	}
}

func TestHistory(t *testing.T) {
	tmp := t.TempDir()
	ds := NewDatastore(tmp)

	filename := "myfile"

	for i := 0; i < 3; i++ {
		content := "version " + string(rune(48+i))
		reader := strings.NewReader(content)
		err := ds.Write(filename, reader, []byte{}, "")
		if err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	hist := ds.History(filename)
	if len(hist) < 1 {
		t.Errorf("expected at least 1 history entry, got %d", len(hist))
	}

	for i := 0; i < len(hist)-1; i++ {
		if hist[i].Timestamp.Before(hist[i+1].Timestamp) {
			t.Errorf("history not sorted by timestamp descending")
		}
	}
}

func TestRollback(t *testing.T) {
	tmp := t.TempDir()
	ds := NewDatastore(tmp)

	filename := "myfile"

	reader1 := strings.NewReader("version1")
	err := ds.Write(filename, reader1, []byte{}, "")
	if err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	hist := ds.History(filename)
	if len(hist) == 0 {
		t.Fatalf("no history found")
	}
	firstVersion := hist[0].Name

	reader2 := strings.NewReader("version2")
	err = ds.Write(filename, reader2, []byte{}, "")
	if err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	err = ds.Rollback(filename, firstVersion)
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	var buf bytes.Buffer
	err = ds.Read(filename, &buf)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if buf.String() != "version1" {
		t.Errorf("expected 'version1', got %q", buf.String())
	}
}

func TestPrune(t *testing.T) {
	tmp := t.TempDir()
	ds := NewDatastore(tmp)

	filename := "myfile"

	for i := 0; i < 5; i++ {
		content := "version" + string(rune(48+i))
		reader := strings.NewReader(content)
		err := ds.Write(filename, reader, []byte{}, "")
		if err != nil {
			t.Fatalf("write failed: %v", err)
		}
		time.Sleep(1 * time.Second)
	}

	hist := ds.History(filename)
	if len(hist) < 5 {
		t.Errorf("expected at least 5 versions, got %d", len(hist))
	}

	err := ds.Prune(filename, 2, false)
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}

	hist = ds.History(filename)
	if len(hist) > 2 {
		t.Errorf("expected 2 or fewer versions after prune, got %d", len(hist))
		t.Logf("history: %+v", hist)
	}
}

func TestPruneDry(t *testing.T) {
	tmp := t.TempDir()
	ds := NewDatastore(tmp)

	filename := "myfile"

	for i := 0; i < 3; i++ {
		content := "version" + string(rune(48+i))
		reader := strings.NewReader(content)
		err := ds.Write(filename, reader, []byte{}, "")
		if err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	hist := ds.History(filename)
	originalCount := len(hist)

	err := ds.Prune(filename, 1, true)
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}

	hist = ds.History(filename)
	if len(hist) != originalCount {
		t.Errorf("expected %d versions after dry-run, got %d", originalCount, len(hist))
	}
}

func TestReadNonExistent(t *testing.T) {
	tmp := t.TempDir()
	ds := NewDatastore(tmp)

	filename := "nonexistent"
	var buf bytes.Buffer
	err := ds.Read(filename, &buf)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestReadHistory(t *testing.T) {
	tmp := t.TempDir()
	ds := NewDatastore(tmp)

	filename := "myfile"
	content := "historical content"

	reader := strings.NewReader(content)
	err := ds.Write(filename, reader, []byte{}, "")
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	hist := ds.History(filename)
	if len(hist) == 0 {
		t.Fatalf("no history found")
	}
	historyName := hist[0].Name

	rc, err := ds.ReadHistory(filename, historyName)
	if err != nil {
		t.Fatalf("read history failed: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read all failed: %v", err)
	}

	if string(data) != content {
		t.Errorf("expected content %q, got %q", content, string(data))
	}
}

func TestWriteWithLock(t *testing.T) {
	tmp := t.TempDir()
	ds := NewDatastore(tmp)

	filename := "myfile"
	lockID := "lock123"
	lockinfo := map[string]interface{}{"ID": lockID}
	lockinfoByte, _ := json.Marshal(lockinfo)

	err := ds.Lock(filename, string(lockinfoByte))
	if err != nil {
		t.Fatalf("lock failed: %v", err)
	}

	reader := strings.NewReader("content")
	err = ds.Write(filename, reader, []byte{}, "wrong-id")
	if err != ErrLocked {
		t.Errorf("expected ErrLocked, got %v", err)
	}

	reader = strings.NewReader("content")
	err = ds.Write(filename, reader, []byte{}, lockID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWalk(t *testing.T) {
	tmp := t.TempDir()
	ds := NewDatastore(tmp)

	// entry1: has a version file, current symlink, and lock file (locked)
	entry1Dir := filepath.Join(tmp, "entry1")
	if err := os.MkdirAll(entry1Dir, 0o755); err != nil {
		t.Fatalf("mkdir entry1 failed: %v", err)
	}
	v1 := "v1"
	if err := os.WriteFile(filepath.Join(entry1Dir, v1), []byte("data1"), 0o644); err != nil {
		t.Fatalf("write v1 failed: %v", err)
	}
	if err := os.Symlink(v1, filepath.Join(entry1Dir, "current")); err != nil {
		t.Fatalf("symlink failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(entry1Dir, "lock"), []byte(`{"ID":"abc"}`), 0o644); err != nil {
		t.Fatalf("write lock failed: %v", err)
	}

	// entry2: has a version file and current symlink, no lock (unlocked)
	entry2Dir := filepath.Join(tmp, "entry2")
	if err := os.MkdirAll(entry2Dir, 0o755); err != nil {
		t.Fatalf("mkdir entry2 failed: %v", err)
	}
	v2 := "v2"
	if err := os.WriteFile(filepath.Join(entry2Dir, v2), []byte("data2"), 0o644); err != nil {
		t.Fatalf("write v2 failed: %v", err)
	}
	if err := os.Symlink(v2, filepath.Join(entry2Dir, "current")); err != nil {
		t.Fatalf("symlink failed: %v", err)
	}

	var entries []FileEntry
	if err := ds.Walk(func(e FileEntry) error {
		entries = append(entries, e)
		return nil
	}); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if len(entries) != 2 {
		// try to show directory listing for debugging
		files, _ := os.ReadDir(tmp)
		t.Fatalf("expected 2 entries, got %d (dirs: %+v)", len(entries), files)
	}

	// Normalize names (strip leading slash if present)
	byName := map[string]FileEntry{}
	for _, e := range entries {
		name := e.Name
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		byName[name] = e
	}

	e1, ok := byName["entry1"]
	if !ok {
		t.Fatalf("entry1 not found in walk results: %+v", entries)
	}
	if !e1.Locked {
		t.Errorf("expected entry1 to be locked")
	}
	if e1.Size == 0 {
		t.Errorf("expected entry1 size > 0")
	}

	e2, ok := byName["entry2"]
	if !ok {
		t.Fatalf("entry2 not found in walk results: %+v", entries)
	}
	if e2.Locked {
		t.Errorf("expected entry2 to be unlocked")
	}
	if e2.Size == 0 {
		t.Errorf("expected entry2 size > 0")
	}
}
