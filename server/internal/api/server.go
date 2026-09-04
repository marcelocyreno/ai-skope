// Package api serves the HTTP and SSE interface the extension talks to.
//
// Everything is JSON. Errors are {"error":{"code":…,"message":…}} so the
// extension can show something specific rather than a status code.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/ai-skope/aiss/internal/chat"
	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/files"
	"github.com/ai-skope/aiss/internal/provider"
	"github.com/ai-skope/aiss/internal/runtime"
	"github.com/ai-skope/aiss/internal/status"
	"github.com/ai-skope/aiss/internal/store"
)

// Deps are everything the API needs; the CLI wires them once.
type Deps struct {
	DB        *store.DB
	Cfg       config.Config
	Guard     *files.Guard
	Indexer   *files.Indexer
	Watcher   *files.Watcher
	Providers *provider.Registry
	Runtimes  *runtime.Registry
	Chat      *chat.Service
	Bus       *status.Bus
	Started   time.Time
}

// Server is the HTTP surface.
type Server struct {
	Deps
	limiter *limiter
}

// New builds the server.
func New(d Deps) *Server {
	if d.Started.IsZero() {
		d.Started = time.Now()
	}
	return &Server{Deps: d, limiter: newLimiter(30, time.Minute)}
}

// Handler returns the router with middleware applied.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Unauthenticated: the extension needs these before it holds a token.
	mux.HandleFunc("GET /v1/health", s.health)
	mux.HandleFunc("POST /v1/pair", s.pair)

	mux.Handle("GET /v1/capabilities", s.auth(s.capabilities))
	mux.Handle("GET /v1/events", s.auth(s.events))

	mux.Handle("GET /v1/runtimes", s.auth(s.listRuntimes))
	mux.Handle("POST /v1/runtimes/detect", s.auth(s.detectRuntimes))
	mux.Handle("PATCH /v1/runtimes/{id}", s.auth(s.patchRuntime))

	mux.Handle("GET /v1/models", s.auth(s.listModels))
	mux.Handle("PUT /v1/models/default", s.auth(s.putDefaultModel))

	mux.Handle("GET /v1/providers", s.auth(s.listProviders))
	mux.Handle("POST /v1/providers", s.auth(s.createProvider))
	mux.Handle("PATCH /v1/providers/{id}", s.auth(s.patchProvider))
	mux.Handle("DELETE /v1/providers/{id}", s.auth(s.deleteProvider))
	mux.Handle("POST /v1/providers/{id}/test", s.auth(s.testProvider))

	mux.Handle("GET /v1/folders", s.auth(s.listFolders))
	mux.Handle("POST /v1/folders", s.auth(s.createFolder))
	mux.Handle("PATCH /v1/folders/{id}", s.auth(s.patchFolder))
	mux.Handle("DELETE /v1/folders/{id}", s.auth(s.deleteFolder))
	mux.Handle("POST /v1/folders/{id}/reindex", s.auth(s.reindexFolder))

	mux.Handle("GET /v1/files/search", s.auth(s.searchFiles))
	mux.Handle("GET /v1/files/recent", s.auth(s.recentFiles))
	mux.Handle("GET /v1/files/browse", s.auth(s.browseFiles))
	mux.Handle("GET /v1/files/read", s.auth(s.readFile))
	mux.Handle("POST /v1/files/resolve", s.auth(s.resolveFile))

	mux.Handle("GET /v1/chats", s.auth(s.listChats))
	mux.Handle("POST /v1/chats", s.auth(s.createChat))
	mux.Handle("GET /v1/chats/{id}", s.auth(s.getChat))
	mux.Handle("PATCH /v1/chats/{id}", s.auth(s.patchChat))
	mux.Handle("DELETE /v1/chats/{id}", s.auth(s.deleteChat))
	mux.Handle("POST /v1/chats/{id}/restore", s.auth(s.restoreChat))
	mux.Handle("POST /v1/chats/{id}/messages", s.auth(s.sendMessage))
	mux.Handle("POST /v1/chats/{id}/cancel", s.auth(s.cancelChat))

	mux.Handle("GET /v1/notes", s.auth(s.listNotes))
	mux.Handle("POST /v1/notes", s.auth(s.createNote))
	mux.Handle("PATCH /v1/notes/{id}", s.auth(s.patchNote))
	mux.Handle("DELETE /v1/notes/{id}", s.auth(s.deleteNote))

	mux.Handle("GET /v1/settings", s.auth(s.getSettings))
	mux.Handle("PATCH /v1/settings", s.auth(s.patchSettings))
	mux.Handle("GET /v1/logs", s.auth(s.getLogs))

	return s.recover(s.logRequests(s.cors(s.limitBody(mux))))
}

// ---- middleware ----

func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("panic serving request", "path", r.URL.Path, "value", v,
					"stack", string(debug.Stack()))
				writeErr(w, http.StatusInternalServerError, "internal", "Something went wrong on the server.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rec := &statusRecorder{ResponseWriter: w, code: 200}
		next.ServeHTTP(rec, r)
		slog.Debug("request", "method", r.Method, "path", r.URL.Path,
			"status", rec.code, "ms", time.Since(started).Milliseconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(c int) { r.code = c; r.ResponseWriter.WriteHeader(c) }

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, s.Cfg.MaxRequestBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeErr(w http.ResponseWriter, code int, id, msg string) {
	var e apiError
	e.Error.Code = id
	e.Error.Message = msg
	writeJSON(w, code, e)
}

// writeStoreErr maps a store or guard error onto a status the extension can act on.
func writeStoreErr(w http.ResponseWriter, err error) {
	switch {
	case err == store.ErrNotFound:
		writeErr(w, http.StatusNotFound, "not_found", "That does not exist.")
	case err == files.ErrNotAllowed:
		writeErr(w, http.StatusForbidden, "folder_not_allowed",
			"That path is outside every folder you have allowed.")
	case err == files.ErrDenied:
		writeErr(w, http.StatusForbidden, "path_denied",
			"That file is on the deny list and is never read.")
	case err == files.ErrNoFolders:
		writeErr(w, http.StatusForbidden, "no_folders",
			"No folders have been allowed yet. Add one in Settings → Folders.")
	case err == files.ErrBinary:
		writeErr(w, http.StatusUnsupportedMediaType, "not_text", "That file is not text.")
	default:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "The request body could not be read: "+err.Error())
		return false
	}
	return true
}

func intParam(r *http.Request, name string, def int) int {
	if v := r.URL.Query().Get(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func ctxWithTimeout(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}

func trimAll(vals ...string) string {
	for _, v := range vals {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}
