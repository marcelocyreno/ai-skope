package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/provider"
	"github.com/ai-skope/aiss/internal/runtime"
	"github.com/ai-skope/aiss/internal/store"
	"github.com/ai-skope/aiss/internal/version"
)

// health is the only endpoint the extension can call before pairing. The
// model chip's dot is driven by this.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"version":    version.Version,
		"apiVersion": version.APIVersion,
		"serverId":   s.DB.ServerID(),
		"uptimeMs":   time.Since(s.Started).Milliseconds(),
		"paired":     s.DB.HasPairings(),
	})
}

// pair exchanges a one-time code, shown by `aiss pair`, for a bearer token
// bound to the calling extension's origin.
func (s *Server) pair(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code   string `json:"code"`
		Origin string `json:"origin"`
		Label  string `json:"label"`
	}
	if !decode(w, r, &in) {
		return
	}
	origin := trimAll(in.Origin, r.Header.Get("Origin"))
	if origin == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "The pairing request must say which origin it is for.")
		return
	}
	token, p, err := s.DB.RedeemPairCode(in.Code, origin, in.Label)
	if err == store.ErrBadCode {
		writeErr(w, http.StatusForbidden, "bad_code",
			"That pairing code is wrong or has expired. Run `aiss pair` for a new one.")
		return
	}
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token, "serverId": s.DB.ServerID(), "pairing": p,
	})
}

// capabilities tells the extension what this server supports, so a newer
// extension can degrade against an older server.
func (s *Server) capabilities(w http.ResponseWriter, r *http.Request) {
	kinds := make([]map[string]any, 0, len(provider.Catalog))
	for _, k := range provider.Catalog {
		kinds = append(kinds, map[string]any{
			"id": k.ID, "name": k.Name, "needsKey": k.NeedsKey, "baseUrl": k.BaseURL,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"apiVersion":      version.APIVersion,
		"version":         version.Version,
		"features":        []string{"chat", "files", "notes", "providers", "runtimes", "events"},
		"providerKinds":   kinds,
		"maxFileBytes":    s.Cfg.MaxFileBytes,
		"maxContextBytes": s.Cfg.MaxContextBytes,
		"fullTextSearch":  s.DB.HasFTS(),
		"keystore":        s.Providers.KeystoreBackend(),
	})
}

// events is the server-sent event stream: runtime health, index progress,
// folder changes.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "internal", "Streaming is not supported here.")
		return
	}
	ch, cancel := s.Bus.Subscribe()
	defer cancel()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 2000\n\n")
	flusher.Flush()

	// An immediate snapshot means a reconnecting client is never blank.
	writeSSE(w, "runtime.status", s.Runtimes.List(r.Context()))
	flusher.Flush()

	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, ev.Type, ev.Data)
			flusher.Flush()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
}

// ---- runtimes ----

func (s *Server) listRuntimes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"runtimes": s.Runtimes.List(r.Context())})
}

func (s *Server) detectRuntimes(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxWithTimeout(r, 60*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, map[string]any{"runtimes": s.Runtimes.Detect(ctx)})
}

func (s *Server) patchRuntime(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled *bool   `json:"enabled"`
		Command *string `json:"command"`
	}
	if !decode(w, r, &in) {
		return
	}
	id := r.PathValue("id")
	current, _ := s.Runtimes.Info(id)
	enabled := current.Enabled
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	command := ""
	if in.Command != nil {
		command = *in.Command
	}
	ctx, cancel := ctxWithTimeout(r, 60*time.Second)
	defer cancel()
	if err := s.Runtimes.SetEnabled(ctx, id, enabled, command); err != nil {
		writeStoreErr(w, err)
		return
	}
	info, _ := s.Runtimes.Info(id)
	writeJSON(w, http.StatusOK, info)
}

// ---- models ----

func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"models":  s.Runtimes.Models(r.Context()),
		"default": s.Runtimes.Default(r.Context()),
	})
}

func (s *Server) putDefaultModel(w http.ResponseWriter, r *http.Request) {
	var sel runtime.Selection
	if !decode(w, r, &sel) {
		return
	}
	if sel.Runtime == "" || sel.Model == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "A default needs a runtime and a model.")
		return
	}
	if err := s.Runtimes.SetDefault(sel); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sel)
}

// ---- providers ----

func (s *Server) listProviders(w http.ResponseWriter, r *http.Request) {
	list, err := s.Providers.List()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.Provider{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": list})
}

func (s *Server) createProvider(w http.ResponseWriter, r *http.Request) {
	var in provider.Input
	if !decode(w, r, &in) {
		return
	}
	ctx, cancel := ctxWithTimeout(r, 30*time.Second)
	defer cancel()
	p, err := s.Providers.Create(ctx, in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "provider_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) patchProvider(w http.ResponseWriter, r *http.Request) {
	var in provider.Input
	if !decode(w, r, &in) {
		return
	}
	ctx, cancel := ctxWithTimeout(r, 30*time.Second)
	defer cancel()
	p, err := s.Providers.Update(ctx, r.PathValue("id"), in)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deleteProvider(w http.ResponseWriter, r *http.Request) {
	if err := s.Providers.Delete(r.PathValue("id")); err != nil {
		writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) testProvider(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxWithTimeout(r, 30*time.Second)
	defer cancel()
	models, err := s.Providers.Test(ctx, r.PathValue("id"))
	if err == store.ErrNotFound {
		writeStoreErr(w, err)
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "models": models,
		"message": fmt.Sprintf("%d models", len(models)),
	})
}

// ---- settings and logs ----

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	all, err := s.DB.AllSettings()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"settings": all,
		"server": map[string]any{
			"version":     version.String(),
			"baseUrl":     s.Cfg.BaseURL(),
			"dbPath":      s.DB.Path(),
			"logPath":     config.LogFile(),
			"keystore":    s.Providers.KeystoreBackend(),
			"fullText":    s.DB.HasFTS(),
			"uptimeMs":    time.Since(s.Started).Milliseconds(),
			"passthrough": s.Cfg.PassthroughEnv,
		},
	})
}

func (s *Server) patchSettings(w http.ResponseWriter, r *http.Request) {
	var in map[string]string
	if !decode(w, r, &in) {
		return
	}
	for k, v := range in {
		if err := s.DB.SetSetting(k, v); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	all, _ := s.DB.AllSettings()
	writeJSON(w, http.StatusOK, map[string]any{"settings": all})
}

// getLogs returns the tail of the log file for the settings page.
func (s *Server) getLogs(w http.ResponseWriter, r *http.Request) {
	n := intParam(r, "tail", 200)
	if n <= 0 || n > 2000 {
		n = 200
	}
	b, err := os.ReadFile(config.LogFile())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"lines": []string{}, "path": config.LogFile()})
		return
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	writeJSON(w, http.StatusOK, map[string]any{"lines": lines, "path": config.LogFile()})
}
