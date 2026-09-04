package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ai-skope/aiss/internal/api"
	"github.com/ai-skope/aiss/internal/chat"
	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/files"
	"github.com/ai-skope/aiss/internal/provider"
	"github.com/ai-skope/aiss/internal/runtime"
	"github.com/ai-skope/aiss/internal/status"
	"github.com/ai-skope/aiss/internal/store"
	"github.com/ai-skope/aiss/internal/version"
)

func cmdStart(args []string) error {
	foreground := false
	for _, a := range args {
		switch a {
		case "--foreground", "-f":
			foreground = true
		default:
			return fmt.Errorf("unknown flag %q for start", a)
		}
	}
	if !foreground {
		return startDetached()
	}
	return serve()
}

// startDetached re-runs this binary in the background and waits for it to
// answer, so `aiss start` returns only once the server is actually up.
func startDetached() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if alive(cfg) {
		fmt.Printf("aiss is already running on %s\n", cfg.BaseURL())
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := config.EnsureDirs(); err != nil {
		return err
	}
	logFile, err := os.OpenFile(config.LogFile(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := exec.Command(exe, "start", "--foreground")
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = os.WriteFile(config.PIDFile(), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if alive(cfg) {
			fmt.Printf("aiss %s listening on %s\n", version.Version, cfg.BaseURL())
			db, err := store.Open(config.DBFile())
			if err == nil {
				defer db.Close()
				if !db.HasPairings() {
					fmt.Println("\nNo browser is paired yet. Run `aiss pair` to get a code.")
				}
			}
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("the server did not come up; see %s", config.LogFile())
}

func alive(cfg config.Config) bool {
	c := &http.Client{Timeout: time.Second}
	resp, err := c.Get(cfg.BaseURL() + "/v1/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// serve runs the server in the foreground until interrupted.
func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !cfg.IsLoopback() {
		return fmt.Errorf("host %q is not a loopback address; aiss exposes local files and refuses to listen on the network", cfg.Host)
	}
	if err := config.EnsureDirs(); err != nil {
		return err
	}
	setupLogging(cfg)

	db, err := store.Open(config.DBFile())
	if err != nil {
		return err
	}
	defer db.Close()

	bus := status.NewBus()
	guard := files.NewGuard(db, cfg)
	indexer := files.NewIndexer(db, cfg, guard, bus)
	keys := provider.NewKeystore(config.StateDir())
	providers := provider.NewRegistry(db, keys)
	runtimes := runtime.NewRegistry(db, cfg, providers, bus)
	watcher := files.NewWatcher(db, cfg, guard, indexer, bus)
	chats := chat.NewService(db, cfg, guard, runtimes, bus)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runtimes.StartProbes(ctx)
	indexer.StartPeriodic(ctx, 15*time.Minute)
	if err := watcher.Start(ctx); err != nil {
		slog.Warn("file watching unavailable", "err", err)
	}
	go purgeLoop(ctx, db)

	srv := &http.Server{
		Addr: cfg.Addr(),
		Handler: api.New(api.Deps{
			DB: db, Cfg: cfg, Guard: guard, Indexer: indexer, Watcher: watcher,
			Providers: providers, Runtimes: runtimes, Chat: chats, Bus: bus,
			Started: time.Now(),
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: chat turns and the event stream are long-lived.
		IdleTimeout: 120 * time.Second,
	}

	ln, err := net.Listen("tcp", cfg.Addr())
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %w", cfg.Addr(), err)
	}
	_ = os.WriteFile(config.PIDFile(), []byte(strconv.Itoa(os.Getpid())), 0o600)
	defer os.Remove(config.PIDFile())

	slog.Info("aiss listening", "addr", cfg.Addr(), "version", version.String(),
		"db", db.Path(), "keystore", keys.Backend(), "fts", db.HasFTS())

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}

func setupLogging(cfg config.Config) {
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	f, err := os.OpenFile(config.LogFile(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	var w *os.File = os.Stderr
	if err == nil {
		w = f
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})))
}

// purgeLoop applies the retention setting to soft-deleted chats and notes.
func purgeLoop(ctx context.Context, db *store.DB) {
	t := time.NewTicker(6 * time.Hour)
	defer t.Stop()
	for {
		purge(db)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func purge(db *store.DB) {
	// Soft-deleted rows are kept for a day so Undo always works, even across
	// a restart.
	cutoff := store.Now() - (24 * time.Hour).Milliseconds()
	var retainMS int64
	if v := db.Setting("privacy.retentionDays", "0"); v != "0" {
		if days, err := strconv.Atoi(v); err == nil && days > 0 {
			retainMS = int64(days) * 24 * time.Hour.Milliseconds()
		}
	}
	if n, err := db.PurgeChats(cutoff, retainMS); err == nil && n > 0 {
		slog.Info("purged chats", "count", n)
	}
	if n, err := db.PurgeNotes(cutoff); err == nil && n > 0 {
		slog.Info("purged notes", "count", n)
	}
}

func cmdStop() error {
	b, err := os.ReadFile(config.PIDFile())
	if err != nil {
		return fmt.Errorf("no pid file at %s; is aiss running?", config.PIDFile())
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return fmt.Errorf("pid file is unreadable: %w", err)
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		os.Remove(config.PIDFile())
		return fmt.Errorf("process %d is not running", pid)
	}
	for i := 0; i < 25; i++ {
		time.Sleep(200 * time.Millisecond)
		if err := p.Signal(syscall.Signal(0)); err != nil {
			fmt.Println("aiss stopped")
			os.Remove(config.PIDFile())
			return nil
		}
	}
	return fmt.Errorf("aiss (pid %d) did not stop", pid)
}
