package main

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type APIHandler struct {
	ds interface {
		Read(name string, out io.Writer) error
		Delete(name string) error
		Write(name string, input io.Reader, hash []byte, lockid string) error
		Lock(name string, lockinfo string) error
		Unlock(name string, lockinfo string) error
	}
}

func (h *APIHandler) APIGet(path string, w io.Writer, r *http.Request) error {
	return h.ds.Read(path, w)
}

func (h *APIHandler) APIDelete(path string, w io.Writer, r *http.Request) error {
	return h.ds.Delete(path)
}

func (h *APIHandler) APIPost(path string, w io.Writer, r *http.Request) error {
	hashb, err0 := base64.StdEncoding.DecodeString(r.Header.Get("content-md5"))
	if err0 != nil {
		hashb = []byte{}
	}
	lockid := r.URL.Query().Get("ID")
	return h.ds.Write(path, r.Body, hashb, lockid)
}

func (h *APIHandler) APILock(path string, w io.Writer, r *http.Request) error {
	body, err0 := io.ReadAll(r.Body)
	if err0 != nil {
		slog.Error("read body", "error", err0, "url", r.URL)
	}
	slog.Debug("lock", "content", string(body))
	return h.ds.Lock(path, string(body))
}

func (h *APIHandler) APIUnlock(path string, w io.Writer, r *http.Request) error {
	body, err0 := io.ReadAll(r.Body)
	if err0 != nil {
		slog.Error("read body", "error", err0, "url", r.URL)
	}
	slog.Debug("unlock", "content", string(body))
	return h.ds.Unlock(path, string(body))
}

func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	st := time.Now()
	slog.Info("access", "method", r.Method, "path", r.URL.Path, "params", r.URL.Query(), "headers", r.Header)
	var err error
	buf := &bytes.Buffer{}
	path := strings.TrimPrefix(r.URL.Path, "/api/")
	switch r.Method {
	case http.MethodGet:
		err = h.APIGet(path, buf, r)
	case http.MethodDelete:
		err = h.APIDelete(path, buf, r)
	case http.MethodPost:
		err = h.APIPost(path, buf, r)
	case "LOCK":
		err = h.APILock(path, buf, r)
	case "UNLOCK":
		err = h.APIUnlock(path, buf, r)
	}
	w.Header().Add("Content-Length", strconv.Itoa(buf.Len()))
	md5sum := md5.Sum(buf.Bytes())
	w.Header().Add("Content-Md5", base64.StdEncoding.EncodeToString(md5sum[:]))
	var statuscode int
	switch err {
	case nil:
		statuscode = http.StatusOK
	case ErrLocked:
		statuscode = http.StatusConflict
	case ErrUnlocked:
		statuscode = http.StatusConflict
	case ErrInvalidPath:
		statuscode = http.StatusBadRequest
	case ErrInvalidHash:
		statuscode = http.StatusBadRequest
	case ErrNotFound:
		statuscode = http.StatusNotFound
	default:
		statuscode = http.StatusInternalServerError
	}
	w.WriteHeader(statuscode)
	written, err1 := io.Copy(w, buf)
	if err1 != nil {
		slog.Warn("write response", "written", written, "error", err1, "path", path)
	}
	elapsed := time.Since(st)
	slog.Info("response", "status", http.StatusText(statuscode), "method", r.Method, "path", r.URL.Path, "elapsed", elapsed)
}

type WebServer struct {
	Listen        string `short:"l" long:"listen" default:":3000" env:"STSV_LISTEN" description:"listen address"`
	Auth          string `short:"u" long:"user" description:"basic auth username:password"`
	OpenTelemetry bool   `long:"opentelemetry"`
	server        *http.ServeMux
	apihandler    *APIHandler
}

func (cmd *WebServer) Execute(args []string) error {
	init_log()
	cmd.server = http.NewServeMux()
	d := NewDatastore(option.Datadir)
	cmd.apihandler = &APIHandler{
		ds: &d,
	}
	cmd.server.Handle("/api/", cmd.apihandler)
	slog.Info("starting server", "address", cmd.Listen)
	return http.ListenAndServe(cmd.Listen, cmd.server)
}
