// Package runtime drives the coding agents installed on this machine —
// Claude Code, Codex, pi, omp, opencode — as the things that actually answer
// a chat. The server never re-implements a model API: an agent brings its own
// provider access, and the server supplies the working directory, the
// environment, the prompt, and a normalised event stream.
package runtime

import (
	"time"

	"github.com/ai-skope/aiss/internal/store"
)

// Info is what the settings page shows for one runtime.
type Info struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Version      string        `json:"version,omitempty"`
	Path         string        `json:"path,omitempty"`
	Available    bool          `json:"available"`
	Enabled      bool          `json:"enabled"`
	Variants     []string      `json:"variants,omitempty"`
	EffortLevels []string      `json:"effortLevels,omitempty"`
	UsesProvider bool          `json:"usesProviders"`
	Status       string        `json:"status"` // ok | degraded | offline
	LatencyMS    int64         `json:"latencyMs,omitempty"`
	Detail       string        `json:"detail,omitempty"`
	Models       []store.Model `json:"-"`
}

// Status values reported for a runtime and its models.
const (
	StatusOK       = "ok"
	StatusDegraded = "degraded"
	StatusOffline  = "offline"
)

// TurnRequest is one message to answer.
type TurnRequest struct {
	Prompt    string
	Model     string
	Provider  string
	Effort    string
	Variant   string
	SessionID string // the agent's own session, to continue a conversation
	WorkDir   string
	Env       []string
	Timeout   time.Duration
}

// Event kinds emitted by a turn. They map one-to-one onto the SSE events the
// extension renders: a tool line, streaming text, usage, an error.
const (
	EventSession = "session"
	EventTool    = "tool"
	// EventText is an incremental delta: append it.
	EventText = "text.delta"
	// EventTextChunk is a whole message block. Agents emit these *as well as*
	// deltas when partial streaming is on, so it is used only when no deltas
	// have arrived.
	EventTextChunk = "text.chunk"
	// EventTextFull is the complete answer, repeated at the end of a turn. It
	// is used only when nothing else produced any text.
	EventTextFull = "text.full"
	EventUsage    = "usage"
	EventError    = "error"
	EventDone     = "done"
)

// Event is one normalised step of a turn.
type Event struct {
	Kind      string            `json:"kind"`
	Text      string            `json:"text,omitempty"`
	Tool      *store.ToolRecord `json:"tool,omitempty"`
	Usage     *store.Usage      `json:"usage,omitempty"`
	SessionID string            `json:"sessionId,omitempty"`
	Err       string            `json:"error,omitempty"`
	Retryable bool              `json:"retryable,omitempty"`
}

// Turn is a running agent invocation.
type Turn interface {
	// Events yields the turn's events and is closed when the turn ends.
	Events() <-chan Event
	// Cancel stops the agent; the event channel is closed shortly after.
	Cancel()
}
