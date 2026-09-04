package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ai-skope/aiss/internal/chat"
	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/files"
	"github.com/ai-skope/aiss/internal/provider"
	"github.com/ai-skope/aiss/internal/runtime"
	"github.com/ai-skope/aiss/internal/status"
	"github.com/ai-skope/aiss/internal/store"
)

const testOrigin = "chrome-extension://abcdefghijklmnop"

type harness struct {
	t      *testing.T
	srv    *httptest.Server
	db     *store.DB
	token  string
	root   string
	client *http.Client
}

func newHarness(t *testing.T, agent string) *harness {
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
	cfg.ProbeTimeout = config.Duration(2 * time.Second)
	fakePath, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "fakes", agent))
	cfg.RuntimeCommands = map[string]string{"custom:fake": fakePath}

	bus := status.NewBus()
	guard := files.NewGuard(db, cfg)
	ix := files.NewIndexer(db, cfg, guard, bus)
	providers := provider.NewRegistry(db, nil)
	rts := runtime.NewRegistry(db, cfg, providers, bus)
	rts.Detect(context.Background())
	_ = rts.SetDefault(runtime.Selection{Runtime: "custom:fake", Model: "fake-1"})
	chats := chat.NewService(db, cfg, guard, rts, bus)

	s := New(Deps{DB: db, Cfg: cfg, Guard: guard, Indexer: ix, Providers: providers,
		Runtimes: rts, Chat: chats, Bus: bus, Started: time.Now()})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	h := &harness{t: t, srv: srv, db: db, root: root, client: srv.Client()}
	h.pair()
	return h
}

func (h *harness) pair() {
	h.t.Helper()
	code, err := h.db.NewPairCode(time.Minute)
	if err != nil {
		h.t.Fatal(err)
	}
	var out struct {
		Token string `json:"token"`
	}
	h.do("POST", "/v1/pair", map[string]string{"code": code, "origin": testOrigin, "label": "test"}, &out, http.StatusOK)
	if out.Token == "" {
		h.t.Fatal("pairing returned no token")
	}
	h.token = out.Token
}

// do performs a request with the harness token and decodes the reply.
func (h *harness) do(method, path string, body any, out any, wantStatus int) *http.Response {
	h.t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			h.t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, h.srv.URL+path, rdr)
	if err != nil {
		h.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", testOrigin)
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	if wantStatus != 0 && resp.StatusCode != wantStatus {
		b, _ := readAllString(resp)
		h.t.Fatalf("%s %s: got %d want %d: %s", method, path, resp.StatusCode, wantStatus, b)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			h.t.Fatalf("%s %s: decode: %v", method, path, err)
		}
	}
	resp.Body.Close()
	return resp
}

func readAllString(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return string(b), err
}

func TestHealthIsOpenAndEverythingElseIsNot(t *testing.T) {
	h := newHarness(t, "claude-like.sh")

	resp, err := http.Get(h.srv.URL + "/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health must be open: %d", resp.StatusCode)
	}

	for _, path := range []string{"/v1/models", "/v1/folders", "/v1/chats", "/v1/settings", "/v1/files/recent"} {
		resp, err := http.Get(h.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s without a token: got %d, want 401", path, resp.StatusCode)
		}
	}
}

func TestBadTokenAndBadOriginRefused(t *testing.T) {
	h := newHarness(t, "claude-like.sh")

	req, _ := http.NewRequest("GET", h.srv.URL+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	resp, _ := h.client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a forged token must be refused, got %d", resp.StatusCode)
	}

	// A real token presented from an origin that was never paired: this is the
	// case that matters, a page on the open web trying to use the server.
	req, _ = http.NewRequest("GET", h.srv.URL+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Origin", "https://evil.example")
	resp, _ = h.client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("an unpaired origin must be refused, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("CORS must not be granted to an unpaired origin")
	}
}

func TestPairCodeIsSingleUse(t *testing.T) {
	h := newHarness(t, "claude-like.sh")
	code, _ := h.db.NewPairCode(time.Minute)
	saved := h.token
	h.token = ""
	h.do("POST", "/v1/pair", map[string]string{"code": code, "origin": testOrigin}, nil, http.StatusOK)
	h.do("POST", "/v1/pair", map[string]string{"code": code, "origin": testOrigin}, nil, http.StatusForbidden)
	h.token = saved
}

func TestFoldersAndFileEndpoints(t *testing.T) {
	h := newHarness(t, "claude-like.sh")
	if err := os.WriteFile(filepath.Join(h.root, "README.md"),
		[]byte("The export format writes CSV and JSON per statement month."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.root, ".env"), []byte("SECRET=1"), 0o644); err != nil {
		t.Fatal(err)
	}

	var folders struct {
		Folders []struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		} `json:"folders"`
	}
	h.do("GET", "/v1/folders", nil, &folders, http.StatusOK)
	if len(folders.Folders) != 1 {
		t.Fatalf("folders: %+v", folders)
	}

	h.do("POST", "/v1/folders/"+folders.Folders[0].ID+"/reindex", nil, nil, http.StatusAccepted)
	deadline := time.Now().Add(10 * time.Second)
	var search struct {
		Files []fileDTO `json:"files"`
	}
	for time.Now().Before(deadline) {
		search.Files = nil
		h.do("GET", "/v1/files/search?q=export", nil, &search, http.StatusOK)
		if len(search.Files) > 0 {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	if len(search.Files) == 0 || search.Files[0].Name != "README.md" {
		t.Fatalf("search after reindex: %+v", search.Files)
	}
	if !strings.HasPrefix(search.Files[0].Display, "~") && !strings.HasPrefix(search.Files[0].Display, "/") {
		t.Fatalf("display path looks wrong: %q", search.Files[0].Display)
	}

	// Reading an allowed file works; a denied or outside file does not.
	var content files.Content
	h.do("GET", "/v1/files/read?path="+filepath.Join(h.root, "README.md"), nil, &content, http.StatusOK)
	if !strings.Contains(content.Text, "export format") {
		t.Fatalf("read: %+v", content)
	}
	h.do("GET", "/v1/files/read?path="+filepath.Join(h.root, ".env"), nil, nil, http.StatusForbidden)
	h.do("GET", "/v1/files/read?path=/etc/passwd", nil, nil, http.StatusForbidden)

	// The file:// resolver is what makes a local page askable.
	var resolved map[string]any
	h.do("POST", "/v1/files/resolve", map[string]string{"url": "file://" + filepath.Join(h.root, "README.md")},
		&resolved, http.StatusOK)
	if resolved["path"] != filepath.Join(h.root, "README.md") {
		t.Fatalf("resolve: %+v", resolved)
	}
	h.do("POST", "/v1/files/resolve", map[string]string{"url": "file:///etc/passwd"}, nil, http.StatusForbidden)

	// Adding a folder that is not a folder fails clearly.
	h.do("POST", "/v1/folders", map[string]string{"path": "/definitely/not/here"}, nil, http.StatusBadRequest)
}

func TestChatTurnStreamsOverSSE(t *testing.T) {
	h := newHarness(t, "claude-like.sh")

	var created struct {
		ID string `json:"id"`
	}
	h.do("POST", "/v1/chats", map[string]string{"url": "https://n.example/pricing", "pageTitle": "Pricing"},
		&created, http.StatusCreated)

	body, _ := json.Marshal(chat.SendRequest{
		Text: "Is Growth enough for 40M events?",
		Page: &chat.PageRef{URL: "https://n.example/pricing", Title: "Pricing"},
		Context: []store.ContextItem{
			{Type: store.ContextElement, Selector: "article.pg-tier.featured", Text: "Growth $149"},
		},
	})
	req, _ := http.NewRequest("POST", h.srv.URL+"/v1/chats/"+created.ID+"/messages", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("a turn must stream: %q", ct)
	}

	events, text := readSSE(t, resp)
	if events[0] != chat.EvTurnStart || events[len(events)-1] != chat.EvTurnEnd {
		t.Fatalf("event order: %v", events)
	}
	if !contains(events, chat.EvTextDelta) || !contains(events, chat.EvTool) || !contains(events, chat.EvUsage) {
		t.Fatalf("missing events: %v", events)
	}
	if !strings.Contains(text, "Growth caps at 25M events") {
		t.Fatalf("answer: %q", text)
	}

	// The transcript is readable afterwards, with the context attached.
	var got struct {
		Chat     store.Chat      `json:"chat"`
		Messages []store.Message `json:"messages"`
		Running  bool            `json:"running"`
	}
	h.do("GET", "/v1/chats/"+created.ID, nil, &got, http.StatusOK)
	if len(got.Messages) != 2 || got.Messages[1].Text == "" {
		t.Fatalf("transcript: %+v", got.Messages)
	}
	if len(got.Messages[0].Context) != 1 {
		t.Fatalf("context lost: %+v", got.Messages[0])
	}
	if got.Running {
		t.Fatal("the turn has ended")
	}
	if got.Chat.Title == "" {
		t.Fatal("the chat should be titled from the first message")
	}

	// History lists it under the page and the site.
	var list struct {
		Chats []store.Chat `json:"chats"`
	}
	h.do("GET", "/v1/chats?url=https://n.example/pricing", nil, &list, http.StatusOK)
	if len(list.Chats) != 1 || list.Chats[0].MessageCount != 2 {
		t.Fatalf("history by page: %+v", list.Chats)
	}
	h.do("GET", "/v1/chats?host=n.example", nil, &list, http.StatusOK)
	if len(list.Chats) != 1 {
		t.Fatalf("history by site: %+v", list.Chats)
	}

	// Delete is soft, so Undo can restore it.
	h.do("DELETE", "/v1/chats/"+created.ID, nil, nil, http.StatusNoContent)
	h.do("GET", "/v1/chats", nil, &list, http.StatusOK)
	if len(list.Chats) != 0 {
		t.Fatal("a deleted chat must not be listed")
	}
	h.do("POST", "/v1/chats/"+created.ID+"/restore", nil, nil, http.StatusOK)
	h.do("GET", "/v1/chats", nil, &list, http.StatusOK)
	if len(list.Chats) != 1 {
		t.Fatal("restore must bring it back")
	}
}

func TestChatWithLocalFileContext(t *testing.T) {
	h := newHarness(t, "claude-like.sh")
	path := filepath.Join(h.root, "notes.md")
	os.WriteFile(path, []byte("Overage is never billed automatically."), 0o644)

	var created struct {
		ID string `json:"id"`
	}
	h.do("POST", "/v1/chats", map[string]string{"url": "file://" + path}, &created, http.StatusCreated)

	body, _ := json.Marshal(chat.SendRequest{
		Text:    "What does this say about overage?",
		Context: []store.ContextItem{{Type: store.ContextFile, Path: path}},
	})
	req, _ := http.NewRequest("POST", h.srv.URL+"/v1/chats/"+created.ID+"/messages", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Origin", testOrigin)
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, text := readSSE(t, resp)
	if !strings.Contains(text, "Overage is never billed automatically") {
		t.Fatalf("the file content should reach the agent: %q", text)
	}

	// It becomes a recent file for the picker.
	var recent struct {
		Files []fileDTO `json:"files"`
	}
	h.do("GET", "/v1/files/recent", nil, &recent, http.StatusOK)
	if len(recent.Files) == 0 || recent.Files[0].Path != path {
		t.Fatalf("recent files: %+v", recent.Files)
	}
}

func TestNotesCRUD(t *testing.T) {
	h := newHarness(t, "claude-like.sh")
	var n store.Note
	h.do("POST", "/v1/notes", map[string]string{
		"url": "https://n.example/pricing", "title": "Pricing", "quote": "Growth is free for 12 months", "body": "Ask finance",
	}, &n, http.StatusCreated)
	if n.ID == "" || n.Host != "n.example" {
		t.Fatalf("note: %+v", n)
	}
	h.do("POST", "/v1/notes", map[string]string{"url": "x"}, nil, http.StatusBadRequest)

	var list struct {
		Notes []store.Note `json:"notes"`
	}
	h.do("GET", "/v1/notes?host=n.example", nil, &list, http.StatusOK)
	if len(list.Notes) != 1 {
		t.Fatalf("notes: %+v", list.Notes)
	}
	h.do("PATCH", "/v1/notes/"+n.ID, map[string]string{"body": "Ask finance about SAFEs"}, nil, http.StatusOK)
	h.do("GET", "/v1/notes?q=SAFEs", nil, &list, http.StatusOK)
	if len(list.Notes) != 1 {
		t.Fatal("search should find the edited note")
	}
	h.do("DELETE", "/v1/notes/"+n.ID, nil, nil, http.StatusNoContent)
	h.do("GET", "/v1/notes", nil, &list, http.StatusOK)
	if len(list.Notes) != 0 {
		t.Fatal("deleted note must be hidden")
	}
}

func TestRuntimesAndModels(t *testing.T) {
	h := newHarness(t, "versioned.sh")
	var rts struct {
		Runtimes []runtime.Info `json:"runtimes"`
	}
	h.do("GET", "/v1/runtimes", nil, &rts, http.StatusOK)
	var fake *runtime.Info
	for i := range rts.Runtimes {
		if rts.Runtimes[i].ID == "custom:fake" {
			fake = &rts.Runtimes[i]
		}
	}
	if fake == nil || !fake.Available {
		t.Fatalf("the fake runtime should be detected: %+v", rts.Runtimes)
	}

	h.do("PATCH", "/v1/runtimes/custom:fake", map[string]any{"enabled": false}, nil, http.StatusOK)
	var models struct {
		Models []runtime.ModelOption `json:"models"`
	}
	h.do("GET", "/v1/models", nil, &models, http.StatusOK)
	for _, m := range models.Models {
		if m.Runtime == "custom:fake" {
			t.Fatal("a disabled runtime must not offer models")
		}
	}

	h.do("PUT", "/v1/models/default", map[string]string{"runtime": "claude-code", "model": "opus-5"}, nil, http.StatusOK)
	h.do("PUT", "/v1/models/default", map[string]string{"runtime": "claude-code"}, nil, http.StatusBadRequest)
}

func TestSettingsAndCapabilities(t *testing.T) {
	h := newHarness(t, "claude-like.sh")
	var caps map[string]any
	h.do("GET", "/v1/capabilities", nil, &caps, http.StatusOK)
	if caps["apiVersion"] == nil || caps["providerKinds"] == nil {
		t.Fatalf("capabilities: %+v", caps)
	}

	h.do("PATCH", "/v1/settings", map[string]string{"privacy.pageContent": "never"}, nil, http.StatusOK)
	var got struct {
		Settings map[string]string `json:"settings"`
	}
	h.do("GET", "/v1/settings", nil, &got, http.StatusOK)
	if got.Settings["privacy.pageContent"] != "never" {
		t.Fatalf("settings: %+v", got.Settings)
	}
}

func TestEventStream(t *testing.T) {
	h := newHarness(t, "claude-like.sh")
	req, _ := http.NewRequest("GET", h.srv.URL+"/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Origin", testOrigin)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := h.client.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	sc := bufio.NewScanner(resp.Body)
	var sawSnapshot bool
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "event: runtime.status") {
			sawSnapshot = true
			break
		}
	}
	if !sawSnapshot {
		t.Fatal("a new event stream should open with a runtime snapshot")
	}
}

func TestUnknownFieldsRejected(t *testing.T) {
	h := newHarness(t, "claude-like.sh")
	h.do("POST", "/v1/chats", map[string]string{"nope": "x"}, nil, http.StatusBadRequest)
}

// readSSE drains an event stream, returning the event names and the joined
// text deltas.
func readSSE(t *testing.T, resp *http.Response) ([]string, string) {
	t.Helper()
	var events []string
	var text strings.Builder
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	var event string
	deadline := time.Now().Add(20 * time.Second)
	for sc.Scan() {
		if time.Now().After(deadline) {
			t.Fatal("timed out reading the stream")
		}
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
			events = append(events, event)
		case strings.HasPrefix(line, "data: "):
			var ev chat.Event
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err == nil {
				if ev.Event == chat.EvTextDelta {
					text.WriteString(ev.Text)
				}
			}
		}
	}
	return events, text.String()
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

var _ = fmt.Sprintf
