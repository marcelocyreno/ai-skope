package runtime

import (
	"testing"

	"github.com/ai-skope/aiss/internal/store"
)

// A JSON frame that arrives split — a fake that lets the shell expand its
// escapes, or an agent whose writer was interrupted — must not reach the user
// as answer text.
func TestParseLineDropsSplitJSONFrames(t *testing.T) {
	if evs := parseLine([]byte(`{"type":"assistant","message":{"content":`)); len(evs) != 0 {
		t.Fatalf("a truncated frame produced events: %+v", evs)
	}
	// A stray brace from a code block is still prose.
	evs := parseLine([]byte("{"))
	if len(evs) != 1 || evs[0].Kind != EventTextChunk {
		t.Fatalf("a bare brace should stay text, got %+v", evs)
	}
}

func TestParseVersionRejectsWhatIsNotAVersion(t *testing.T) {
	for _, in := range []string{
		`{"type":"system","subtype":"init","session_id":"shot-1"}`,
		"Usage: agent [options] <prompt>",
		"",
	} {
		if got := parseVersion(in); got != "" {
			t.Errorf("parseVersion(%q) = %q, want empty", in, got)
		}
	}
	for in, want := range map[string]string{
		"claude 2.1.261":  "2.1.261",
		"omp/18.1.6":      "omp/18.1.6",
		"v0.152.1":        "0.152.1",
		"codex-cli 1.2.3": "1.2.3",
	} {
		if got := parseVersion(in); got != want {
			t.Errorf("parseVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

// A stray plain-text line must not be read as a streaming delta: the chat
// service treats the first delta as proof the runtime streams, and drops the
// structured text that follows. One log line on stdout would lose the answer.
func TestParseLineTreatsLooseTextAsChunk(t *testing.T) {
	evs := parseLine([]byte("warning: config not found, using defaults"))
	if len(evs) != 1 || evs[0].Kind != EventTextChunk {
		t.Fatalf("loose text should be a chunk, got %+v", evs)
	}
}

// toolRows collects the rows a sequence of agent lines produces, merged by
// the same store.AppendTool the chat service uses. The parser and the merge
// are two halves of one behaviour, and testing either alone proves nothing
// about what a user ends up seeing.
func toolRows(t *testing.T, lines ...string) []store.ToolRecord {
	t.Helper()
	var rows []store.ToolRecord
	for _, line := range lines {
		for _, ev := range parseLine([]byte(line)) {
			if ev.Kind == EventTool && ev.Tool != nil {
				rows = store.AppendTool(rows, *ev.Tool)
			}
		}
	}
	return rows
}

// pi announces a call in three message_update frames and then runs it. All
// four described one tool call, but the frames disagreed about the name — the
// parser had fallen back to the wire event's own type — so they became four
// rows, of which the two that never reported an end spun for ever.
func TestPiToolCallIsOneRow(t *testing.T) {
	rows := toolRows(t,
		`{"type":"message_update","assistantMessageEvent":{"type":"toolcall_start","contentIndex":2,"id":"call_7","toolName":"read"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"toolcall_delta","contentIndex":2,"delta":"{\"pa"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"toolcall_end","contentIndex":2,"toolCall":{"type":"toolCall","id":"call_7","name":"read","arguments":{"path":"README.md"}}}}`,
		`{"type":"tool_execution_start","toolCallId":"call_7","toolName":"read","args":{"path":"README.md"}}`,
		`{"type":"tool_execution_end","toolCallId":"call_7","toolName":"read","result":"# AI Skope","isError":false}`,
	)
	if len(rows) != 1 {
		t.Fatalf("four frames for one call made %d rows: %+v", len(rows), rows)
	}
	if rows[0].Name != "read" || rows[0].Target != "README.md" || rows[0].State != "done" {
		t.Errorf("row reads %+v, want read / README.md / done", rows[0])
	}
}

// The delta is the argument JSON arriving character by character. It names no
// tool and belongs to no row of its own.
func TestPiToolCallDeltaIsNotARow(t *testing.T) {
	if rows := toolRows(t,
		`{"type":"message_update","assistantMessageEvent":{"type":"toolcall_delta","contentIndex":2,"delta":"{\"path\":"}}`,
	); len(rows) != 0 {
		t.Fatalf("a delta produced rows: %+v", rows)
	}
}

// Nothing used to emit "failed", so the transcript showed a check mark and
// "Read" for a tool that had not read anything.
func TestToolExecutionErrorIsFailed(t *testing.T) {
	rows := toolRows(t,
		`{"type":"message_update","assistantMessageEvent":{"type":"toolcall_start","contentIndex":0,"id":"call_9","toolName":"read"}}`,
		`{"type":"tool_execution_end","toolCallId":"call_9","toolName":"read","result":"no such file","isError":true}`,
	)
	if len(rows) != 1 || rows[0].State != "failed" {
		t.Fatalf("a failing tool made %+v, want one failed row", rows)
	}
}

// Claude Code's result block names no tool: it points back at the use block by
// tool_use_id. Without reading that, the pair was two rows, one of them left
// running and the other called "tool_result".
func TestClaudeToolUseAndResultMerge(t *testing.T) {
	rows := toolRows(t,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"/tmp/notes.md"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"notes"}]}}`,
	)
	if len(rows) != 1 {
		t.Fatalf("a use/result pair made %d rows: %+v", len(rows), rows)
	}
	if rows[0].Name != "Read" || rows[0].Target != "/tmp/notes.md" || rows[0].State != "done" {
		t.Errorf("row reads %+v, want Read / /tmp/notes.md / done", rows[0])
	}
}

// Codex repeats the item id across started and completed, so the pair merges
// the same way — and the item's own type still names the tool.
func TestCodexItemPairMerges(t *testing.T) {
	rows := toolRows(t,
		`{"type":"item.started","item":{"id":"item_3","type":"command_execution","command":"ls -la"}}`,
		`{"type":"item.completed","item":{"id":"item_3","type":"command_execution","command":"ls -la"}}`,
	)
	if len(rows) != 1 || rows[0].State != "done" || rows[0].Name != "command_execution" {
		t.Fatalf("a Codex item pair made %+v, want one done command_execution row", rows)
	}
}

// An agent that gives no id keeps the behaviour it always had: the row is
// found by name and target.
func TestToolsWithoutIdsStillMerge(t *testing.T) {
	rows := toolRows(t,
		`{"type":"tool_use","name":"grep","input":{"pattern":"TODO"}}`,
		`{"type":"tool_result","name":"grep","input":{"pattern":"TODO"}}`,
	)
	if len(rows) != 1 || rows[0].State != "done" {
		t.Fatalf("an id-less pair made %+v, want one done row", rows)
	}
}
