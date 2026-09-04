package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ai-skope/aiss/internal/store"
)

func newRegistry(t *testing.T) (*Registry, *store.DB) {
	t.Helper()
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ks := newFileStore(t.TempDir()) // never touch the developer's real keychain
	return NewRegistry(db, ks), db
}

func TestFileKeystoreRoundTrip(t *testing.T) {
	ks := newFileStore(t.TempDir())
	if err := ks.Set("provider:x", "sk-secret-value"); err != nil {
		t.Fatal(err)
	}
	got, err := ks.Get("provider:x")
	if err != nil || got != "sk-secret-value" {
		t.Fatalf("get: %v %q", err, got)
	}
	if _, err := ks.Get("provider:missing"); err != ErrNoKey {
		t.Fatalf("missing key must report ErrNoKey, got %v", err)
	}
	if err := ks.Delete("provider:x"); err != nil {
		t.Fatal(err)
	}
	if _, err := ks.Get("provider:x"); err != ErrNoKey {
		t.Fatal("deleted key must be gone")
	}
}

func TestKeystoreFileIsEncrypted(t *testing.T) {
	dir := t.TempDir()
	ks := newFileStore(dir)
	if err := ks.Set("provider:x", "sk-plaintext-must-not-appear"); err != nil {
		t.Fatal(err)
	}
	raw, err := readAll(dir + "/keys.enc")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "sk-plaintext-must-not-appear") {
		t.Fatal("secret stored in plaintext on disk")
	}
}

func TestMask(t *testing.T) {
	cases := map[string]string{
		"":                        "",
		"abc":                     "…bc",
		"sk-ant-api03-longkey1234": "sk-ant…1234",
	}
	for in, want := range cases {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCreateTestAndEnvInjection(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/models" {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":"GLM 5.3","context_length":200000},{"id":"GLM 5.3 Flash"}]}`))
	}))
	defer srv.Close()

	reg, db := newRegistry(t)
	p, err := reg.Create(context.Background(), Input{
		Kind: "openai-compatible", Name: "z.ai", BaseURL: srv.URL,
		Key: "zai-secret-key-value", AvailableTo: []string{"pi", "opencode"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.KeyMasked == "zai-secret-key-value" || !strings.Contains(p.KeyMasked, "…") {
		t.Fatalf("key must be masked in the API shape, got %q", p.KeyMasked)
	}
	if len(p.Models) != 2 || p.Models[0].Name != "GLM 5.3" {
		t.Fatalf("models not discovered: %+v", p.Models)
	}
	if gotAuth != "Bearer zai-secret-key-value" {
		t.Fatalf("auth header: %q", gotAuth)
	}
	if !p.LastTestOK {
		t.Fatal("successful test must be recorded")
	}

	// The stored row must never carry the plaintext.
	rows, _ := db.Providers()
	for _, row := range rows {
		if strings.Contains(row.KeyMasked, "secret-key-value") {
			t.Fatal("plaintext key leaked into the database")
		}
	}

	env := reg.Env("pi")
	var found bool
	for _, e := range env {
		if e == "OPENAI_API_KEY=zai-secret-key-value" {
			found = true
		}
		if strings.HasPrefix(e, "OPENAI_BASE_URL=") && !strings.Contains(e, srv.URL) {
			t.Fatalf("base url env wrong: %q", e)
		}
	}
	if !found {
		t.Fatalf("pi must receive the key in its environment: %v", env)
	}
	if env := reg.Env("claude-code"); len(env) != 0 {
		t.Fatalf("a runtime not in availableTo must get nothing: %v", env)
	}
}

func TestTestReportsRejectedKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	reg, _ := newRegistry(t)
	p, err := reg.Create(context.Background(), Input{Kind: "openai-compatible", BaseURL: srv.URL, Key: "bad"})
	if err != nil {
		t.Fatal(err)
	}
	if p.LastTestOK || !strings.Contains(p.LastTestMsg, "rejected") {
		t.Fatalf("a 401 must be reported as a rejected key: %+v", p)
	}
}

func TestOllamaNeedsNoKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("ollama must not receive an Authorization header")
		}
		w.Write([]byte(`{"models":[{"name":"qwen3:8b"}]}`))
	}))
	defer srv.Close()
	reg, _ := newRegistry(t)
	p, err := reg.Create(context.Background(), Input{Kind: "ollama", BaseURL: srv.URL, AvailableTo: []string{"pi"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Models) != 1 || p.Models[0].Name != "qwen3:8b" {
		t.Fatalf("ollama models: %+v", p.Models)
	}
}

func TestUnknownKindRejected(t *testing.T) {
	reg, _ := newRegistry(t)
	if _, err := reg.Create(context.Background(), Input{Kind: "nope", Key: "x"}); err == nil {
		t.Fatal("unknown provider kind must be rejected")
	}
	if _, err := reg.Create(context.Background(), Input{Kind: "zai"}); err == nil {
		t.Fatal("a provider that needs a key must not be created without one")
	}
}

func readAll(path string) (string, error) {
	b, err := osReadFile(path)
	return string(b), err
}

func osReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
