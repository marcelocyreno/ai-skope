package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ai-skope/aiss/internal/chat"
	"github.com/ai-skope/aiss/internal/files"
	"github.com/ai-skope/aiss/internal/store"
)

// ---- chats ----

func (s *Server) listChats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := s.DB.Chats(store.ChatFilter{
		URL:   q.Get("url"),
		Host:  q.Get("host"),
		Query: q.Get("q"),
		Limit: intParam(r, "limit", 200),
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"chats": list})
}

func (s *Server) createChat(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title     string `json:"title"`
		URL       string `json:"url"`
		PageTitle string `json:"pageTitle"`
		Favicon   string `json:"favicon"`
	}
	if !decode(w, r, &in) {
		return
	}
	c, err := s.DB.CreateChat(store.Chat{
		Title: in.Title, URL: in.URL, Host: hostOf(in.URL),
		PageTitle: in.PageTitle, Favicon: in.Favicon,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) getChat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := s.DB.Chat(id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	msgs, err := s.DB.Messages(id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"chat": c, "messages": msgs, "running": s.Chat.Running(id),
	})
}

func (s *Server) patchChat(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string `json:"title"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.DB.SetChatTitle(r.PathValue("id"), in.Title); err != nil {
		writeStoreErr(w, err)
		return
	}
	c, err := s.DB.Chat(r.PathValue("id"))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// deleteChat soft-deletes, so the extension's Undo has something to restore.
func (s *Server) deleteChat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.Chat.Cancel(id)
	if err := s.DB.SoftDeleteChat(id); err != nil {
		writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) restoreChat(w http.ResponseWriter, r *http.Request) {
	if err := s.DB.RestoreChat(r.PathValue("id")); err != nil {
		writeStoreErr(w, err)
		return
	}
	c, _ := s.DB.Chat(r.PathValue("id"))
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) cancelChat(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"cancelled": s.Chat.Cancel(r.PathValue("id"))})
}

// sendMessage runs a turn and streams it back as server-sent events.
func (s *Server) sendMessage(w http.ResponseWriter, r *http.Request) {
	p := pairingOf(r)
	key := "anon"
	if p != nil {
		key = p.ID
	}
	if !s.limiter.allow(key) {
		writeErr(w, http.StatusTooManyRequests, "rate_limited",
			"Too many messages in a short time. Wait a moment and try again.")
		return
	}

	var req chat.SendRequest
	if !decode(w, r, &req) {
		return
	}
	// The page's own text is only accepted when the user allows it; the
	// extension enforces the choice, the server records what it received.
	if s.DB.Setting("privacy.pageContent", "ask") == "never" && req.Page != nil {
		req.Page.Text = ""
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "internal", "Streaming is not supported here.")
		return
	}

	stream, err := s.Chat.Send(r.Context(), r.PathValue("id"), req)
	if err != nil {
		switch {
		case err == store.ErrNotFound:
			writeStoreErr(w, err)
		case err == chat.ErrBusy:
			writeErr(w, http.StatusConflict, "busy", "This chat is already answering.")
		default:
			writeErr(w, http.StatusBadRequest, "turn_failed", err.Error())
		}
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for ev := range stream {
		b, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Event, b)
		flusher.Flush()
	}
}

// ---- notes ----

func (s *Server) listNotes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := s.DB.Notes(store.NoteFilter{
		URL: q.Get("url"), Host: q.Get("host"), Query: q.Get("q"),
		Limit: intParam(r, "limit", 200),
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notes": list})
}

func (s *Server) createNote(w http.ResponseWriter, r *http.Request) {
	var in store.Note
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Body) == "" && strings.TrimSpace(in.Quote) == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "A note needs some text or a quote.")
		return
	}
	in.Host = hostOf(in.URL)
	n, err := s.DB.CreateNote(in)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) patchNote(w http.ResponseWriter, r *http.Request) {
	var in store.Note
	if !decode(w, r, &in) {
		return
	}
	in.ID = r.PathValue("id")
	if err := s.DB.UpdateNote(in); err != nil {
		writeStoreErr(w, err)
		return
	}
	n, err := s.DB.Note(in.ID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) deleteNote(w http.ResponseWriter, r *http.Request) {
	if err := s.DB.SoftDeleteNote(r.PathValue("id")); err != nil {
		writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// hostOf groups history by site; a local file is its own "host" so the
// extension can show local-file chats together.
func hostOf(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Scheme == "file" {
		return "local file"
	}
	return u.Host
}

var _ = files.Tilde
