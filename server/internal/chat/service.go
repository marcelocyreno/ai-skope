package chat

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/files"
	"github.com/ai-skope/aiss/internal/runtime"
	"github.com/ai-skope/aiss/internal/status"
	"github.com/ai-skope/aiss/internal/store"
)

// Service runs turns and keeps the transcript.
type Service struct {
	db       *store.DB
	cfg      config.Config
	guard    *files.Guard
	runtimes *runtime.Registry
	bus      *status.Bus

	mu     sync.Mutex
	active map[string]context.CancelFunc
}

// NewService builds the chat service.
func NewService(db *store.DB, cfg config.Config, guard *files.Guard, rt *runtime.Registry, bus *status.Bus) *Service {
	return &Service{db: db, cfg: cfg, guard: guard, runtimes: rt, bus: bus,
		active: map[string]context.CancelFunc{}}
}

// PageRef is the page the question is about.
type PageRef struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Favicon string `json:"favicon"`
	Text    string `json:"text"` // sent only when the user allows page content
}

// SendRequest is one message to answer.
type SendRequest struct {
	Text    string              `json:"text"`
	Page    *PageRef            `json:"page"`
	Context []store.ContextItem `json:"context"`
	Model   *runtime.Selection  `json:"model"`
}

// Stream event names, mirrored by the SSE endpoint and the extension's UI.
const (
	EvTurnStart = "turn.start"
	EvTool      = "tool"
	EvTextDelta = "text.delta"
	EvTextDone  = "text.done"
	EvUsage     = "usage"
	EvError     = "error"
	EvTurnEnd   = "turn.end"
)

// Event is one step of a turn as the extension sees it.
type Event struct {
	Event     string            `json:"event"`
	MessageID string            `json:"messageId,omitempty"`
	Text      string            `json:"text,omitempty"`
	Tool      *store.ToolRecord `json:"tool,omitempty"`
	Usage     *store.Usage      `json:"usage,omitempty"`
	Code      string            `json:"code,omitempty"`
	Message   string            `json:"message,omitempty"`
	Retryable bool              `json:"retryable,omitempty"`
	Model     string            `json:"model,omitempty"`
}

// ErrBusy is returned when a chat already has a turn running.
var ErrBusy = errors.New("this chat is already answering")

// Send persists the user's message, starts the turn, and returns the stream.
// The returned channel is closed when the turn ends.
func (s *Service) Send(ctx context.Context, chatID string, req SendRequest) (<-chan Event, error) {
	if strings.TrimSpace(req.Text) == "" {
		return nil, fmt.Errorf("the message is empty")
	}
	chat, err := s.db.Chat(chatID)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	if _, running := s.active[chatID]; running {
		s.mu.Unlock()
		return nil, ErrBusy
	}
	s.mu.Unlock()

	sel := s.resolveSelection(ctx, chat, req)
	if sel.Runtime == "" {
		return nil, fmt.Errorf("no runtime is available — install one, or switch to API keys")
	}

	// Remember the page and the model on the chat, and title it from the
	// first thing the user asked.
	if req.Page != nil {
		chat.URL, chat.PageTitle, chat.Favicon = req.Page.URL, req.Page.Title, req.Page.Favicon
		chat.Host = hostOf(req.Page.URL)
	}
	if chat.Title == "" {
		chat.Title = titleFrom(req.Text)
	}
	chat.Runtime, chat.Provider, chat.Model, chat.Effort = sel.Runtime, sel.Provider, sel.Model, sel.Effort
	if err := s.db.UpdateChat(chat); err != nil {
		return nil, err
	}

	userMsg, err := s.db.AddMessage(store.Message{
		ChatID: chatID, Role: "user", Text: req.Text, Context: req.Context,
	})
	if err != nil {
		return nil, err
	}

	var pageItem *store.ContextItem
	if req.Page != nil && req.Page.Text != "" {
		pageItem = &store.ContextItem{
			Type: store.ContextPage, URL: req.Page.URL, Title: req.Page.Title, Text: req.Page.Text,
		}
	}
	packed := Pack(s.guard, req.Text, pageItem, req.Context, s.cfg.MaxContextBytes)

	workDir, err := s.guard.PrimaryRoot(packed.FilePaths)
	if err != nil {
		// With no allowed folder the agent still needs somewhere to run.
		workDir = "."
	}
	for _, p := range packed.FilePaths {
		_ = s.db.TouchRecentFile(p)
	}

	turnCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.mu.Lock()
	s.active[chatID] = cancel
	s.mu.Unlock()

	release := func() {
		s.mu.Lock()
		delete(s.active, chatID)
		s.mu.Unlock()
		cancel()
	}

	turn, err := s.runtimes.Start(turnCtx, sel.Runtime, runtime.TurnRequest{
		Prompt:    packed.Prompt,
		Model:     sel.Model,
		Provider:  sel.Provider,
		Effort:    sel.Effort,
		SessionID: chat.AgentSession,
		WorkDir:   workDir,
		Timeout:   s.cfg.TurnTimeout.D(),
	})
	if err != nil {
		release()
		// Record the failure in the transcript so the user sees why.
		_, _ = s.db.AddMessage(store.Message{
			ChatID: chatID, Role: "assistant", Error: err.Error(), Model: sel.Label(),
		})
		return nil, err
	}

	out := make(chan Event, 64)
	go s.pump(chat, sel, userMsg, turn, out, release)
	return out, nil
}

// pump forwards the runtime's events, accumulating the assistant message so it
// can be persisted when the turn ends.
func (s *Service) pump(chat store.Chat, sel runtime.Selection, userMsg store.Message,
	turn runtime.Turn, out chan<- Event, release func()) {

	defer close(out)
	defer release()

	msg := store.Message{
		ID: store.NewID(), ChatID: chat.ID, Role: "assistant",
		Model: sel.Label(), CreatedAt: store.Now(),
	}
	var text strings.Builder
	started := time.Now()

	send := func(ev Event) {
		ev.MessageID = msg.ID
		select {
		case out <- ev:
		case <-time.After(5 * time.Second):
			// The client vanished; the transcript is still persisted below.
		}
	}

	send(Event{Event: EvTurnStart, Model: sel.Label()})

	for ev := range turn.Events() {
		switch ev.Kind {
		case runtime.EventSession:
			if ev.SessionID != "" && ev.SessionID != chat.AgentSession {
				chat.AgentSession = ev.SessionID
				_ = s.db.SetChatAgentSession(chat.ID, ev.SessionID)
			}
		case runtime.EventText:
			text.WriteString(ev.Text)
			send(Event{Event: EvTextDelta, Text: ev.Text})
		case runtime.EventTool:
			if ev.Tool != nil {
				msg.Tools = appendTool(msg.Tools, *ev.Tool)
				send(Event{Event: EvTool, Tool: ev.Tool})
			}
		case runtime.EventUsage:
			if ev.Usage != nil {
				if msg.Usage == nil {
					msg.Usage = &store.Usage{}
				}
				if ev.Usage.InputTokens > 0 {
					msg.Usage.InputTokens = ev.Usage.InputTokens
				}
				if ev.Usage.OutputTokens > 0 {
					msg.Usage.OutputTokens = ev.Usage.OutputTokens
				}
				if ev.Usage.MS > 0 {
					msg.Usage.MS = ev.Usage.MS
				}
			}
		case runtime.EventError:
			msg.Error = ev.Err
			send(Event{Event: EvError, Code: "runtime_error", Message: ev.Err, Retryable: ev.Retryable})
		}
	}

	msg.Text = strings.TrimSpace(text.String())
	if msg.Usage == nil {
		msg.Usage = &store.Usage{}
	}
	if msg.Usage.MS == 0 {
		msg.Usage.MS = time.Since(started).Milliseconds()
	}
	if msg.Text == "" && msg.Error == "" {
		msg.Error = "The runtime finished without producing an answer."
	}
	if _, err := s.db.AddMessage(msg); err != nil {
		send(Event{Event: EvError, Code: "store_error", Message: err.Error()})
	}
	_ = userMsg

	send(Event{Event: EvTextDone, Text: msg.Text})
	send(Event{Event: EvUsage, Usage: msg.Usage})
	send(Event{Event: EvTurnEnd})
}

// appendTool merges a tool line with the running one of the same name, so a
// start/finish pair shows as one row that changes state.
func appendTool(list []store.ToolRecord, t store.ToolRecord) []store.ToolRecord {
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].Name == t.Name && list[i].Target == t.Target && list[i].State == "running" {
			list[i].State = t.State
			if t.Detail != "" {
				list[i].Detail = t.Detail
			}
			return list
		}
	}
	return append(list, t)
}

// Cancel stops the turn running on a chat, if any.
func (s *Service) Cancel(chatID string) bool {
	s.mu.Lock()
	cancel, ok := s.active[chatID]
	s.mu.Unlock()
	if !ok {
		return false
	}
	cancel()
	return true
}

// Running reports whether a chat has a turn in flight.
func (s *Service) Running(chatID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.active[chatID]
	return ok
}

// resolveSelection picks the runtime and model for this turn: what the request
// asked for, else what the chat used last, else the stored default.
func (s *Service) resolveSelection(ctx context.Context, chat store.Chat, req SendRequest) runtime.Selection {
	if req.Model != nil && req.Model.Runtime != "" {
		return *req.Model
	}
	if chat.Runtime != "" {
		return runtime.Selection{Runtime: chat.Runtime, Provider: chat.Provider, Model: chat.Model, Effort: chat.Effort}
	}
	return s.runtimes.Default(ctx)
}

// titleFrom turns the first message into a chat title.
func titleFrom(text string) string {
	t := strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if len([]rune(t)) <= 60 {
		return t
	}
	r := []rune(t)
	return strings.TrimSpace(string(r[:57])) + "…"
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Scheme == "file" {
		return "local file"
	}
	return u.Host
}
