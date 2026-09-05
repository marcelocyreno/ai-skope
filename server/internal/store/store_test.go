package store

import (
	"testing"
	"time"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrationsAndFTS(t *testing.T) {
	db := newTestDB(t)
	if !db.HasFTS() {
		t.Log("FTS5 unavailable; search falls back to LIKE")
	}
	if id := db.ServerID(); id == "" || db.ServerID() != id {
		t.Fatal("server id must be stable and non-empty")
	}
}

func TestPairingRoundTrip(t *testing.T) {
	db := newTestDB(t)
	code, err := db.NewPairCode(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	tok, p, err := db.RedeemPairCode(code, "chrome-extension://abc", "Chrome")
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" || p.Origin != "chrome-extension://abc" {
		t.Fatalf("bad pairing %q %+v", tok, p)
	}
	if _, _, err := db.RedeemPairCode(code, "chrome-extension://abc", ""); err != ErrBadCode {
		t.Fatal("a pairing code must be single-use")
	}
	got, err := db.PairingByToken(tok)
	if err != nil || got.ID != p.ID {
		t.Fatalf("lookup: %v %+v", err, got)
	}
	if _, err := db.PairingByToken("nope"); err != ErrNotFound {
		t.Fatal("unknown token must not resolve")
	}
	if _, err := db.RevokePairing(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.PairingByToken(tok); err != ErrNotFound {
		t.Fatal("revoked token must not resolve")
	}
}

func TestExpiredPairCode(t *testing.T) {
	db := newTestDB(t)
	code, err := db.NewPairCode(-time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.RedeemPairCode(code, "o", ""); err != ErrBadCode {
		t.Fatalf("expired code must be rejected, got %v", err)
	}
}

func TestChatsAndMessages(t *testing.T) {
	db := newTestDB(t)
	c, err := db.CreateChat(Chat{Title: "Growth plan", URL: "https://n.example/pricing", Host: "n.example", Model: "Opus 5"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := db.AddMessage(Message{ChatID: c.ID, Role: "user", Text: "40M events?", Context: []ContextItem{
		{Type: ContextElement, Selector: "article.tier", Text: "Growth $149"},
		{Type: ContextFile, Path: "/tmp/readme.md"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddMessage(Message{ChatID: c.ID, Role: "assistant", Text: "No.",
		Tools: []ToolRecord{{Name: "read", Target: "table", State: "done"}}, Usage: &Usage{InputTokens: 10, MS: 5}}); err != nil {
		t.Fatal(err)
	}
	msgs, err := db.Messages(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("want 2 messages, got %d", len(msgs))
	}
	if len(msgs[0].Context) != 2 || msgs[0].Context[0].Selector != "article.tier" {
		t.Fatalf("context not round-tripped: %+v", msgs[0].Context)
	}
	if msgs[1].Usage == nil || msgs[1].Usage.InputTokens != 10 {
		t.Fatalf("usage not round-tripped: %+v", msgs[1].Usage)
	}
	if m.ID == "" {
		t.Fatal("message id must be assigned")
	}

	list, err := db.Chats(ChatFilter{URL: "https://n.example/pricing"})
	if err != nil || len(list) != 1 || list[0].MessageCount != 2 {
		t.Fatalf("history: %v %+v", err, list)
	}
	if err := db.SoftDeleteChat(c.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := db.Chats(ChatFilter{}); len(list) != 0 {
		t.Fatal("soft-deleted chat must be hidden")
	}
	if err := db.RestoreChat(c.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := db.Chats(ChatFilter{}); len(list) != 1 {
		t.Fatal("restore must bring the chat back")
	}
}

func TestFileSearch(t *testing.T) {
	db := newTestDB(t)
	f, err := db.AddFolder("/tmp/dev", AccessRead)
	if err != nil {
		t.Fatal(err)
	}
	files := []struct {
		path, name, body string
	}{
		{"/tmp/dev/README.md", "README.md", "The export format writes CSV and JSON per statement month."},
		{"/tmp/dev/notes.md", "notes.md", "Northwind pricing evaluation, overage policy."},
		{"/tmp/dev/main.go", "main.go", "package main"},
	}
	for _, x := range files {
		if err := db.UpsertFile(File{Path: x.path, FolderID: f.ID, Name: x.name, Ext: ".md"}, x.body); err != nil {
			t.Fatal(err)
		}
	}
	got, err := db.SearchFiles("export", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || got[0].Name != "README.md" {
		t.Fatalf("search by body failed: %+v", got)
	}
	got, err = db.SearchFiles("notes", 10)
	if err != nil || len(got) == 0 {
		t.Fatalf("search by name failed: %v %+v", err, got)
	}
	// A query with FTS operators must be treated as literal text, not syntax.
	if _, err := db.SearchFiles(`"unbalanced (OR* NEAR`, 10); err != nil {
		t.Fatalf("hostile query must not error: %v", err)
	}
	if err := db.DeleteFile("/tmp/dev/README.md"); err != nil {
		t.Fatal(err)
	}
	got, _ = db.SearchFiles("export", 10)
	for _, g := range got {
		if g.Path == "/tmp/dev/README.md" {
			t.Fatal("deleted file still indexed")
		}
	}
}

func TestProvidersAndFolders(t *testing.T) {
	db := newTestDB(t)
	p := Provider{ID: NewID(), Kind: "zai", Name: "z.ai", KeyMasked: "zai-…0000", AvailableTo: []string{"pi", "opencode"}}
	if err := db.SaveProvider(p); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceProviderModels(p.ID, []Model{{Name: "GLM 5.3", Ctx: 200000}, {Name: "GLM 5.3 Flash"}}); err != nil {
		t.Fatal(err)
	}
	got, err := db.Provider(p.ID)
	if err != nil || len(got.Models) != 2 || got.AvailableTo[0] != "pi" {
		t.Fatalf("provider round-trip: %v %+v", err, got)
	}
	if err := db.DeleteProvider(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Provider(p.ID); err != ErrNotFound {
		t.Fatal("provider must be gone")
	}

	f1, _ := db.AddFolder("/tmp/a", AccessReadWatch)
	f2, _ := db.AddFolder("/tmp/a", AccessRead) // duplicate is idempotent
	if f1.ID != f2.ID {
		t.Fatal("adding the same folder twice must return the same row")
	}
	if err := db.UpdateFolderAccess(f1.ID, AccessRead); err != nil {
		t.Fatal(err)
	}
	list, _ := db.Folders()
	if len(list) != 1 || list[0].Access != AccessRead {
		t.Fatalf("folders: %+v", list)
	}
}

func TestNotes(t *testing.T) {
	db := newTestDB(t)
	n, err := db.CreateNote(Note{URL: "https://x.example/a", Host: "x.example", Title: "A", Body: "hello", Quote: "q"})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := db.Note(n.ID); err != nil || got.Body != "hello" {
		t.Fatalf("note: %v %+v", err, got)
	}
	if err := db.SoftDeleteNote(n.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Note(n.ID); err != ErrNotFound {
		t.Fatal("deleted note must be hidden")
	}
	if list, _ := db.Notes(NoteFilter{}); len(list) != 0 {
		t.Fatal("deleted note must not be listed")
	}
}
