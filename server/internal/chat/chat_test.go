package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/files"
	"github.com/ai-skope/aiss/internal/provider"
	"github.com/ai-skope/aiss/internal/runtime"
	"github.com/ai-skope/aiss/internal/status"
	"github.com/ai-skope/aiss/internal/store"
)

func fakeAgent(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fakes", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func newService(t *testing.T, agent string) (*Service, *store.DB, string) {
	t.Helper()
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	root := t.TempDir()
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	if _, err := db.AddFolder(root, store.AccessRead); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.RuntimeCommands = map[string]string{"custom:fake": fakeAgent(t, agent)}
	cfg.ProbeTimeout = config.Duration(2 * time.Second)
	guard := files.NewGuard(db, cfg)
	bus := status.NewBus()
	rt := runtime.NewRegistry(db, cfg, provider.NewRegistry(db, nil), bus)
	rt.Detect(context.Background())
	if err := rt.SetDefault(runtime.Selection{Runtime: "custom:fake", Model: "fake-1"}); err != nil {
		t.Fatal(err)
	}
	return NewService(db, cfg, guard, rt, bus), db, root
}

func drain(t *testing.T, ch <-chan Event) []Event {
	t.Helper()
	var out []Event
	timeout := time.After(20 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, ev)
		case <-timeout:
			t.Fatal("timed out waiting for the turn to finish")
		}
	}
}

func kinds(evs []Event) []string {
	var out []string
	for _, e := range evs {
		out = append(out, e.Event)
	}
	return out
}

func joinText(evs []Event) string {
	var sb strings.Builder
	for _, e := range evs {
		if e.Event == EvTextDelta {
			sb.WriteString(e.Text)
		}
	}
	return sb.String()
}

func TestSendStreamsAndPersists(t *testing.T) {
	svc, db, _ := newService(t, "claude-like.sh")
	chat, err := db.CreateChat(store.Chat{URL: "https://n.example/pricing", Host: "n.example"})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := svc.Send(context.Background(), chat.ID, SendRequest{
		Text: "Is Growth enough for 40M events?",
		Page: &PageRef{URL: "https://n.example/pricing", Title: "Pricing"},
		Context: []store.ContextItem{
			{Type: store.ContextElement, Selector: "article.pg-tier.featured", Text: "Growth $149", Rect: []int{320, 412}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	evs := drain(t, ch)

	k := kinds(evs)
	if k[0] != EvTurnStart || k[len(k)-1] != EvTurnEnd {
		t.Fatalf("a turn must start and end: %v", k)
	}
	if !strings.Contains(joinText(evs), "Growth caps at 25M events") {
		t.Fatalf("answer text missing: %q", joinText(evs))
	}
	var sawTool bool
	for _, e := range evs {
		if e.Event == EvTool && e.Tool.Name == "Read" {
			sawTool = true
		}
	}
	if !sawTool {
		t.Fatal("tool line should reach the client")
	}

	// The prompt reaches the agent with the context in it.
	if !strings.Contains(joinText(evs), "article.pg-tier.featured") {
		t.Fatalf("context did not reach the agent: %q", joinText(evs))
	}

	msgs, err := db.Messages(chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("transcript: %+v", msgs)
	}
	if len(msgs[0].Context) != 1 || msgs[0].Context[0].Selector != "article.pg-tier.featured" {
		t.Fatalf("user context not persisted: %+v", msgs[0].Context)
	}
	if msgs[1].Text == "" || msgs[1].Usage == nil || msgs[1].Usage.InputTokens != 1200 {
		t.Fatalf("assistant message not persisted: %+v", msgs[1])
	}
	if len(msgs[1].Tools) == 0 || msgs[1].Tools[0].State != "running" && msgs[1].Tools[0].State != "done" {
		t.Fatalf("tools not persisted: %+v", msgs[1].Tools)
	}

	// The chat is titled from the first question and remembers the agent session.
	got, _ := db.Chat(chat.ID)
	if !strings.HasPrefix(got.Title, "Is Growth enough") {
		t.Fatalf("title: %q", got.Title)
	}
	if got.AgentSession != "sess-123" {
		t.Fatalf("agent session should be remembered for the next turn: %q", got.AgentSession)
	}
	if got.Runtime != "custom:fake" {
		t.Fatalf("runtime not recorded: %+v", got)
	}
}

func TestFileContextIsReadAndInlined(t *testing.T) {
	svc, db, root := newService(t, "claude-like.sh")
	path := filepath.Join(root, "README.md")
	if err := os.WriteFile(path, []byte("The export format writes CSV and JSON per month."), 0o644); err != nil {
		t.Fatal(err)
	}
	chat, _ := db.CreateChat(store.Chat{})
	ch, err := svc.Send(context.Background(), chat.ID, SendRequest{
		Text:    "What does the export section say?",
		Context: []store.ContextItem{{Type: store.ContextFile, Path: path}},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := joinText(drain(t, ch))
	if !strings.Contains(text, "The export format writes CSV") {
		t.Fatalf("file content was not inlined into the prompt: %q", text)
	}
	// Attaching a file makes it recent, so the picker offers it next time.
	recents, _ := db.RecentFiles(10)
	var found bool
	for _, r := range recents {
		if r.Path == path {
			found = true
		}
	}
	if !found {
		t.Fatal("attached file should become a recent file")
	}
}

func TestUnreadableFileIsReportedNotHidden(t *testing.T) {
	svc, db, _ := newService(t, "claude-like.sh")
	chat, _ := db.CreateChat(store.Chat{})
	ch, err := svc.Send(context.Background(), chat.ID, SendRequest{
		Text:    "explain",
		Context: []store.ContextItem{{Type: store.ContextFile, Path: "/etc/passwd"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := joinText(drain(t, ch))
	if !strings.Contains(text, "could not be read") {
		t.Fatalf("a refused file must be stated in the prompt, not skipped: %q", text)
	}
}

func TestFailingRuntimeIsRecorded(t *testing.T) {
	svc, db, _ := newService(t, "failing.sh")
	chat, _ := db.CreateChat(store.Chat{})
	ch, err := svc.Send(context.Background(), chat.ID, SendRequest{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	evs := drain(t, ch)
	var errEv *Event
	for i := range evs {
		if evs[i].Event == EvError {
			errEv = &evs[i]
		}
	}
	if errEv == nil || !strings.Contains(errEv.Message, "connection refused") {
		t.Fatalf("the failure must reach the client: %+v", evs)
	}
	msgs, _ := db.Messages(chat.ID)
	if len(msgs) != 2 || msgs[1].Error == "" {
		t.Fatalf("the failure must be in the transcript: %+v", msgs)
	}
}

func TestCancelEndsTheTurn(t *testing.T) {
	svc, db, _ := newService(t, "slow.sh")
	chat, _ := db.CreateChat(store.Chat{})
	ch, err := svc.Send(context.Background(), chat.ID, SendRequest{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	// Wait for the turn to be under way, then cancel it.
	deadline := time.After(10 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Event == EvTextDelta {
				goto cancel
			}
		case <-deadline:
			t.Fatal("no output before cancelling")
		}
	}
cancel:
	if !svc.Running(chat.ID) {
		t.Fatal("the chat should be running")
	}
	if !svc.Cancel(chat.ID) {
		t.Fatal("cancel should report that it stopped something")
	}
	drain(t, ch)
	if svc.Running(chat.ID) {
		t.Fatal("the chat should no longer be running")
	}
	if svc.Cancel(chat.ID) {
		t.Fatal("cancelling an idle chat should report nothing to stop")
	}
}

func TestSecondTurnWhileRunningIsRefused(t *testing.T) {
	svc, db, _ := newService(t, "slow.sh")
	chat, _ := db.CreateChat(store.Chat{})
	ch, err := svc.Send(context.Background(), chat.ID, SendRequest{Text: "one"})
	if err != nil {
		t.Fatal(err)
	}
	<-ch // the turn has started
	if _, err := svc.Send(context.Background(), chat.ID, SendRequest{Text: "two"}); err != ErrBusy {
		t.Fatalf("a second turn must be refused, got %v", err)
	}
	svc.Cancel(chat.ID)
	drain(t, ch)
}

func TestEmptyMessageRefused(t *testing.T) {
	svc, db, _ := newService(t, "claude-like.sh")
	chat, _ := db.CreateChat(store.Chat{})
	if _, err := svc.Send(context.Background(), chat.ID, SendRequest{Text: "   "}); err == nil {
		t.Fatal("an empty message must be refused")
	}
}

func TestPackBudget(t *testing.T) {
	db, _ := store.OpenMemory()
	defer db.Close()
	root := t.TempDir()
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	db.AddFolder(root, store.AccessRead)
	guard := files.NewGuard(db, config.Default())

	big := strings.Repeat("word ", 20000)
	packed := Pack(guard, "question?", &store.ContextItem{Type: store.ContextPage, Text: big, URL: "u"},
		[]store.ContextItem{{Type: store.ContextElement, Selector: "div", Text: big}}, 4000)
	if packed.Bytes > 6000 {
		t.Fatalf("packed prompt ignored the budget: %d bytes", packed.Bytes)
	}
	if !strings.Contains(packed.Prompt, "question?") {
		t.Fatal("the question must always survive")
	}
	if !strings.Contains(packed.Prompt, "truncated") {
		t.Fatal("truncation must be stated, not silent")
	}
	if !strings.Contains(packed.Prompt, "## Picked element") || !strings.Contains(packed.Prompt, "## Page") {
		t.Fatalf("every context item must appear: %q", packed.Prompt[:200])
	}
	// The most specific context comes before the whole page.
	if strings.Index(packed.Prompt, "## Picked element") > strings.Index(packed.Prompt, "## Page") {
		t.Fatal("picked element should precede the page")
	}
}

func TestAnswerIsNotRepeatedWhenTheAgentSendsItThreeWays(t *testing.T) {
	// Claude Code streams token deltas, then the assembled message, then the
	// whole answer again in its result frame. Naively taking all three prints
	// the answer three times.
	svc, db, _ := newService(t, "claude-partial.sh")
	chat, _ := db.CreateChat(store.Chat{})
	ch, err := svc.Send(context.Background(), chat.ID, SendRequest{Text: "count"})
	if err != nil {
		t.Fatal(err)
	}
	evs := drain(t, ch)
	streamed := joinText(evs)
	if streamed != "one two three" {
		t.Fatalf("the answer should appear exactly once, got %q", streamed)
	}

	msgs, _ := db.Messages(chat.ID)
	if got := msgs[1].Text; got != "one two three" {
		t.Fatalf("stored transcript: %q", got)
	}
	if msgs[1].Usage == nil || msgs[1].Usage.InputTokens != 12 {
		t.Fatalf("usage from the result frame: %+v", msgs[1].Usage)
	}
	got, _ := db.Chat(chat.ID)
	if got.AgentSession != "sess-partial" {
		t.Fatalf("session id: %q", got.AgentSession)
	}
}

// Each agent reports a turn differently. These lock in the shapes that were
// verified against the real binaries, including the parts that must NOT reach
// the transcript: the user's own prompt echoed back, and the model's private
// reasoning.
func TestAgentOutputShapes(t *testing.T) {
	cases := []struct {
		name, fake, want, session string
		inputTokens              int64
	}{
		{"pi and omp", "pi-like.sh", "one two", "01a06f28-pi-session", 2044},
		{"opencode", "opencode-like.sh", "one two", "ses_opencode123", 11},
		{"claude code", "claude-partial.sh", "one two three", "sess-partial", 12},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, db, _ := newService(t, c.fake)
			chat, _ := db.CreateChat(store.Chat{})
			ch, err := svc.Send(context.Background(), chat.ID, SendRequest{Text: "count"})
			if err != nil {
				t.Fatal(err)
			}
			streamed := joinText(drain(t, ch))
			if streamed != c.want {
				t.Fatalf("streamed %q, want %q", streamed, c.want)
			}
			if strings.Contains(streamed, "SECRET REASONING") {
				t.Fatal("the model's private reasoning must not appear in the answer")
			}
			if strings.Contains(streamed, "THE USER PROMPT") {
				t.Fatal("the user's own message must not be echoed into the answer")
			}
			msgs, _ := db.Messages(chat.ID)
			if msgs[1].Text != c.want {
				t.Fatalf("stored %q, want %q", msgs[1].Text, c.want)
			}
			if msgs[1].Usage == nil || msgs[1].Usage.InputTokens != c.inputTokens {
				t.Fatalf("usage: %+v, want %d input tokens", msgs[1].Usage, c.inputTokens)
			}
			got, _ := db.Chat(chat.ID)
			if got.AgentSession != c.session {
				t.Fatalf("session %q, want %q", got.AgentSession, c.session)
			}
		})
	}
}

func TestPageIsNamedEvenWhenItsTextIsWithheld(t *testing.T) {
	// With page access on Ask — the default — a question arrives with the
	// page's URL and title but no text. The model must still be told which
	// page it is being asked about, or it cannot even say what it is missing.
	svc, db, _ := newService(t, "claude-like.sh")
	chat, _ := db.CreateChat(store.Chat{})
	ch, err := svc.Send(context.Background(), chat.ID, SendRequest{
		Text: "what is this about?",
		Page: &PageRef{URL: "https://news.example/article", Title: "A headline"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The fake echoes its prompt, so the prompt itself can be inspected.
	prompt := joinText(drain(t, ch))
	if !strings.Contains(prompt, "https://news.example/article") {
		t.Fatalf("the page's URL must reach the model: %q", prompt)
	}
	if !strings.Contains(prompt, "A headline") {
		t.Fatalf("the page's title must reach the model: %q", prompt)
	}
}

func TestPageTextIsIncludedWhenShared(t *testing.T) {
	svc, db, _ := newService(t, "claude-like.sh")
	chat, _ := db.CreateChat(store.Chat{})
	ch, err := svc.Send(context.Background(), chat.ID, SendRequest{
		Text: "summarize",
		Page: &PageRef{URL: "https://news.example/a", Title: "T", Text: "The report was criticised by allies."},
	})
	if err != nil {
		t.Fatal(err)
	}
	if prompt := joinText(drain(t, ch)); !strings.Contains(prompt, "criticised by allies") {
		t.Fatalf("shared page text must reach the model: %q", prompt)
	}
}
