package runtime

import "testing"

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
