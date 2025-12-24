package main

import (
	"crypto/md5"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockDS struct {
	readBody    string
	readErr     error
	deleteErr   error
	writeErr    error
	lockErr     error
	unlockErr   error
	lastWrite   string
	lastLockArg string
}

func (m *mockDS) Read(name string, out io.Writer) error {
	if m.readErr != nil {
		return m.readErr
	}
	_, _ = out.Write([]byte(m.readBody))
	return nil
}

func (m *mockDS) Delete(name string) error { return m.deleteErr }

func (m *mockDS) Write(name string, input io.Reader, hash []byte, lockid string) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	b, _ := io.ReadAll(input)
	m.lastWrite = string(b)
	return nil
}

func (m *mockDS) Lock(name string, lockinfo string) error {
	m.lastLockArg = lockinfo
	return m.lockErr
}

func (m *mockDS) Unlock(name string, lockinfo string) error {
	m.lastLockArg = lockinfo
	return m.unlockErr
}

func (m *mockDS) History(name string) []FileEntry {
	return nil
}

func (m *mockDS) ReadHistory(name string, target string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockDS) Walk(prefix string, fn func(entry FileEntry) error) error {
	return nil
}

func TestAPIGet_Success(t *testing.T) {
	ds := &mockDS{readBody: "hello"}
	h := &APIHandler{ds: ds}

	req := httptest.NewRequest(http.MethodGet, "/api/foo", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "hello" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
	// verify md5 header
	sum := md5.Sum([]byte("hello"))
	expect := base64.StdEncoding.EncodeToString(sum[:])
	if got := rr.Header().Get("content-md5"); got != expect {
		t.Fatalf("content-md5 mismatch: %s vs %s", got, expect)
	}
}

func TestAPIGet_NotFound(t *testing.T) {
	ds := &mockDS{readErr: ErrNotFound}
	h := &APIHandler{ds: ds}
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestAPIDelete(t *testing.T) {
	ds := &mockDS{deleteErr: nil}
	h := &APIHandler{ds: ds}
	req := httptest.NewRequest(http.MethodDelete, "/api/a", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAPIPost_Write(t *testing.T) {
	body := "payload"
	sum := md5.Sum([]byte(body))
	md5b64 := base64.StdEncoding.EncodeToString(sum[:])

	ds := &mockDS{}
	h := &APIHandler{ds: ds}
	req := httptest.NewRequest(http.MethodPost, "/api/f", strings.NewReader(body))
	req.Header.Set("content-md5", md5b64)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ds.lastWrite != body {
		t.Fatalf("write not received by datastore: %q", ds.lastWrite)
	}
}

func TestAPIPost_InvalidHash(t *testing.T) {
	ds := &mockDS{writeErr: ErrInvalidHash}
	h := &APIHandler{ds: ds}
	req := httptest.NewRequest(http.MethodPost, "/api/f", strings.NewReader("x"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAPILockUnlock(t *testing.T) {
	ds := &mockDS{lockErr: nil, unlockErr: nil}
	h := &APIHandler{ds: ds}

	// LOCK
	req := httptest.NewRequest("LOCK", "/api/z", strings.NewReader("{\"ID\":\"1\"}"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for LOCK, got %d", rr.Code)
	}
	if ds.lastLockArg != "{\"ID\":\"1\"}" {
		t.Fatalf("lock arg mismatch: %q", ds.lastLockArg)
	}

	// UNLOCK
	req2 := httptest.NewRequest("UNLOCK", "/api/z", strings.NewReader("{\"ID\":\"1\"}"))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 for UNLOCK, got %d", rr2.Code)
	}
}

func TestAPILock_Conflict(t *testing.T) {
	ds := &mockDS{lockErr: ErrLocked}
	h := &APIHandler{ds: ds}
	req := httptest.NewRequest("LOCK", "/api/z", strings.NewReader("{\"ID\":\"1\"}"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 for LOCK conflict, got %d", rr.Code)
	}
}

func TestAPIUnlock_NotLocked(t *testing.T) {
	ds := &mockDS{unlockErr: ErrUnlocked}
	h := &APIHandler{ds: ds}
	req := httptest.NewRequest("UNLOCK", "/api/z", strings.NewReader("{\"ID\":\"1\"}"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 for UNLOCK not-locked, got %d", rr.Code)
	}
}

func TestAPIPost_Locked(t *testing.T) {
	ds := &mockDS{writeErr: ErrLocked}
	h := &APIHandler{ds: ds}
	req := httptest.NewRequest(http.MethodPost, "/api/f", strings.NewReader("payload"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 for POST when locked, got %d", rr.Code)
	}
}

func TestAPIDelete_NotFound(t *testing.T) {
	ds := &mockDS{deleteErr: ErrNotFound}
	h := &APIHandler{ds: ds}
	req := httptest.NewRequest(http.MethodDelete, "/api/a", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for DELETE not found, got %d", rr.Code)
	}
}

func TestAPIGet_InvalidPath(t *testing.T) {
	ds := &mockDS{readErr: ErrInvalidPath}
	h := &APIHandler{ds: ds}
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for GET invalid path, got %d", rr.Code)
	}
}
