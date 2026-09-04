package provider

import "strings"

// Kind describes a provider family: where its API lives, which environment
// variables the agents read, and how to list the models a key unlocks.
type Kind struct {
	ID         string
	Name       string
	BaseURL    string
	NeedsKey   bool
	KeyEnv     []string // env vars set for a runtime that may use this provider
	BaseURLEnv []string
	ModelsPath string // relative to BaseURL
	Style      string // openai | anthropic | google | ollama
}

// Catalog is every provider kind the server understands. "openai-compatible"
// is the escape hatch for anything speaking the OpenAI API at a custom URL.
var Catalog = []Kind{
	{ID: "anthropic", Name: "Anthropic", BaseURL: "https://api.anthropic.com", NeedsKey: true,
		KeyEnv: []string{"ANTHROPIC_API_KEY"}, ModelsPath: "/v1/models", Style: "anthropic"},
	{ID: "openai", Name: "OpenAI", BaseURL: "https://api.openai.com/v1", NeedsKey: true,
		KeyEnv: []string{"OPENAI_API_KEY"}, BaseURLEnv: []string{"OPENAI_BASE_URL"},
		ModelsPath: "/models", Style: "openai"},
	{ID: "google", Name: "Google", BaseURL: "https://generativelanguage.googleapis.com", NeedsKey: true,
		KeyEnv: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}, ModelsPath: "/v1beta/models", Style: "google"},
	{ID: "zai", Name: "z.ai", BaseURL: "https://api.z.ai/api/paas/v4", NeedsKey: true,
		KeyEnv: []string{"ZAI_API_KEY", "ZHIPUAI_API_KEY"}, ModelsPath: "/models", Style: "openai"},
	{ID: "fireworks", Name: "fireworks.ai", BaseURL: "https://api.fireworks.ai/inference/v1", NeedsKey: true,
		KeyEnv: []string{"FIREWORKS_API_KEY"}, ModelsPath: "/models", Style: "openai"},
	{ID: "openrouter", Name: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1", NeedsKey: true,
		KeyEnv: []string{"OPENROUTER_API_KEY"}, ModelsPath: "/models", Style: "openai"},
	{ID: "groq", Name: "Groq", BaseURL: "https://api.groq.com/openai/v1", NeedsKey: true,
		KeyEnv: []string{"GROQ_API_KEY"}, ModelsPath: "/models", Style: "openai"},
	{ID: "together", Name: "Together", BaseURL: "https://api.together.xyz/v1", NeedsKey: true,
		KeyEnv: []string{"TOGETHER_API_KEY"}, ModelsPath: "/models", Style: "openai"},
	{ID: "ollama", Name: "Ollama", BaseURL: "http://localhost:11434", NeedsKey: false,
		BaseURLEnv: []string{"OLLAMA_HOST"}, ModelsPath: "/api/tags", Style: "ollama"},
	{ID: "openai-compatible", Name: "OpenAI-compatible", BaseURL: "", NeedsKey: true,
		KeyEnv: []string{"OPENAI_API_KEY"}, BaseURLEnv: []string{"OPENAI_BASE_URL"},
		ModelsPath: "/models", Style: "openai"},
}

// KindByID looks up a provider kind.
func KindByID(id string) (Kind, bool) {
	for _, k := range Catalog {
		if strings.EqualFold(k.ID, id) {
			return k, true
		}
	}
	return Kind{}, false
}
