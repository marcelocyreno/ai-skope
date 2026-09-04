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
		VersionArgs:    []string{"--version"},
		EffortLevels:   []string{"low", "medium", "high", "max"},
		PromptViaStdin: true,
		Models: []store.Model{
			{Name: "opus-5", Ctx: 1000000},
			{Name: "sonnet-5", Ctx: 1000000},
			{Name: "haiku-4.5", Ctx: 200000},
		},
		Args: func(req TurnRequest) []string {
			// -p is print (non-interactive) mode; stream-json needs --verbose.
			// plan mode keeps the agent read-only: it may inspect the allowed
			// folder but never write to it.
			a := []string{"-p", "--output-format", "stream-json", "--verbose", "--permission-mode", "plan"}
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
		VersionArgs:    []string{"--version"},
		EffortLevels:   []string{"low", "medium", "high"},
		PromptViaStdin: true,
		Models: []store.Model{
			{Name: "gpt-5-codex", Ctx: 400000},
			{Name: "gpt-5", Ctx: 400000},
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
		VersionArgs:    []string{"--version"},
		UsesProvider:   true,
		PromptViaStdin: true,
		Args: func(req TurnRequest) []string {
			a := []string{"run", "--print-logs"}
			if m := qualifiedModel(req); m != "" {
				a = append(a, "--model", m)
			}
			if req.SessionID != "" {
				a = append(a, "--session", req.SessionID)
			}
			return a
		},
	},
	{
		ID: "pi", Name: "pi", Group: "agents", Bin: "pi",
		VersionArgs:    []string{"--version"},
		UsesProvider:   true,
		PromptViaStdin: true,
		Args: func(req TurnRequest) []string {
			a := []string{"--json"}
			if m := qualifiedModel(req); m != "" {
				a = append(a, "--model", m)
			}
			if req.SessionID != "" {
				a = append(a, "--session", req.SessionID)
			}
			return a
		},
	},
	{
		ID: "omp", Name: "omp", Group: "agents", Bin: "omp",
		VersionArgs:    []string{"--version"},
		UsesProvider:   true,
		PromptViaStdin: true,
		Args: func(req TurnRequest) []string {
			a := []string{"--json"}
			if m := qualifiedModel(req); m != "" {
				a = append(a, "--model", m)
			}
			if req.SessionID != "" {
				a = append(a, "--session", req.SessionID)
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
