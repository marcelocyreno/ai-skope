package runtime

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/ai-skope/aiss/internal/store"
)

// The three provider-backed agents each answer "which models can you reach?"
// in their own shape. None of it is a stable contract, so every parser here
// skips what it does not recognise rather than failing: a listing that changes
// format should cost the user some rows, never the whole switcher.
//
// The exact commands and the versions they were read from live in
// docs/runtimes/COMPAT.md.

// parseOpencodeModels reads `opencode models`: one "provider/model" per line.
func parseOpencodeModels(out []byte) []DiscoveredModel {
	var models []DiscoveredModel
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// --verbose interleaves a JSON blob per model; ignore anything that is
		// not a bare selector.
		if line == "" || strings.ContainsAny(line, "{}\"") || strings.Contains(line, " ") {
			continue
		}
		provider, name, ok := strings.Cut(line, "/")
		if !ok || provider == "" || name == "" {
			continue
		}
		models = append(models, DiscoveredModel{Provider: provider, Model: store.Model{Name: name}})
	}
	return models
}

// parsePiModels reads `pi --list-models`: a whitespace-aligned table whose
// first three columns are provider, model and context ("262.1K", "1M").
func parsePiModels(out []byte) []DiscoveredModel {
	var models []DiscoveredModel
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] == "provider" {
			continue
		}
		provider, name := fields[0], fields[1]
		// Every real row carries a context size, and prose does not. Requiring
		// it is what keeps a help line or an error message off the list — the
		// provider column alone is just a word.
		ctx := parseCtx(fields[2])
		if ctx == 0 || strings.Contains(provider, "/") {
			continue
		}
		models = append(models, DiscoveredModel{
			Provider: provider,
			Model:    store.Model{Name: name, Ctx: ctx},
		})
	}
	return models
}

// parseOmpModels reads `omp models --json`.
func parseOmpModels(out []byte) []DiscoveredModel {
	var payload struct {
		Models []struct {
			Provider      string `json:"provider"`
			ID            string `json:"id"`
			Name          string `json:"name"`
			ContextWindow int64  `json:"contextWindow"`
		} `json:"models"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil
	}
	var models []DiscoveredModel
	for _, m := range payload.Models {
		name := m.ID
		if name == "" {
			name = m.Name
		}
		if m.Provider == "" || name == "" {
			continue
		}
		models = append(models, DiscoveredModel{
			Provider: m.Provider,
			Model:    store.Model{Name: name, Ctx: m.ContextWindow},
		})
	}
	return models
}

// parseCtx reads a context size written for people — "262.1K", "1M", "128000".
// It is a display hint, so an unreadable value costs the column, not the row.
func parseCtx(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	mult := int64(1)
	switch s[len(s)-1] {
	case 'K', 'k':
		mult, s = 1_000, s[:len(s)-1]
	case 'M', 'm':
		mult, s = 1_000_000, s[:len(s)-1]
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 {
		return 0
	}
	return int64(f * float64(mult))
}
