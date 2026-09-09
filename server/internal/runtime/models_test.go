package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/provider"
	"github.com/ai-skope/aiss/internal/status"
	"github.com/ai-skope/aiss/internal/store"
)

// The fixtures below are trimmed copies of what the real agents printed; the
// versions they came from are in docs/runtimes/COMPAT.md.

const opencodeListing = `opencode/big-pickle
opencode/ling-3.0-flash-fin-free
zai-coding-plan/glm-4.7
`

const piListing = `provider     model                         context  max-out  thinking  images
ollama       ornith-1.5:35b                262.1K   16.4K    no        yes
ollama       qwen3.8:27b-mlx               128K     16.4K    yes       yes
opencode-go  deepseek-v4-flash             1M       384K     yes       no
`

const ompListing = `{"models":[
{"provider":"ollama","id":"ornith-1.5:35b","contextWindow":262144},
{"provider":"zai","id":"glm-4.5","contextWindow":131072}
]}`

func TestParseAgentModelListings(t *testing.T) {
	t.Run("opencode", func(t *testing.T) {
		got := parseOpencodeModels([]byte(opencodeListing))
		if len(got) != 3 {
			t.Fatalf("want 3 models, got %d: %+v", len(got), got)
		}
		if got[2].Provider != "zai-coding-plan" || got[2].Model.Name != "glm-4.7" {
			t.Errorf("provider/model split: %+v", got[2])
		}
	})

	t.Run("pi", func(t *testing.T) {
		got := parsePiModels([]byte(piListing))
		if len(got) != 3 {
			t.Fatalf("want 3 models, got %d: %+v", len(got), got)
		}
		// The header row must not become a model.
		for _, m := range got {
			if m.Provider == "provider" {
				t.Fatal("header row parsed as a model")
			}
		}
		if got[0].Model.Name != "ornith-1.5:35b" || got[0].Model.Ctx != 262100 {
			t.Errorf("first row: %+v", got[0])
		}
		if got[2].Model.Ctx != 1_000_000 {
			t.Errorf("1M context: %+v", got[2])
		}
	})

	t.Run("omp", func(t *testing.T) {
		got := parseOmpModels([]byte(ompListing))
		if len(got) != 2 {
			t.Fatalf("want 2 models, got %d: %+v", len(got), got)
		}
		if got[1].Provider != "zai" || got[1].Model.Ctx != 131072 {
			t.Errorf("second row: %+v", got[1])
		}
	})

	// A listing whose format moved on should cost rows, never panic.
	t.Run("garbage", func(t *testing.T) {
		for _, p := range []func([]byte) []DiscoveredModel{
			parseOpencodeModels, parsePiModels, parseOmpModels,
		} {
			for _, in := range []string{"", "\n\n", "not a listing at all", "{"} {
				if got := p([]byte(in)); len(got) != 0 {
					t.Errorf("%q parsed into %+v", in, got)
				}
			}
		}
	})
}

func TestParseCtx(t *testing.T) {
	cases := map[string]int64{
		"262.1K": 262100, "1M": 1_000_000, "128K": 128_000,
		"128000": 128000, "": 0, "-": 0, "0": 0, "huge": 0,
	}
	for in, want := range cases {
		if got := parseCtx(in); got != want {
			t.Errorf("parseCtx(%q) = %d, want %d", in, got, want)
		}
	}
}

// An agent that carries its own credentials must appear in the switcher even
// when the user never added a provider — that emptiness was the whole bug.
func TestModelsFallBackToAgentListing(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := config.Default()
	cfg.ProbeTimeout = config.Duration(5 * time.Second)
	cfg.RuntimeCommands = map[string]string{"opencode": fake(t, "opencode-models.sh")}
	reg := NewRegistry(db, cfg, provider.NewRegistry(db, nil), status.NewBus())
	ctx := context.Background()

	var got []ModelOption
	for _, o := range reg.Models(ctx) {
		if o.Runtime == "opencode" {
			got = append(got, o)
		}
	}
	if len(got) != 2 {
		t.Fatalf("want opencode's own 2 models, got %d: %+v", len(got), got)
	}
	if got[0].Provider != "zai-coding-plan" || got[0].Model != "glm-4.7" {
		t.Errorf("first option: %+v", got[0])
	}
	if got[0].Label != "zai-coding-plan / glm-4.7" {
		t.Errorf("label should read provider / model: %q", got[0].Label)
	}

	// A provider scoped to this runtime is a deliberate override: it replaces
	// the agent's own listing rather than joining it.
	if err = db.SaveProvider(store.Provider{
		ID: "p1", Kind: "ollama", Name: "Ollama", BaseURL: "http://localhost:11434",
		AvailableTo: []string{"opencode"},
	}); err != nil {
		t.Fatal(err)
	}
	if err = db.ReplaceProviderModels("p1", []store.Model{{Name: "qwen3", Ctx: 128000}}); err != nil {
		t.Fatal(err)
	}
	got = nil
	for _, o := range reg.Models(ctx) {
		if o.Runtime == "opencode" {
			got = append(got, o)
		}
	}
	if len(got) != 1 || got[0].Model != "qwen3" {
		t.Fatalf("a scoped provider must win outright, got %+v", got)
	}
}

// `aiss runtimes enable …` writes from a separate process, so the running
// server only ever sees the database row change — never a call on its own
// registry. Enabling a runtime it already knows leaves the set of ids
// identical, which is exactly the case the old cache check missed.
func TestListNoticesEnabledChangedByAnotherProcess(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := config.Default()
	cfg.ProbeTimeout = config.Duration(5 * time.Second)
	reg := NewRegistry(db, cfg, provider.NewRegistry(db, nil), status.NewBus())
	ctx := context.Background()

	if err = db.SetRuntimeOverride(store.RuntimeOverride{ID: "codex", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if enabledIn(reg.List(ctx), "codex") {
		t.Fatal("codex should start disabled")
	}

	// The other process writes the row; nothing calls into this registry.
	if err = db.SetRuntimeOverride(store.RuntimeOverride{ID: "codex", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if !enabledIn(reg.List(ctx), "codex") {
		t.Fatal("List kept serving a stale cache after the row changed")
	}
}

func enabledIn(infos []Info, id string) bool {
	for _, i := range infos {
		if i.ID == id {
			return i.Enabled
		}
	}
	return false
}
