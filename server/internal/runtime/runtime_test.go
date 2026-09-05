package runtime

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/provider"
	"github.com/ai-skope/aiss/internal/status"
	"github.com/ai-skope/aiss/internal/store"
)

func fake(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fakes", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// collect drains a turn and returns its text, tools, usage and error.
func collect(t *testing.T, turn Turn) (string, []store.ToolRecord, *store.Usage, string, string) {
	t.Helper()
	var text strings.Builder
	var tools []store.ToolRecord
	var usage *store.Usage
	var errMsg, session string
	timeout := time.After(15 * time.Second)
	for {
		select {
		case ev, ok := <-turn.Events():
			if !ok {
				return text.String(), tools, usage, errMsg, session
			}
			switch ev.Kind {
			// These tests are about parsing, so every flavour of text counts;
			// choosing between them is the chat service's job.
			case EventText, EventTextChunk, EventTextFull:
				text.WriteString(ev.Text)
			case EventTool:
				tools = append(tools, *ev.Tool)
			case EventUsage:
				if ev.Usage != nil && (ev.Usage.InputTokens > 0 || ev.Usage.OutputTokens > 0) {
					usage = ev.Usage
				}
			case EventError:
				errMsg = ev.Err
			case EventSession:
				session = ev.SessionID
			}
		case <-timeout:
			t.Fatal("timed out draining the turn")
		}
	}
}

func runFake(t *testing.T, script string, spec Spec, req TurnRequest) Turn {
	t.Helper()
	if spec.Args == nil {
		spec.Args = func(TurnRequest) []string { return nil }
	}
	spec.PromptViaStdin = true
	if req.Env == nil {
		req.Env = BaseEnv(nil)
	}
	if req.WorkDir == "" {
		req.WorkDir = t.TempDir()
	}
	if req.Timeout == 0 {
		req.Timeout = 15 * time.Second
	}
	turn, err := spawn(context.Background(), spec, req, fake(t, script))
	if err != nil {
		t.Fatal(err)
	}
	return turn
}

func TestClaudeShapeParsed(t *testing.T) {
	turn := runFake(t, "claude-like.sh", Spec{ID: "claude-code"}, TurnRequest{Prompt: "40M events?"})
	text, tools, usage, errMsg, session := collect(t, turn)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !strings.Contains(text, "Growth caps at 25M events") {
		t.Fatalf("assistant text missing: %q", text)
	}
	if !strings.Contains(text, "You asked: 40M events?") {
		t.Fatalf("prompt was not delivered on stdin: %q", text)
	}
	if session != "sess-123" {
		t.Fatalf("session id not captured: %q", session)
	}
	if len(tools) != 1 || tools[0].Name != "Read" || tools[0].Target != "/tmp/x/README.md" {
		t.Fatalf("tool event: %+v", tools)
	}
	if usage == nil || usage.InputTokens != 1200 || usage.OutputTokens != 48 {
		t.Fatalf("usage: %+v", usage)
	}
}

func TestCodexShapeParsed(t *testing.T) {
	turn := runFake(t, "codex-like.sh", Spec{ID: "codex"}, TurnRequest{Prompt: "hi"})
	text, tools, usage, errMsg, session := collect(t, turn)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !strings.Contains(text, "Two conditions apply") {
		t.Fatalf("msg-wrapped text missing: %q", text)
	}
	if session != "th-9" {
		t.Fatalf("thread id should map to the session: %q", session)
	}
	if len(tools) < 1 || tools[0].Name != "command_execution" {
		t.Fatalf("tools: %+v", tools)
	}
	if usage == nil || usage.InputTokens != 10 {
		t.Fatalf("usage: %+v", usage)
	}
}

func TestPlainAndMalformedOutput(t *testing.T) {
	turn := runFake(t, "plain.sh", Spec{ID: "custom:x"}, TurnRequest{Prompt: "x"})
	text, _, _, errMsg, _ := collect(t, turn)
	if errMsg != "" {
		t.Fatalf("plain output must not be an error: %s", errMsg)
	}
	if !strings.Contains(text, "just plain text") || !strings.Contains(text, "{not valid json") {
		t.Fatalf("both lines should survive as text: %q", text)
	}
}

func TestFailingAgentSurfacesStderr(t *testing.T) {
	turn := runFake(t, "failing.sh", Spec{ID: "x"}, TurnRequest{Prompt: "x"})
	_, _, _, errMsg, _ := collect(t, turn)
	if !strings.Contains(errMsg, "connection refused") {
		t.Fatalf("stderr should explain the failure, got %q", errMsg)
	}
}

func TestCancelStopsTheTurn(t *testing.T) {
	turn := runFake(t, "slow.sh", Spec{ID: "x"}, TurnRequest{Prompt: "x", Timeout: time.Minute})
	// Wait for the first token, then cancel.
	select {
	case ev := <-turn.Events():
		if ev.Kind != EventText && ev.Kind != EventTextChunk {
			t.Fatalf("expected text first, got %+v", ev)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no output from the slow agent")
	}
	done := make(chan struct{})
	go func() {
		for range turn.Events() {
		}
		close(done)
	}()
	turn.Cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("cancel did not stop the turn")
	}
}

func TestTurnTimeout(t *testing.T) {
	turn := runFake(t, "slow.sh", Spec{ID: "x"}, TurnRequest{Prompt: "x", Timeout: 700 * time.Millisecond})
	_, _, _, errMsg, _ := collect(t, turn)
	if !strings.Contains(strings.ToLower(errMsg), "too long") {
		t.Fatalf("timeout should be reported plainly, got %q", errMsg)
	}
}

func TestEnvironmentIsScrubbedAndInjected(t *testing.T) {
	t.Setenv("SOME_PRIVATE_TOKEN", "must-not-leak")
	t.Setenv("HOME", t.TempDir())
	env := append(BaseEnv(nil), "ZAI_API_KEY=injected-secret")
	turn := runFake(t, "envdump.sh", Spec{ID: "x"}, TurnRequest{Prompt: "x", Env: env})
	text, _, _, _, _ := collect(t, turn)
	if strings.Contains(text, "must-not-leak") {
		t.Fatal("the server's own environment leaked into the agent")
	}
	if !strings.Contains(text, "ZAI_API_KEY=injected-secret") {
		t.Fatalf("injected provider key missing from the agent environment: %q", text)
	}
	if !strings.Contains(text, "PATH=") {
		t.Fatal("PATH must be passed through or the agent cannot run anything")
	}
}

func TestPassthroughEnv(t *testing.T) {
	t.Setenv("MY_OPT_IN", "yes")
	env := BaseEnv([]string{"MY_OPT_IN"})
	var found bool
	for _, e := range env {
		if e == "MY_OPT_IN=yes" {
			found = true
		}
	}
	if !found {
		t.Fatal("passthroughEnv must be honoured")
	}
}

func TestRegistryDetectAndModels(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := config.Default()
	cfg.ProbeTimeout = config.Duration(2 * time.Second)
	cfg.RuntimeCommands = map[string]string{"custom:fake": fake(t, "versioned.sh")}
	reg := NewRegistry(db, cfg, provider.NewRegistry(db, nil), status.NewBus())

	infos := reg.Detect(context.Background())
	var fakeInfo *Info
	for i := range infos {
		if infos[i].ID == "custom:fake" {
			fakeInfo = &infos[i]
		}
	}
	if fakeInfo == nil {
		t.Fatal("custom runtime was not detected")
	}
	if !fakeInfo.Available || fakeInfo.Status != StatusOK {
		t.Fatalf("fake runtime should be healthy: %+v", fakeInfo)
	}
	if fakeInfo.Version != "1.4.2" {
		t.Fatalf("version parsing: %q", fakeInfo.Version)
	}

	// A runtime that is not installed is reported, not hidden.
	var claude *Info
	for i := range infos {
		if infos[i].ID == "claude-code" {
			claude = &infos[i]
		}
	}
	if claude == nil {
		t.Fatal("built-in runtimes must always be listed")
	}
	if !claude.Available && !strings.Contains(claude.Detail, "PATH") {
		t.Fatalf("a missing runtime should say so: %+v", claude)
	}
}

func TestRegistryStartRefusesDisabled(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := config.Default()
	cfg.ProbeTimeout = config.Duration(2 * time.Second)
	cfg.RuntimeCommands = map[string]string{"custom:fake": fake(t, "versioned.sh")}
	reg := NewRegistry(db, cfg, provider.NewRegistry(db, nil), status.NewBus())
	ctx := context.Background()
	reg.Detect(ctx)

	if err := reg.SetEnabled(ctx, "custom:fake", false, fake(t, "versioned.sh")); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Start(ctx, "custom:fake", TurnRequest{Prompt: "x", WorkDir: t.TempDir()}); err == nil {
		t.Fatal("a disabled runtime must refuse to start")
	}
	if _, err := reg.Start(ctx, "nope", TurnRequest{Prompt: "x"}); err == nil {
		t.Fatal("an unknown runtime must refuse to start")
	}
}

func TestSpecArgs(t *testing.T) {
	cc, _ := SpecByID("claude-code")
	args := cc.Args(TurnRequest{Model: "opus-5", SessionID: "s1", Effort: "High"})
	joined := strings.Join(args, " ")
	for _, want := range []string{"-p", "--output-format stream-json", "--verbose", "--permission-mode plan",
		"--model opus-5", "--resume s1", "--effort high"} {
		if !strings.Contains(joined, want) {
			t.Errorf("claude args missing %q: %v", want, args)
		}
	}
	cx, _ := SpecByID("codex")
	joined = strings.Join(cx.Args(TurnRequest{Model: "gpt-5", Effort: "medium"}), " ")
	for _, want := range []string{"exec", "--json", "--sandbox read-only", "--model gpt-5", "model_reasoning_effort=medium"} {
		if !strings.Contains(joined, want) {
			t.Errorf("codex args missing %q: %s", want, joined)
		}
	}
	joined = strings.Join(cx.Args(TurnRequest{SessionID: "abc"}), " ")
	if !strings.Contains(joined, "exec resume abc") {
		t.Errorf("codex resume: %s", joined)
	}
	oc, _ := SpecByID("opencode")
	joined = strings.Join(oc.Args(TurnRequest{Provider: "z.ai", Model: "GLM 5.3"}), " ")
	if !strings.Contains(joined, "--model z.ai/GLM 5.3") {
		t.Errorf("opencode should address models as provider/model: %s", joined)
	}
}

func TestPromptNeverOnArgv(t *testing.T) {
	// A prompt on the command line would be visible in `ps` to every process
	// on the machine. Every built-in spec must take it on stdin.
	for _, s := range Specs {
		if !s.PromptViaStdin {
			t.Errorf("%s must pass the prompt on stdin", s.ID)
		}
		for _, a := range s.Args(TurnRequest{Prompt: "SECRET-PROMPT", Model: "m"}) {
			if strings.Contains(a, "SECRET-PROMPT") {
				t.Errorf("%s put the prompt in argv: %q", s.ID, a)
			}
		}
	}
}

func TestServerXDGDirsAreNotInheritedByAgents(t *testing.T) {
	// aiss stores its own config and data under XDG_*. Agents keep their
	// credentials under the same paths, so handing ours down makes an
	// authenticated agent look unauthenticated — opencode fails outright.
	t.Setenv("XDG_DATA_HOME", "/tmp/aiss-scratch/data")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/aiss-scratch/config")
	for _, e := range BaseEnv(nil) {
		if strings.HasPrefix(e, "XDG_") {
			t.Fatalf("the server's %s must not reach an agent", strings.SplitN(e, "=", 2)[0])
		}
	}
	// Unless the user asks for it explicitly.
	var found bool
	for _, e := range BaseEnv([]string{"XDG_DATA_HOME"}) {
		if e == "XDG_DATA_HOME=/tmp/aiss-scratch/data" {
			found = true
		}
	}
	if !found {
		t.Fatal("passthroughEnv must still be able to pass XDG_DATA_HOME")
	}
}
