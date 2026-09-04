package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/files"
	"github.com/ai-skope/aiss/internal/provider"
	"github.com/ai-skope/aiss/internal/runtime"
	"github.com/ai-skope/aiss/internal/status"
	"github.com/ai-skope/aiss/internal/store"
	"github.com/ai-skope/aiss/internal/version"
)

func cmdStatus() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	health, err := get("/v1/health")
	if err != nil {
		fmt.Printf("aiss is not running (%s)\n", cfg.BaseURL())
		fmt.Println("Start it with: aiss start")
		return nil
	}
	w := tab()
	fmt.Fprintf(w, "status\trunning\n")
	fmt.Fprintf(w, "address\t%s\n", cfg.BaseURL())
	fmt.Fprintf(w, "version\t%v\n", health["version"])
	if ms, ok := health["uptimeMs"].(float64); ok {
		fmt.Fprintf(w, "uptime\t%s\n", time.Duration(ms)*time.Millisecond)
	}
	fmt.Fprintf(w, "paired\t%v\n", health["paired"])
	w.Flush()

	db, err := openDB()
	if err != nil {
		return nil
	}
	defer db.Close()
	folders, _ := db.Folders()
	fmt.Printf("\nFolders (%d)\n", len(folders))
	w = tab()
	for _, f := range folders {
		fmt.Fprintf(w, "  %s\t%s\t%d files\tindexed %s\n",
			files.Tilde(f.Path), f.Access, f.FileCount, ago(f.LastIndexedAt))
	}
	w.Flush()
	return nil
}

func cmdPair(args []string) error {
	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if len(args) > 0 && args[0] == "--revoke" {
		target := ""
		if len(args) > 1 && args[1] != "all" {
			target = args[1]
		}
		n, err := db.RevokePairing(target)
		if err != nil {
			return err
		}
		fmt.Printf("revoked %d pairing(s)\n", n)
		return nil
	}
	if len(args) > 0 && args[0] == "--list" {
		list, err := db.Pairings()
		if err != nil {
			return err
		}
		w := tab()
		fmt.Fprintln(w, "ID\tORIGIN\tLABEL\tLAST SEEN\tSTATE")
		for _, p := range list {
			state := "active"
			if p.RevokedAt != 0 {
				state = "revoked"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", p.ID, p.Origin, p.Label, ago(p.LastSeenAt), state)
		}
		return w.Flush()
	}

	code, err := db.NewPairCode(10 * time.Minute)
	if err != nil {
		return err
	}
	cfg, _ := loadConfig()
	fmt.Printf("Pairing code: %s\n\n", code)
	fmt.Printf("Open AI Skope in Chrome, go to Settings → Server, point it at\n  %s\nand enter the code. It is valid once, for 10 minutes.\n", cfg.BaseURL())
	return nil
}

func cmdDoctor() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ok := func(b bool) string {
		if b {
			return "OK  "
		}
		return "FAIL"
	}
	fmt.Println("AI Skope Server", version.String())
	fmt.Println()

	// Server reachable?
	running := alive(cfg)
	fmt.Printf("[%s] server on %s\n", ok(running), cfg.BaseURL())
	if !running {
		fmt.Println("       start it with: aiss start")
	}
	fmt.Printf("[%s] host is loopback (%s)\n", ok(cfg.IsLoopback()), cfg.Host)

	db, err := openDB()
	if err != nil {
		fmt.Printf("[FAIL] database at %s: %v\n", config.DBFile(), err)
		return nil
	}
	defer db.Close()
	fmt.Printf("[%s] database %s\n", ok(true), db.Path())
	fmt.Printf("[%s] full-text search available\n", ok(db.HasFTS()))
	if !db.HasFTS() {
		fmt.Println("       file search will fall back to substring matching")
	}

	paired := db.HasPairings()
	fmt.Printf("[%s] a browser is paired\n", ok(paired))
	if !paired {
		fmt.Println("       run: aiss pair")
	}

	keys := provider.NewKeystore(config.StateDir())
	probeErr := keys.Set("doctor-probe", "ok")
	if probeErr == nil {
		_, probeErr = keys.Get("doctor-probe")
		_ = keys.Delete("doctor-probe")
	}
	fmt.Printf("[%s] secret storage (%s)\n", ok(probeErr == nil), keys.Backend())
	if probeErr != nil {
		fmt.Printf("       %v\n", probeErr)
	}

	folders, _ := db.Folders()
	fmt.Printf("[%s] %d folder(s) allowed\n", ok(len(folders) > 0), len(folders))
	if len(folders) == 0 {
		fmt.Println("       add one: aiss folders add ~/dev")
	}
	for _, f := range folders {
		fi, statErr := os.Stat(f.Path)
		readable := statErr == nil && fi.IsDir()
		fmt.Printf("       [%s] %s (%d files, indexed %s)\n",
			ok(readable), files.Tilde(f.Path), f.FileCount, ago(f.LastIndexedAt))
	}

	bus := status.NewBus()
	providers := provider.NewRegistry(db, keys)
	rts := runtime.NewRegistry(db, cfg, providers, bus)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	infos := rts.Detect(ctx)
	installed := 0
	for _, i := range infos {
		if i.Available {
			installed++
		}
	}
	fmt.Printf("[%s] %d runtime(s) installed\n", ok(installed > 0), installed)
	for _, i := range infos {
		fmt.Printf("       [%s] %-12s %s %s\n", ok(i.Available), i.Name,
			strings.TrimSpace(i.Version+" "+i.Path), i.Detail)
	}
	if installed == 0 {
		fmt.Println("       install one of: claude, codex, opencode, pi, omp")
	}

	provs, _ := providers.List()
	fmt.Printf("[%s] %d provider(s) configured\n", ok(true), len(provs))
	for _, p := range provs {
		fmt.Printf("       %-16s %s → %s\n", p.Name, p.KeyMasked, strings.Join(p.AvailableTo, ", "))
	}
	return nil
}

func cmdRuntimes(args []string) error {
	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	cfg, _ := loadConfig()
	reg := runtime.NewRegistry(db, cfg, provider.NewRegistry(db, provider.NewKeystore(config.StateDir())), status.NewBus())

	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "list", "detect":
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		w := tab()
		fmt.Fprintln(w, "ID\tNAME\tVERSION\tPATH\tENABLED\tSTATUS")
		for _, i := range reg.Detect(ctx) {
			detail := i.Status
			if i.Detail != "" {
				detail = i.Detail
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", i.ID, i.Name, i.Version, i.Path, yesNo(i.Enabled), detail)
		}
		return w.Flush()
	case "enable", "disable":
		if len(args) < 2 {
			return fmt.Errorf("usage: aiss runtimes %s <id>", sub)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := reg.SetEnabled(ctx, args[1], sub == "enable", ""); err != nil {
			return err
		}
		fmt.Printf("%s %sd\n", args[1], sub)
		return nil
	case "command":
		if len(args) < 3 {
			return fmt.Errorf("usage: aiss runtimes command <id> <command…>")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := reg.SetEnabled(ctx, args[1], true, strings.Join(args[2:], " ")); err != nil {
			return err
		}
		fmt.Printf("%s now runs: %s\n", args[1], strings.Join(args[2:], " "))
		return nil
	default:
		return fmt.Errorf("unknown runtimes subcommand %q", sub)
	}
}

func cmdProviders(args []string) error {
	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	reg := provider.NewRegistry(db, provider.NewKeystore(config.StateDir()))

	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "list":
		list, err := reg.List()
		if err != nil {
			return err
		}
		w := tab()
		fmt.Fprintln(w, "ID\tKIND\tNAME\tKEY\tAVAILABLE TO\tMODELS\tLAST TEST")
		for _, p := range list {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n", p.ID, p.Kind, p.Name, p.KeyMasked,
				strings.Join(p.AvailableTo, ","), len(p.Models), p.LastTestMsg)
		}
		return w.Flush()
	case "add":
		kind := ""
		name := ""
		baseURL := ""
		var availableTo []string
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--kind":
				i++
				if i < len(args) {
					kind = args[i]
				}
			case "--name":
				i++
				if i < len(args) {
					name = args[i]
				}
			case "--base-url":
				i++
				if i < len(args) {
					baseURL = args[i]
				}
			case "--for":
				i++
				if i < len(args) {
					availableTo = strings.Split(args[i], ",")
				}
			}
		}
		if kind == "" {
			var kinds []string
			for _, k := range provider.Catalog {
				kinds = append(kinds, k.ID)
			}
			return fmt.Errorf("usage: aiss providers add --kind <%s> [--name N] [--base-url U] [--for pi,opencode]",
				strings.Join(kinds, "|"))
		}
		key := os.Getenv("AISS_PROVIDER_KEY")
		if key == "" {
			fmt.Print("API key (input is not echoed to the log): ")
			line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			key = strings.TrimSpace(line)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		p, err := reg.Create(ctx, provider.Input{
			Kind: kind, Name: name, BaseURL: baseURL, Key: key, AvailableTo: availableTo,
		})
		if err != nil {
			return err
		}
		fmt.Printf("added %s (%s) — %d models\n", p.Name, p.ID, len(p.Models))
		return nil
	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("usage: aiss providers remove <id>")
		}
		if err := reg.Delete(args[1]); err != nil {
			return err
		}
		fmt.Println("removed")
		return nil
	case "test":
		if len(args) < 2 {
			return fmt.Errorf("usage: aiss providers test <id>")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		models, err := reg.Test(ctx, args[1])
		if err != nil {
			return err
		}
		fmt.Printf("key works — %d models\n", len(models))
		for _, m := range models {
			fmt.Println("  ", m.Name)
		}
		return nil
	default:
		return fmt.Errorf("unknown providers subcommand %q", sub)
	}
}

func cmdFolders(args []string) error {
	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	cfg, _ := loadConfig()

	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "list":
		list, err := db.Folders()
		if err != nil {
			return err
		}
		w := tab()
		fmt.Fprintln(w, "ID\tPATH\tACCESS\tFILES\tINDEXED")
		for _, f := range list {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", f.ID, files.Tilde(f.Path), f.Access, f.FileCount, ago(f.LastIndexedAt))
		}
		return w.Flush()
	case "add":
		if len(args) < 2 {
			return fmt.Errorf("usage: aiss folders add <path> [--watch]")
		}
		access := store.AccessRead
		for _, a := range args[2:] {
			if a == "--watch" {
				access = store.AccessReadWatch
			}
		}
		abs, err := files.Expand(args[1])
		if err != nil {
			return err
		}
		fi, err := os.Stat(abs)
		if err != nil || !fi.IsDir() {
			return fmt.Errorf("%s is not a folder", filepath.Clean(abs))
		}
		f, err := db.AddFolder(abs, access)
		if err != nil {
			return err
		}
		fmt.Printf("allowed %s (%s)\nindexing…\n", files.Tilde(f.Path), f.Access)
		guard := files.NewGuard(db, cfg)
		ix := files.NewIndexer(db, cfg, guard, status.NewBus())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := ix.IndexFolder(ctx, f); err != nil {
			return err
		}
		n, _ := db.CountFiles(f.ID)
		fmt.Printf("indexed %d files\n", n)
		return nil
	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("usage: aiss folders remove <id>")
		}
		if err := db.DeleteFolder(args[1]); err != nil {
			return err
		}
		fmt.Println("removed")
		return nil
	case "reindex":
		list, err := db.Folders()
		if err != nil {
			return err
		}
		guard := files.NewGuard(db, cfg)
		ix := files.NewIndexer(db, cfg, guard, status.NewBus())
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		for _, f := range list {
			if len(args) > 1 && args[1] != f.ID {
				continue
			}
			if err := ix.IndexFolder(ctx, f); err != nil {
				return err
			}
			n, _ := db.CountFiles(f.ID)
			fmt.Printf("%s: %d files\n", files.Tilde(f.Path), n)
		}
		return nil
	default:
		return fmt.Errorf("unknown folders subcommand %q", sub)
	}
}

func cmdModels(args []string) error {
	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	cfg, _ := loadConfig()
	reg := runtime.NewRegistry(db, cfg, provider.NewRegistry(db, provider.NewKeystore(config.StateDir())), status.NewBus())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if len(args) >= 3 && args[0] == "--set" {
		sel := runtime.Selection{Runtime: args[1], Model: args[2]}
		if err := reg.SetDefault(sel); err != nil {
			return err
		}
		fmt.Printf("default is now %s on %s\n", sel.Model, sel.Runtime)
		return nil
	}
	def := reg.Default(ctx)
	w := tab()
	fmt.Fprintln(w, "RUNTIME\tPROVIDER\tMODEL\tCTX\tSTATUS\tDEFAULT")
	for _, m := range reg.Models(ctx) {
		star := ""
		if m.Runtime == def.Runtime && m.Model == def.Model && m.Provider == def.Provider {
			star = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n", m.Runtime, m.Provider, m.Model, m.Ctx, m.Status, star)
	}
	return w.Flush()
}

func cmdLogs(args []string) error {
	n := 100
	follow := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--tail":
			i++
			if i < len(args) {
				fmt.Sscanf(args[i], "%d", &n)
			}
		case "-f", "--follow":
			follow = true
		}
	}
	path := config.LogFile()
	if follow {
		cmd := exec.Command("tail", "-f", "-n", fmt.Sprint(n), path)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		return cmd.Run()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("no log file yet at %s", path)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	fmt.Println(strings.Join(lines, "\n"))
	return nil
}

func cmdConfig(args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	sub := "show"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "show":
		fmt.Printf("file        %s\n", config.ConfigFile())
		fmt.Printf("host        %s\nport        %d\nlogLevel    %s\n", cfg.Host, cfg.Port, cfg.LogLevel)
		fmt.Printf("maxFile     %d bytes\nmaxContext  %d bytes\n", cfg.MaxFileBytes, cfg.MaxContextBytes)
		fmt.Printf("turnTimeout %s\nprobeEvery  %s\n", cfg.TurnTimeout.D(), cfg.ProbeInterval.D())
		fmt.Printf("passthrough %s\n", strings.Join(cfg.PassthroughEnv, ", "))
		return nil
	case "set":
		if len(args) < 3 {
			return fmt.Errorf("usage: aiss config set <host|port|logLevel|passthroughEnv> <value>")
		}
		switch args[1] {
		case "host":
			cfg.Host = args[2]
		case "port":
			if _, err := fmt.Sscanf(args[2], "%d", &cfg.Port); err != nil {
				return err
			}
		case "logLevel":
			cfg.LogLevel = args[2]
		case "passthroughEnv":
			cfg.PassthroughEnv = strings.Split(args[2], ",")
		default:
			return fmt.Errorf("cannot set %q from the CLI; edit %s", args[1], config.ConfigFile())
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("saved %s\nrestart aiss for it to take effect\n", config.ConfigFile())
		return nil
	default:
		return fmt.Errorf("unknown config subcommand %q", sub)
	}
}
