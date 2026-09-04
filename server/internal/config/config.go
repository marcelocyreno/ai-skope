// Package config loads the server's configuration from config.yaml plus
// AISS_* environment overrides, and writes it back when the CLI changes it.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the whole of the server's tunable behaviour. Every field has a
// working default, so a missing config.yaml is not an error.
type Config struct {
	// Host must stay a loopback address: the server exposes local files and
	// spawns agents, and is never meant to be reachable from the network.
	Host string `yaml:"host"`
	Port int    `yaml:"port"`

	LogLevel string `yaml:"logLevel"` // debug | info | warn | error
	DevMode  bool   `yaml:"devMode"`  // also accept http://localhost:* origins

	// Limits guarding reads and requests.
	MaxFileBytes    int64 `yaml:"maxFileBytes"`
	MaxIndexBytes   int64 `yaml:"maxIndexBytes"`
	MaxRequestBytes int64 `yaml:"maxRequestBytes"`
	MaxContextBytes int   `yaml:"maxContextBytes"`

	// Timeouts.
	TurnTimeout   Duration `yaml:"turnTimeout"`
	ProbeInterval Duration `yaml:"probeInterval"`
	ProbeTimeout  Duration `yaml:"probeTimeout"`

	// IgnoreGlobs are skipped by the indexer in addition to .gitignore.
	IgnoreGlobs []string `yaml:"ignoreGlobs"`
	// DenyGlobs are never readable, even when explicitly requested.
	DenyGlobs []string `yaml:"denyGlobs"`
	// IndexExts get their text content indexed and are readable as text.
	IndexExts []string `yaml:"indexExts"`

	// RuntimeCommands overrides the binary (and flags) used for a runtime id.
	RuntimeCommands map[string]string `yaml:"runtimeCommands"`

	// PassthroughEnv names environment variables the server hands to agents
	// unchanged. Agents otherwise start with a scrubbed environment plus the
	// credentials the provider registry injects, so a key sitting in the
	// user's shell never leaks into a subprocess by accident.
	PassthroughEnv []string `yaml:"passthroughEnv"`
}

// Duration is a YAML-friendly time.Duration ("30s", "5m").
type Duration time.Duration

func (d Duration) D() time.Duration { return time.Duration(d) }

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

func (d Duration) MarshalYAML() (any, error) { return time.Duration(d).String(), nil }

// Default returns the configuration used when nothing is stored on disk.
func Default() Config {
	return Config{
		Host:            "127.0.0.1",
		Port:            7331,
		LogLevel:        "info",
		MaxFileBytes:    2 << 20,
		MaxIndexBytes:   512 << 10,
		MaxRequestBytes: 8 << 20,
		MaxContextBytes: 24000,
		TurnTimeout:     Duration(10 * time.Minute),
		ProbeInterval:   Duration(5 * time.Minute),
		ProbeTimeout:    Duration(10 * time.Second),
		IgnoreGlobs: []string{
			"node_modules", ".git", ".hg", ".svn", "dist", "build", "target",
			".next", ".nuxt", ".venv", "venv", "__pycache__", ".cache",
			"vendor", "*.min.js", "*.min.css", "*.map",
		},
		DenyGlobs: []string{
			".env", ".env.*", "*.pem", "*.key", "id_rsa*", "id_ed25519*",
			".ssh", ".aws", ".gnupg", "*.kdbx", "*.keychain", "credentials",
			".npmrc", ".netrc", "*.p12", "*.pfx",
		},
		IndexExts: []string{
			".md", ".mdx", ".markdown", ".txt", ".html", ".htm", ".rst", ".adoc",
			".json", ".yaml", ".yml", ".toml", ".csv", ".tsv",
			".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rb", ".rs", ".java",
			".c", ".h", ".cc", ".cpp", ".hpp", ".cs", ".php", ".swift", ".kt",
			".sh", ".bash", ".zsh", ".sql", ".css", ".scss",
		},
		RuntimeCommands: map[string]string{},
		PassthroughEnv:  []string{},
	}
}

// Load reads config.yaml (if present) over the defaults, then applies
// AISS_* environment overrides.
func Load() (Config, error) {
	cfg := Default()
	b, err := os.ReadFile(ConfigFile())
	if err == nil {
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return cfg, fmt.Errorf("parse %s: %w", ConfigFile(), err)
		}
	} else if !os.IsNotExist(err) {
		return cfg, err
	}
	cfg.applyEnv()
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 7331
	}
	if cfg.RuntimeCommands == nil {
		cfg.RuntimeCommands = map[string]string{}
	}
	return cfg, nil
}

func (c *Config) applyEnv() {
	if v := os.Getenv("AISS_HOST"); v != "" {
		c.Host = v
	}
	if v := os.Getenv("AISS_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Port = p
		}
	}
	if v := os.Getenv("AISS_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
	if v := os.Getenv("AISS_DEV"); v == "1" || strings.EqualFold(v, "true") {
		c.DevMode = true
	}
}

// Save writes the configuration back to config.yaml.
func (c Config) Save() error {
	if err := EnsureDirs(); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFile(), b, 0o600)
}

// Addr is the host:port the HTTP server listens on.
func (c Config) Addr() string { return fmt.Sprintf("%s:%d", c.Host, c.Port) }

// BaseURL is the address the extension should be pointed at.
func (c Config) BaseURL() string { return fmt.Sprintf("http://%s:%d", c.Host, c.Port) }

// IsLoopback reports whether the configured host is a loopback address. The
// server refuses to start otherwise: it would expose local files to the network.
func (c Config) IsLoopback() bool {
	h := strings.ToLower(c.Host)
	return h == "127.0.0.1" || h == "localhost" || h == "::1" || strings.HasPrefix(h, "127.")
}
