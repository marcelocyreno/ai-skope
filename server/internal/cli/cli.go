// Package cli is the aiss command line: starting and stopping the server,
// pairing a browser, and managing runtimes, providers and folders.
//
// Commands that change stored state write to the same SQLite database the
// server uses (WAL makes that safe across processes). A running server picks
// the change up on its next pass, or immediately for anything it reads live.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/store"
	"github.com/ai-skope/aiss/internal/version"
)

const usage = `aiss — the AI Skope Server

Usage:
  aiss start [--foreground]      Start the server
  aiss stop                      Stop a running server
  aiss status                    Is it running, and what does it see
  aiss pair [--revoke [id|all]]  Show a one-time pairing code for the extension
  aiss doctor                    Check the installation and explain what to fix
  aiss runtimes <list|detect|enable ID|disable ID|command ID CMD>
  aiss providers <list|add|remove ID|test ID>
  aiss folders <list|add PATH [--watch]|remove ID|reindex ID>
  aiss models [--set RUNTIME MODEL]
  aiss logs [--tail N]
  aiss config <show|set KEY VALUE>
  aiss reset [--yes]             Delete everything it stores, keys included
  aiss version

Everything lives under:
  config  %s
  data    %s
  logs    %s
`

// Main runs the CLI and returns a process exit code.
func Main(args []string) int {
	if len(args) == 0 {
		fmt.Printf(usage, config.ConfigDir(), config.DataDir(), config.StateDir())
		return 2
	}
	cmd, rest := args[0], args[1:]
	var err error
	switch cmd {
	case "start":
		err = cmdStart(rest)
	case "stop":
		err = cmdStop()
	case "status":
		err = cmdStatus()
	case "pair":
		err = cmdPair(rest)
	case "doctor":
		err = cmdDoctor()
	case "runtimes":
		err = cmdRuntimes(rest)
	case "providers":
		err = cmdProviders(rest)
	case "folders":
		err = cmdFolders(rest)
	case "models":
		err = cmdModels(rest)
	case "logs":
		err = cmdLogs(rest)
	case "config":
		err = cmdConfig(rest)
	case "reset":
		err = cmdReset(rest)
	case "version", "--version", "-v":
		fmt.Println("aiss", version.String())
	case "help", "--help", "-h":
		fmt.Printf(usage, config.ConfigDir(), config.DataDir(), config.StateDir())
	default:
		fmt.Fprintf(os.Stderr, "aiss: unknown command %q\n", cmd)
		fmt.Printf(usage, config.ConfigDir(), config.DataDir(), config.StateDir())
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "aiss:", err)
		return 1
	}
	return 0
}

// openDB opens the database the server uses.
func openDB() (*store.DB, error) {
	if err := config.EnsureDirs(); err != nil {
		return nil, err
	}
	return store.Open(config.DBFile())
}

func loadConfig() (config.Config, error) { return config.Load() }

// get calls the running server's API.
func get(path string) (map[string]any, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(cfg.BaseURL() + path)
	if err != nil {
		return nil, fmt.Errorf("the server is not answering on %s (start it with `aiss start`)", cfg.BaseURL())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("unexpected reply from the server: %s", strings.TrimSpace(string(body)))
	}
	return out, nil
}

func tab() *tabwriter.Writer { return tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0) }

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func ago(ms int64) string {
	if ms == 0 {
		return "never"
	}
	d := time.Since(time.UnixMilli(ms))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d d ago", int(d.Hours()/24))
	}
}
