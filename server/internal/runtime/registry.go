package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/provider"
	"github.com/ai-skope/aiss/internal/status"
	"github.com/ai-skope/aiss/internal/store"
)

// Registry detects the agents installed on this machine, reports their health,
// and starts turns on them.
type Registry struct {
	db        *store.DB
	cfg       config.Config
	providers *provider.Registry
	bus       *status.Bus

	mu    sync.RWMutex
	infos map[string]Info
}

// NewRegistry builds a runtime registry.
func NewRegistry(db *store.DB, cfg config.Config, providers *provider.Registry, bus *status.Bus) *Registry {
	return &Registry{db: db, cfg: cfg, providers: providers, bus: bus, infos: map[string]Info{}}
}

// specs returns the built-in specs plus any custom commands the user configured.
func (r *Registry) specs() []Spec {
	out := append([]Spec{}, Specs...)
	overrides, _ := r.db.RuntimeOverrides()
	for i, s := range out {
		if cmd := r.commandFor(s.ID, overrides); cmd != "" {
			fields := strings.Fields(cmd)
			if len(fields) > 0 {
				out[i].Bin = fields[0]
			}
		}
	}
	// Custom runtimes exist only as stored commands under a "custom:" id.
	for id, ov := range overrides {
		if strings.HasPrefix(id, "custom:") && ov.Command != "" {
			out = append(out, customSpec(id, ov.Command))
		}
	}
	for id, cmd := range r.cfg.RuntimeCommands {
		if strings.HasPrefix(id, "custom:") && cmd != "" {
			out = append(out, customSpec(id, cmd))
		}
	}
	return out
}

func (r *Registry) commandFor(id string, overrides map[string]store.RuntimeOverride) string {
	if ov, ok := overrides[id]; ok && ov.Command != "" {
		return ov.Command
	}
	return r.cfg.RuntimeCommands[id]
}

// Detect probes every runtime: is the binary on PATH, what version, how fast
// does it answer. Results are cached and pushed to subscribers.
func (r *Registry) Detect(ctx context.Context) []Info {
	overrides, _ := r.db.RuntimeOverrides()
	specs := r.specs()
	infos := make([]Info, 0, len(specs))

	for _, s := range specs {
		info := Info{
			ID: s.ID, Name: s.Name, EffortLevels: s.EffortLevels,
			UsesProvider: s.UsesProvider, Enabled: true, Status: StatusOffline,
		}
		if s.Group != "" {
			info.Variants = groupMembers(specs, s.Group)
		}
		if ov, ok := overrides[s.ID]; ok {
			info.Enabled = ov.Enabled
		}
		if s.Bin == "" {
			info.Detail = "no command configured"
			infos = append(infos, info)
			continue
		}
		path, err := exec.LookPath(s.Bin)
		if err != nil {
			info.Detail = fmt.Sprintf("%s is not on PATH", s.Bin)
			infos = append(infos, info)
			continue
		}
		info.Path = path
		info.Available = true

		started := time.Now()
		out, verr := probeVersion(ctx, path, s.VersionArgs, r.cfg.ProbeTimeout.D())
		info.LatencyMS = time.Since(started).Milliseconds()
		if verr != nil {
			info.Status = StatusDegraded
			info.Detail = strings.TrimSpace(firstLine(string(out)))
			if info.Detail == "" {
				info.Detail = verr.Error()
			}
		} else {
			info.Status = StatusOK
			info.Version = parseVersion(string(out))
		}
		if !info.Enabled {
			info.Status = StatusOffline
			info.Detail = "disabled"
		}
		infos = append(infos, info)
	}

	r.mu.Lock()
	for _, i := range infos {
		r.infos[i.ID] = i
	}
	r.mu.Unlock()
	r.bus.Emit("runtime.status", infos)
	return infos
}

func groupMembers(specs []Spec, group string) []string {
	var out []string
	for _, s := range specs {
		if s.Group == group {
			out = append(out, s.ID)
		}
	}
	sort.Strings(out)
	return out
}

// List returns the cached detection results, detecting again when the set of
// runtimes has changed.
//
// The cache matters because probing spawns a process per runtime, but it must
// not hide a runtime that appeared since: `aiss runtimes command …` writes to
// the database from a separate process, and without this the running server
// would ignore it until its next scheduled probe, minutes later.
func (r *Registry) List(ctx context.Context) []Info {
	specs := r.specs()
	r.mu.RLock()
	n := len(r.infos)
	changed := len(specs) != n
	if !changed {
		for _, s := range specs {
			if _, known := r.infos[s.ID]; !known {
				changed = true
				break
			}
		}
	}
	r.mu.RUnlock()
	if n == 0 || changed {
		return r.Detect(ctx)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Info, 0, len(r.infos))
	for _, i := range r.infos {
		out = append(out, i)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

// Info returns one cached runtime description.
func (r *Registry) Info(id string) (Info, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	i, ok := r.infos[id]
	return i, ok
}

// SetEnabled stores whether a runtime may be used, and an optional command.
func (r *Registry) SetEnabled(ctx context.Context, id string, enabled bool, command string) error {
	if err := r.db.SetRuntimeOverride(store.RuntimeOverride{ID: id, Enabled: enabled, Command: command}); err != nil {
		return err
	}
	r.Detect(ctx)
	return nil
}

// ModelOption is one selectable entry in the extension's model switcher.
type ModelOption struct {
	Runtime      string   `json:"runtime"`
	RuntimeName  string   `json:"runtimeName"`
	Group        string   `json:"group,omitempty"`
	Provider     string   `json:"provider,omitempty"`
	Model        string   `json:"model"`
	Label        string   `json:"label"`
	Ctx          int64    `json:"ctx,omitempty"`
	Status       string   `json:"status"`
	LatencyMS    int64    `json:"latencyMs,omitempty"`
	EffortLevels []string `json:"effortLevels,omitempty"`
	Default      bool     `json:"default,omitempty"`
}

// Selection is the stored default model.
type Selection struct {
	Runtime  string `json:"runtime"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model"`
	Effort   string `json:"effort,omitempty"`
}

const defaultModelKey = "default.model"

// Default returns the stored default selection, falling back to the first
// healthy option.
func (r *Registry) Default(ctx context.Context) Selection {
	var sel Selection
	if err := r.db.SettingJSON(defaultModelKey, &sel); err == nil && sel.Runtime != "" {
		return sel
	}
	for _, o := range r.Models(ctx) {
		if o.Status == StatusOK {
			return Selection{Runtime: o.Runtime, Provider: o.Provider, Model: o.Model}
		}
	}
	return sel
}

// SetDefault stores the default selection.
func (r *Registry) SetDefault(sel Selection) error {
	return r.db.SetSettingJSON(defaultModelKey, sel)
}

// Models flattens every runtime and provider into the switcher's list.
func (r *Registry) Models(ctx context.Context) []ModelOption {
	infos := r.List(ctx)
	specByID := map[string]Spec{}
	for _, s := range r.specs() {
		specByID[s.ID] = s
	}
	var def Selection
	_ = r.db.SettingJSON(defaultModelKey, &def)

	out := []ModelOption{}
	for _, info := range infos {
		if !info.Enabled {
			continue
		}
		spec := specByID[info.ID]
		add := func(providerName string, m store.Model) {
			label := m.Name
			if providerName != "" {
				label = providerName + " / " + m.Name
			}
			st := info.Status
			if !info.Available {
				st = StatusOffline
			}
			out = append(out, ModelOption{
				Runtime: info.ID, RuntimeName: info.Name, Group: spec.Group,
				Provider: providerName, Model: m.Name, Label: label, Ctx: m.Ctx,
				Status: st, LatencyMS: info.LatencyMS, EffortLevels: info.EffortLevels,
				Default: def.Runtime == info.ID && def.Model == m.Name && def.Provider == providerName,
			})
		}
		if spec.UsesProvider {
			for _, p := range r.providers.ModelsFor(info.ID) {
				for _, m := range p.Models {
					// The agent addresses a model as <provider>/<model> using
					// its own provider id — which is the kind, not whatever
					// display name the user typed when adding the key.
					add(p.Kind, m)
				}
			}
			continue
		}
		for _, m := range spec.Models {
			add("", m)
		}
	}
	return out
}

// Start begins a turn on a runtime. The caller supplies the prompt and the
// working directory; the registry supplies the scrubbed environment plus the
// credentials this runtime is allowed to use.
func (r *Registry) Start(ctx context.Context, runtimeID string, req TurnRequest) (Turn, error) {
	var spec Spec
	var found bool
	for _, s := range r.specs() {
		if s.ID == runtimeID {
			spec, found = s, true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("unknown runtime %q", runtimeID)
	}
	info, ok := r.Info(runtimeID)
	if !ok {
		r.Detect(ctx)
		info, _ = r.Info(runtimeID)
	}
	if !info.Enabled {
		return nil, fmt.Errorf("%s is disabled", spec.Name)
	}
	if spec.Bin == "" {
		return nil, fmt.Errorf("%s has no command configured", spec.Name)
	}
	bin, err := exec.LookPath(spec.Bin)
	if err != nil {
		return nil, fmt.Errorf("%s is not installed (%s is not on PATH)", spec.Name, spec.Bin)
	}
	if req.Timeout == 0 {
		req.Timeout = r.cfg.TurnTimeout.D()
	}
	req.Env = append(BaseEnv(r.cfg.PassthroughEnv), r.providers.Env(runtimeID)...)
	return spawn(ctx, spec, req, bin)
}

// StartProbes re-detects runtimes on an interval until ctx is done.
func (r *Registry) StartProbes(ctx context.Context) {
	go func() {
		r.Detect(ctx)
		t := time.NewTicker(r.cfg.ProbeInterval.D())
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				r.Detect(ctx)
			}
		}
	}()
}

// probeVersion asks a binary for its version under a hard time bound.
//
// It uses the same process-group handling as a turn: an agent that hangs (or
// that leaves a child holding its output pipe) must not stall detection, which
// runs on a timer and blocks the settings page.
func probeVersion(parent context.Context, path string, args []string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, args...)
	configureProcAttr(cmd)
	cmd.Stdin = strings.NewReader("")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.Bytes(), err
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// parseVersion pulls a version number out of whatever `--version` printed.
func parseVersion(out string) string {
	line := strings.TrimSpace(firstLine(out))
	for _, f := range strings.Fields(line) {
		f = strings.TrimPrefix(f, "v")
		if f == "" {
			continue
		}
		if f[0] >= '0' && f[0] <= '9' && strings.ContainsAny(f, ".") {
			return strings.Trim(f, "(),")
		}
	}
	return line
}

// Label renders a selection the way the transcript shows it.
func (s Selection) Label() string {
	switch {
	case s.Provider != "" && s.Model != "":
		return s.Provider + " / " + s.Model
	case s.Model != "":
		return s.Model
	default:
		return s.Runtime
	}
}
