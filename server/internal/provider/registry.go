package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ai-skope/aiss/internal/store"
)

// Registry is the server's view of configured providers: it owns the rows in
// the database, the secrets in the keystore, and the model discovery calls.
type Registry struct {
	db   *store.DB
	keys Keystore
	http *http.Client
}

// NewRegistry builds a registry.
func NewRegistry(db *store.DB, keys Keystore) *Registry {
	return &Registry{db: db, keys: keys, http: &http.Client{Timeout: 20 * time.Second}}
}

// Keystore exposes the backing secret store (used by `aiss doctor`).
func (r *Registry) Keystore() Keystore { return r.keys }

// KeystoreBackend names where secrets are kept, or "none" when the registry
// was built without a keystore (embedded uses and tests that never store a
// key). Callers should not have to nil-check the store to report status.
func (r *Registry) KeystoreBackend() string {
	if r.keys == nil {
		return "none"
	}
	return r.keys.Backend()
}

// List returns every provider, with masked keys only.
func (r *Registry) List() ([]store.Provider, error) { return r.db.Providers() }

// Get returns one provider.
func (r *Registry) Get(id string) (store.Provider, error) { return r.db.Provider(id) }

// Input is the payload accepted when creating or updating a provider.
type Input struct {
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	BaseURL     string   `json:"baseUrl"`
	Key         string   `json:"key"`
	AvailableTo []string `json:"availableTo"`
}

// Create stores a provider and its secret, then tries to discover its models.
func (r *Registry) Create(ctx context.Context, in Input) (store.Provider, error) {
	kind, ok := KindByID(in.Kind)
	if !ok {
		return store.Provider{}, fmt.Errorf("unknown provider kind %q", in.Kind)
	}
	if kind.NeedsKey && strings.TrimSpace(in.Key) == "" {
		return store.Provider{}, fmt.Errorf("%s needs an API key", kind.Name)
	}
	p := store.Provider{
		ID:          store.NewID(),
		Kind:        kind.ID,
		Name:        firstNonEmpty(in.Name, kind.Name),
		BaseURL:     firstNonEmpty(in.BaseURL, kind.BaseURL),
		AvailableTo: in.AvailableTo,
		KeyMasked:   Mask(in.Key),
	}
	if p.BaseURL == "" {
		return store.Provider{}, fmt.Errorf("%s needs a base URL", p.Name)
	}
	if in.Key != "" {
		p.KeyRef = "provider:" + p.ID
		if err := r.keys.Set(p.KeyRef, in.Key); err != nil {
			return store.Provider{}, fmt.Errorf("store key: %w", err)
		}
	}
	if err := r.db.SaveProvider(p); err != nil {
		return store.Provider{}, err
	}
	if models, err := r.Test(ctx, p.ID); err == nil {
		p.Models = models
	}
	return r.db.Provider(p.ID)
}

// Update patches a provider; an empty key leaves the stored secret alone.
func (r *Registry) Update(ctx context.Context, id string, in Input) (store.Provider, error) {
	p, err := r.db.Provider(id)
	if err != nil {
		return p, err
	}
	if in.Name != "" {
		p.Name = in.Name
	}
	if in.BaseURL != "" {
		p.BaseURL = in.BaseURL
	}
	if in.AvailableTo != nil {
		p.AvailableTo = in.AvailableTo
	}
	if strings.TrimSpace(in.Key) != "" {
		if p.KeyRef == "" {
			p.KeyRef = "provider:" + p.ID
		}
		if err := r.keys.Set(p.KeyRef, in.Key); err != nil {
			return p, err
		}
		p.KeyMasked = Mask(in.Key)
	}
	if err := r.db.SaveProvider(p); err != nil {
		return p, err
	}
	return r.db.Provider(id)
}

// Delete removes a provider and forgets its secret.
func (r *Registry) Delete(id string) error {
	p, err := r.db.Provider(id)
	if err != nil {
		return err
	}
	if p.KeyRef != "" {
		_ = r.keys.Delete(p.KeyRef)
	}
	return r.db.DeleteProvider(id)
}

// Secret returns the plaintext key for internal use (env injection, tests).
func (r *Registry) Secret(p store.Provider) string {
	if p.KeyRef == "" {
		return ""
	}
	v, err := r.keys.Get(p.KeyRef)
	if err != nil {
		return ""
	}
	return v
}

// Test calls the provider's model listing endpoint, records the outcome and
// stores the models it found. This is what powers "Key works · 4 models".
func (r *Registry) Test(ctx context.Context, id string) ([]store.Model, error) {
	p, err := r.db.Provider(id)
	if err != nil {
		return nil, err
	}
	kind, ok := KindByID(p.Kind)
	if !ok {
		return nil, fmt.Errorf("unknown provider kind %q", p.Kind)
	}
	models, err := r.fetchModels(ctx, kind, p)
	p.LastTestAt = store.Now()
	p.LastTestOK = err == nil
	p.LastTestMsg = ""
	if err != nil {
		p.LastTestMsg = err.Error()
	} else {
		p.LastTestMsg = fmt.Sprintf("%d models", len(models))
	}
	_ = r.db.SaveProvider(p)
	if err != nil {
		return nil, err
	}
	if err := r.db.ReplaceProviderModels(p.ID, models); err != nil {
		return nil, err
	}
	return models, nil
}

func (r *Registry) fetchModels(ctx context.Context, kind Kind, p store.Provider) ([]store.Model, error) {
	base := strings.TrimSuffix(firstNonEmpty(p.BaseURL, kind.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("no base URL configured")
	}
	url := base + kind.ModelsPath
	secret := r.Secret(p)
	if kind.NeedsKey && secret == "" {
		return nil, fmt.Errorf("no API key stored")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	switch kind.Style {
	case "anthropic":
		req.Header.Set("x-api-key", secret)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "google":
		q := req.URL.Query()
		q.Set("key", secret)
		req.URL.RawQuery = q.Encode()
	case "ollama":
		// no auth
	default:
		req.Header.Set("Authorization", "Bearer "+secret)
	}

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach %s: %w", base, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("the key was rejected (%s)", resp.Status)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned %s", base, resp.Status)
	}

	var payload struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			ContextLength int64  `json:"context_length"`
			ContextWindow int64  `json:"context_window"`
		} `json:"data"`
		Models []struct {
			Name        string `json:"name"`
			Model       string `json:"model"`
			DisplayName string `json:"displayName"`
			InputLimit  int64  `json:"inputTokenLimit"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("unexpected response from %s", base)
	}
	var out []store.Model
	for _, m := range payload.Data {
		name := firstNonEmpty(m.ID, m.Name)
		if name == "" {
			continue
		}
		out = append(out, store.Model{Name: name, Ctx: max64(m.ContextLength, m.ContextWindow)})
	}
	for _, m := range payload.Models {
		name := firstNonEmpty(m.Model, m.Name, m.DisplayName)
		if name == "" {
			continue
		}
		name = strings.TrimPrefix(name, "models/")
		out = append(out, store.Model{Name: name, Ctx: m.InputLimit})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no models reported by %s", base)
	}
	return out, nil
}

// Env returns the environment a runtime should be started with, given every
// provider that runtime is allowed to use. This is how an agent reaches a
// provider whose key only the server holds.
func (r *Registry) Env(runtimeID string) []string {
	providers, err := r.db.Providers()
	if err != nil {
		return nil
	}
	var env []string
	for _, p := range providers {
		if !allows(p.AvailableTo, runtimeID) {
			continue
		}
		kind, ok := KindByID(p.Kind)
		if !ok {
			continue
		}
		secret := r.Secret(p)
		if kind.NeedsKey && secret == "" {
			continue
		}
		for _, k := range kind.KeyEnv {
			env = append(env, k+"="+secret)
		}
		base := firstNonEmpty(p.BaseURL, kind.BaseURL)
		for _, k := range kind.BaseURLEnv {
			if base != "" {
				env = append(env, k+"="+base)
			}
		}
	}
	return env
}

// ModelsFor lists the models available to a runtime, labelled by provider.
func (r *Registry) ModelsFor(runtimeID string) []store.Provider {
	providers, err := r.db.Providers()
	if err != nil {
		return nil
	}
	var out []store.Provider
	for _, p := range providers {
		if allows(p.AvailableTo, runtimeID) && len(p.Models) > 0 {
			out = append(out, p)
		}
	}
	return out
}

func allows(list []string, id string) bool {
	for _, v := range list {
		if strings.EqualFold(v, id) {
			return true
		}
	}
	return false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
