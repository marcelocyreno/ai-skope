package runtime

import (
	"encoding/json"
	"strings"

	"github.com/ai-skope/aiss/internal/store"
)

// The agents this server drives all speak JSON lines, but not the same JSON.
// Rather than pin one shape per agent and break on the next release, the
// parser below understands the union of the shapes they are known to emit and
// falls back to treating a line as plain text.
//
// docs/runtimes/COMPAT.md records what was verified against which version.

// parseLine converts one line of agent output into zero or more events.
func parseLine(line []byte) []Event {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		return nil
	}
	if !strings.HasPrefix(trimmed, "{") {
		// Plain-text output (agents that do not implement a JSON mode, or
		// stray log lines) is still useful as answer text.
		return []Event{{Kind: EventText, Text: trimmed + "\n"}}
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return []Event{{Kind: EventText, Text: trimmed + "\n"}}
	}
	return parseObject(raw)
}

func parseObject(raw map[string]any) []Event {
	var out []Event

	// Session identity, under any of the names the agents use.
	for _, k := range []string{"session_id", "sessionId", "thread_id", "threadId", "conversation_id"} {
		if s, ok := str(raw[k]); ok && s != "" {
			out = append(out, Event{Kind: EventSession, SessionID: s})
			break
		}
	}

	typ, _ := str(raw["type"])

	// Codex wraps its payload in "msg"; unwrap and recurse.
	if msg, ok := raw["msg"].(map[string]any); ok {
		return append(out, parseObject(msg)...)
	}

	switch typ {
	case "assistant", "message":
		if m, ok := raw["message"].(map[string]any); ok {
			out = append(out, parseContent(m["content"])...)
		} else {
			out = append(out, parseContent(raw["content"])...)
		}
	case "content_block_delta", "stream_event":
		if d, ok := raw["delta"].(map[string]any); ok {
			if t, ok := str(d["text"]); ok && t != "" {
				out = append(out, Event{Kind: EventText, Text: t})
			}
		} else if ev, ok := raw["event"].(map[string]any); ok {
			out = append(out, parseObject(ev)...)
		}
	case "item.started", "item.completed", "item.updated":
		if item, ok := raw["item"].(map[string]any); ok {
			out = append(out, parseItem(item, typ == "item.completed")...)
		}
	case "agent_message", "agent_message_delta":
		if t, ok := firstString(raw, "message", "text", "delta"); ok {
			out = append(out, Event{Kind: EventText, Text: t})
		}
	case "tool_use", "tool_call", "function_call", "exec_command_begin":
		out = append(out, toolEvent(raw, "running"))
	case "tool_result", "exec_command_end":
		out = append(out, toolEvent(raw, "done"))
	case "error":
		out = append(out, errorEvent(raw))
	case "result", "turn.completed", "response.completed":
		if t, ok := firstString(raw, "result", "text", "message"); ok && t != "" {
			// Some agents only emit the full answer at the end; keep it, the
			// caller de-duplicates against what it already streamed.
			out = append(out, Event{Kind: EventText, Text: t})
		}
		if u := parseUsage(raw["usage"]); u != nil {
			out = append(out, Event{Kind: EventUsage, Usage: u})
		}
		if sub, _ := str(raw["subtype"]); strings.Contains(sub, "error") {
			out = append(out, errorEvent(raw))
		}
	default:
		// Unknown shapes: salvage any obvious text or usage, ignore the rest.
		if t, ok := firstString(raw, "text", "delta", "content"); ok && t != "" {
			out = append(out, Event{Kind: EventText, Text: t})
		}
		if u := parseUsage(raw["usage"]); u != nil {
			out = append(out, Event{Kind: EventUsage, Usage: u})
		}
	}

	if u := parseUsage(raw["usage"]); u != nil && typ != "result" && typ != "turn.completed" {
		out = append(out, Event{Kind: EventUsage, Usage: u})
	}
	return out
}

// parseContent handles Anthropic-style content blocks.
func parseContent(v any) []Event {
	var out []Event
	switch c := v.(type) {
	case string:
		if c != "" {
			out = append(out, Event{Kind: EventText, Text: c})
		}
	case []any:
		for _, item := range c {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			switch t, _ := str(m["type"]); t {
			case "text":
				if s, ok := str(m["text"]); ok && s != "" {
					out = append(out, Event{Kind: EventText, Text: s})
				}
			case "tool_use":
				out = append(out, toolEvent(m, "running"))
			case "tool_result":
				out = append(out, toolEvent(m, "done"))
			}
		}
	}
	return out
}

// parseItem handles Codex's item envelope.
func parseItem(item map[string]any, completed bool) []Event {
	switch t, _ := str(item["type"]); t {
	case "agent_message", "assistant_message":
		if s, ok := firstString(item, "text", "message"); ok && s != "" {
			return []Event{{Kind: EventText, Text: s}}
		}
	case "command_execution", "tool_call", "file_change", "mcp_tool_call":
		state := "running"
		if completed {
			state = "done"
		}
		// For an item envelope the item's own type names the tool; the
		// command or path it acted on is the target.
		ev := toolEvent(item, state)
		ev.Tool.Name = t
		if ev.Tool.Target == "" {
			ev.Tool.Target, _ = firstString(item, "command", "path", "file", "name")
		}
		return []Event{ev}
	case "error":
		return []Event{errorEvent(item)}
	}
	return nil
}

func toolEvent(m map[string]any, state string) Event {
	name, _ := firstString(m, "name", "tool", "tool_name", "command", "type")
	target, _ := firstString(m, "target", "path", "file", "file_path", "selector")
	if target == "" {
		if input, ok := m["input"].(map[string]any); ok {
			target, _ = firstString(input, "file_path", "path", "pattern", "command", "selector")
		}
	}
	detail, _ := firstString(m, "detail", "description", "summary")
	if name == "" {
		name = "tool"
	}
	return Event{Kind: EventTool, Tool: &store.ToolRecord{
		Name: name, Target: target, Detail: detail, State: state,
	}}
}

func errorEvent(m map[string]any) Event {
	msg, _ := firstString(m, "message", "error", "detail", "result")
	if msg == "" {
		msg = "the runtime reported an error"
	}
	if e, ok := m["error"].(map[string]any); ok {
		if s, ok := firstString(e, "message", "detail"); ok && s != "" {
			msg = s
		}
	}
	return Event{Kind: EventError, Err: msg, Retryable: true}
}

func parseUsage(v any) *store.Usage {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	u := &store.Usage{
		InputTokens:  num(m, "input_tokens", "inputTokens", "prompt_tokens"),
		OutputTokens: num(m, "output_tokens", "outputTokens", "completion_tokens"),
	}
	if u.InputTokens == 0 && u.OutputTokens == 0 {
		return nil
	}
	return u
}

func str(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func firstString(m map[string]any, keys ...string) (string, bool) {
	for _, k := range keys {
		if s, ok := str(m[k]); ok && s != "" {
			return s, true
		}
	}
	return "", false
}

func num(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		switch v := m[k].(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		}
	}
	return 0
}
