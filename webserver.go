package main

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/sprig/v3"
)

// APIHandler serves API requests for terraform state backends
type APIHandler struct {
	ds DsIf
}

// APIGet handles GET requests to retrieve file contents
func (h *APIHandler) APIGet(path string, w io.Writer, r *http.Request) error {
	hist := r.URL.Query().Get("history")
	if hist == "" {
		return h.ds.Read(path, w)
	}
	if ior, err := h.ds.ReadHistory(path, hist); err != nil {
		slog.Error("cannot read history", "error", err, "path", path, "history", hist)
		return err
	} else {
		defer ior.Close()
		_, err = io.Copy(w, ior)
		return err
	}
}

// APIDelete handles DELETE requests to remove files
func (h *APIHandler) APIDelete(path string, w io.Writer, r *http.Request) error {
	return h.ds.Delete(path)
}

// APIPost handles POST requests to write file contents
func (h *APIHandler) APIPost(path string, w io.Writer, r *http.Request) error {
	hashb, err0 := base64.StdEncoding.DecodeString(r.Header.Get("content-md5"))
	if err0 != nil {
		hashb = []byte{}
	}
	lockid := r.URL.Query().Get("ID")
	return h.ds.Write(path, r.Body, hashb, lockid)
}

// APILock handles LOCK requests to lock a file
func (h *APIHandler) APILock(path string, w io.Writer, r *http.Request) error {
	body, err0 := io.ReadAll(r.Body)
	if err0 != nil {
		slog.Error("read body", "error", err0, "url", r.URL)
	}
	slog.Debug("lock", "content", string(body))
	return h.ds.Lock(path, string(body))
}

// APIUnlock handles UNLOCK requests to unlock a file
func (h *APIHandler) APIUnlock(path string, w io.Writer, r *http.Request) error {
	body, err0 := io.ReadAll(r.Body)
	if err0 != nil {
		slog.Error("read body", "error", err0, "url", r.URL)
	}
	slog.Debug("unlock", "content", string(body))
	return h.ds.Unlock(path, string(body))
}

// ServeHTTP routes HTTP requests to the appropriate API handler methods
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

// HTMLHandler serves HTML pages for the web interface
type HTMLHandler struct {
	ds   DsIf
	fmap template.FuncMap
}

// Index serves the index page listing all files
func (h *HTMLHandler) Index(path string, w io.Writer, r *http.Request) error {
	tmpl, err := template.New("list.html").Funcs(h.fmap).ParseFS(template_files, "templates/list.html")
	if err != nil {
		slog.Error("template load failed", "path", path, "error", err)
		return err
	}
	files := make([]FileEntry, 0)
	h.ds.Walk(func(e FileEntry) error {
		files = append(files, e)
		return nil
	})
	entries := make(map[string]interface{})
	entries["Files"] = files
	entries["Title"] = "index"
	slog.Debug("entries", "files", files)

	if err := tmpl.Execute(w, entries); err != nil {
		slog.Error("template execute failed", "path", path, "error", err)
		return err
	}
	return nil
}

// Resource serves static resources like CSS and JS files
func (h *HTMLHandler) Resource(path string, w io.Writer, r *http.Request) error {
	buf, err := template_files.ReadFile(filepath.Join("templates", path))
	if err != nil {
		slog.Error("no such assets", "path", path, "error", err)
		return err
	}
	_, err = w.Write(buf)
	return err
}

// ViewFile serves the detailed view of a specific file
func (h *HTMLHandler) ViewFile(name string, w io.Writer, r *http.Request) error {
	tmpl, err := template.New("view.html").Funcs(h.fmap).ParseFS(template_files, "templates/view.html")
	if err != nil {
		slog.Error("template load failed", "name", name, "error", err)
		return err
	}
	historyfiles := h.ds.History(name)
	buf := &bytes.Buffer{}
	target := r.URL.Query().Get("history")
	slog.Debug("reading target", "history", target)
	if target != "" {
		rdc, err := h.ds.ReadHistory(name, target)
		if err != nil {
			slog.Error("cannot read history", "name", name, "target", target, "error", err)
			return err
		}
		defer rdc.Close()
		if _, err := io.Copy(buf, rdc); err != nil {
			slog.Error("read history", "name", name, "target", target, "error", err)
			return err
		}
	} else {
		if err := h.ds.Read(name, buf); err != nil {
			slog.Error("read failes", "name", name, "error", err)
			return err
		}
	}
	target_data := make(map[string]interface{})
	if err := json.Unmarshal(buf.Bytes(), &target_data); err != nil {
		slog.Error("json decode", "name", name, "error", err)
		return err
	}
	data := make(map[string]interface{})
	data["name"] = target
	data["file"] = name
	data["data"] = target_data
	data["history"] = historyfiles
	data["Title"] = name
	if err := tmpl.Execute(w, data); err != nil {
		slog.Error("template", "name", name, "error", err)
		return err
	}
	return nil
}

// ServeHTTP routes HTTP requests to the appropriate HTML handler methods
func (h *HTMLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	st := time.Now()
	slog.Info("access", "method", r.Method, "path", r.URL.Path, "params", r.URL.Query(), "headers", r.Header)
	var err error
	buf := &bytes.Buffer{}
	path := strings.TrimPrefix(r.URL.Path, "/html/")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if path == "" {
		err = h.Index(path, buf, r)
	} else if strings.HasPrefix(path, "view/") {
		name := strings.TrimPrefix(path, "view/")
		err = h.ViewFile(name, buf, r)
	} else {
		err = h.Resource(path, buf, r)
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

// WebServer represents the web server command
type WebServer struct {
	Listen        string `short:"l" long:"listen" default:":3000" env:"STSV_LISTEN" description:"listen address"`
	Auth          string `short:"u" long:"user" description:"basic auth username:password"`
	OpenTelemetry bool   `long:"opentelemetry"`
	server        *http.ServeMux
	apihandler    *APIHandler
	htmlhandler   *HTMLHandler
}

func mytime(ts *time.Time) template.HTML {
	duration := strings.TrimSuffix(time.Since(*ts).Truncate(time.Minute).String(), "0s")
	if duration == "" {
		duration = "just now"
	}
	return template.HTML(fmt.Sprintf("<abbr title=\"%s\" class=\"default\">%s</abbr>", ts.Format(time.RFC3339), duration))
}

func (cmd *WebServer) Execute(args []string) error {
	init_log()
	cmd.server = http.NewServeMux()
	d := NewDatastore(option.Datadir)
	cmd.apihandler = &APIHandler{
		ds: &d,
	}
	cmd.htmlhandler = &HTMLHandler{
		ds:   &d,
		fmap: sprig.FuncMap(),
	}
	cmd.htmlhandler.fmap["mytime"] = mytime
	cmd.server.Handle("/api/", cmd.apihandler)
	cmd.server.Handle("/html/", cmd.htmlhandler)
	slog.Info("starting server", "address", cmd.Listen)
	return http.ListenAndServe(cmd.Listen, cmd.server)
}
