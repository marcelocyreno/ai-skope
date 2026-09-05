package runtime

import (
	"strings"

	"github.com/ai-skope/aiss/internal/store"
)

// Spec declares how to drive one agent: how to ask it for its version, how to
// build the argument list for a turn, and how to read its output.
//
// Keeping this declarative means adapting to a new release of an agent is a
// change to a few fields, not to the process machinery. What has actually been
// verified, and against which version, is recorded in docs/runtimes/COMPAT.md.
type Spec struct {
	ID             string
	Name           string
	Group          string // UI grouping; pi/omp/opencode share the provider registry
	Bin            string
	VersionArgs    []string
	EffortLevels   []string
	UsesProvider   bool          // models come from the provider registry
	Models         []store.Model // static catalogue for agents with fixed models
	PromptViaStdin bool          // never put a prompt on argv: it shows up in ps

	// Args builds the command line for one turn.
	Args func(req TurnRequest) []string
	// Parse overrides the tolerant parser for agents needing special handling.
	Parse func(line []byte) []Event
}

func (s Spec) parse(line []byte) []Event {
	if s.Parse != nil {
		return s.Parse(line)
	}
	return parseLine(line)
}

// effortFlag maps the UI's effort level onto an agent flag value.
func effortFlag(effort string) string { return strings.ToLower(strings.TrimSpace(effort)) }

// Specs is every runtime the server knows how to drive.
var Specs = []Spec{
	{
		ID: "claude-code", Name: "Claude Code", Bin: "claude",
		VersionArgs: []string{"--version"},
		// Verified against Claude Code 2.1.261: --effort takes these five
		// levels, and --model takes an alias for the latest model of a family
		// (or a full model name). Aliases keep working as new models ship.
		EffortLevels:   []string{"low", "medium", "high", "xhigh", "max"},
		PromptViaStdin: true,
		Models: []store.Model{
			{Name: "opus", Ctx: 1000000},
			{Name: "sonnet", Ctx: 1000000},
			{Name: "haiku", Ctx: 200000},
		},
		Args: func(req TurnRequest) []string {
			// -p is print (non-interactive) mode; stream-json needs --verbose.
			// plan mode keeps the agent read-only: it may inspect the allowed
			// folder but never write to it.
			// --include-partial-messages turns the answer into token-level
			// deltas rather than one block at the end.
			a := []string{
				"-p", "--output-format", "stream-json", "--verbose",
				"--include-partial-messages", "--permission-mode", "plan",
			}
			if req.Model != "" {
				a = append(a, "--model", req.Model)
			}
			if req.SessionID != "" {
				a = append(a, "--resume", req.SessionID)
			}
			if req.Effort != "" {
				a = append(a, "--effort", effortFlag(req.Effort))
			}
			return a
		},
	},
	{
		ID: "codex", Name: "Codex", Bin: "codex",
		VersionArgs: []string{"--version"},
		// Verified against codex-cli 0.152.1. On a ChatGPT account the model
		// list is fixed by the plan — gpt-5.5 was the only one accepted here,
		// and the cheap lever is model_reasoning_effort rather than a smaller
		// model. An API-key account offers more, so this is a starting list,
		// not a limit: any name the CLI accepts can be selected.
		EffortLevels:   []string{"low", "medium", "high"},
		PromptViaStdin: true,
		Models: []store.Model{
			{Name: "gpt-5.5", Ctx: 400000},
			{Name: "gpt-5.5-codex", Ctx: 400000},
			{Name: "gpt-5.2-codex", Ctx: 400000},
		},
		Args: func(req TurnRequest) []string {
			a := []string{"exec", "--json", "--sandbox", "read-only", "--skip-git-repo-check"}
			if req.SessionID != "" {
				a = []string{"exec", "resume", req.SessionID, "--json", "--sandbox", "read-only", "--skip-git-repo-check"}
			}
			if req.Model != "" {
				a = append(a, "--model", req.Model)
			}
			if req.Effort != "" {
				a = append(a, "-c", "model_reasoning_effort="+effortFlag(req.Effort))
			}
			a = append(a, "-")
			return a
		},
	},
	{
		ID: "opencode", Name: "opencode", Group: "agents", Bin: "opencode",
		VersionArgs: []string{"--version"},
		// Verified against opencode 1.18.20: `run --format json` emits raw
		// events, and --variant carries the provider-specific reasoning
		// effort.
		EffortLevels:   []string{"minimal", "low", "medium", "high", "max"},
		UsesProvider:   true,
		PromptViaStdin: true,
		Args: func(req TurnRequest) []string {
			a := []string{"run", "--format", "json"}
			if m := qualifiedModel(req); m != "" {
				a = append(a, "--model", m)
			}
			if req.SessionID != "" {
				a = append(a, "--session", req.SessionID)
			}
			if req.Effort != "" {
				a = append(a, "--variant", effortFlag(req.Effort))
			}
			return a
		},
	},
	{
		ID: "pi", Name: "pi", Group: "agents", Bin: "pi",
		VersionArgs: []string{"--version"},
		// Verified against pi 0.84.3: -p is non-interactive, --mode json emits
		// the event stream, and --thinking is its effort control. The tool
		// allowlist keeps it read-only, matching Claude Code's plan mode.
		EffortLevels:   []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"},
		UsesProvider:   true,
		PromptViaStdin: true,
		Args: func(req TurnRequest) []string {
			a := []string{"-p", "--mode", "json", "--tools", "read,grep,find,ls"}
			if m := qualifiedModel(req); m != "" {
				a = append(a, "--model", m)
			}
			if req.SessionID != "" {
				a = append(a, "--session-id", req.SessionID)
			}
			if req.Effort != "" {
				a = append(a, "--thinking", effortFlag(req.Effort))
			}
			return a
		},
	},
	{
		ID: "omp", Name: "omp", Group: "agents", Bin: "omp",
		VersionArgs: []string{"--version"},
		// Verified against omp 18.1.6. It shares pi's flags, except that a
		// session is continued with --resume rather than --session-id.
		EffortLevels:   []string{"off", "minimal", "low", "medium", "high", "xhigh", "max", "auto"},
		UsesProvider:   true,
		PromptViaStdin: true,
		Args: func(req TurnRequest) []string {
			// --no-tools rather than an allowlist: omp's tool names depend on
			// which extensions are installed, and naming one it does not have
			// is a hard error. No tools is always valid and always read-only;
			// the context the pane attaches is inlined in the prompt anyway.
			a := []string{"-p", "--mode", "json", "--no-tools"}
			if m := qualifiedModel(req); m != "" {
				a = append(a, "--model", m)
			}
			if req.SessionID != "" {
				a = append(a, "--resume", req.SessionID)
			}
			if req.Effort != "" {
				a = append(a, "--thinking", effortFlag(req.Effort))
			}
			return a
		},
	},
}

// qualifiedModel renders "provider/model" for the agents that address models
// that way, and plain "model" otherwise.
func qualifiedModel(req TurnRequest) string {
	if req.Provider != "" && req.Model != "" {
		return req.Provider + "/" + req.Model
	}
	return req.Model
}

// customSpec builds a spec for a user-supplied command. The command must read
// the prompt on stdin and emit either JSON lines or plain text.
func customSpec(id, command string) Spec {
	fields := strings.Fields(command)
	bin := ""
	var rest []string
	if len(fields) > 0 {
		bin = fields[0]
		rest = fields[1:]
	}
	return Spec{
		ID: id, Name: id, Bin: bin, VersionArgs: []string{"--version"},
		PromptViaStdin: true, UsesProvider: true,
		Args: func(req TurnRequest) []string {
			a := append([]string{}, rest...)
			if req.Model != "" {
				a = append(a, "--model", qualifiedModel(req))
			}
			return a
		},
	}
}

// SpecByID returns a built-in spec.
func SpecByID(id string) (Spec, bool) {
	for _, s := range Specs {
		if s.ID == id {
			return s, true
		}
	}
	return Spec{}, false
}
