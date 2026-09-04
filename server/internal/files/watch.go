package files

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/status"
	"github.com/ai-skope/aiss/internal/store"
	"github.com/fsnotify/fsnotify"
)

// Watcher keeps folders marked read+watch in sync with the index.
//
// Editors write files in bursts (write, rename, chmod), so changes are
// debounced: a path is reindexed once the writes stop.
type Watcher struct {
	db    *store.DB
	cfg   config.Config
	guard *Guard
	ix    *Indexer
	bus   *status.Bus

	mu      sync.Mutex
	w       *fsnotify.Watcher
	pending map[string]string // path -> folder id
	timer   *time.Timer
	roots   map[string]string // watched dir -> folder id
}

const debounce = 400 * time.Millisecond

// NewWatcher builds a watcher.
func NewWatcher(db *store.DB, cfg config.Config, guard *Guard, ix *Indexer, bus *status.Bus) *Watcher {
	return &Watcher{db: db, cfg: cfg, guard: guard, ix: ix, bus: bus,
		pending: map[string]string{}, roots: map[string]string{}}
}

// Start begins watching and returns once the initial subscriptions are set up.
// It re-syncs the watch list whenever the folder allow-list changes.
func (wt *Watcher) Start(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	wt.w = w
	wt.Sync()

	go func() {
		defer w.Close()
		resync := time.NewTicker(30 * time.Second)
		defer resync.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-resync.C:
				wt.Sync()
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				wt.handle(ev)
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				slog.Warn("watch error", "err", err)
			}
		}
	}()
	return nil
}

// Sync subscribes to every directory under folders marked read+watch and drops
// subscriptions that are no longer allowed.
func (wt *Watcher) Sync() {
	if wt.w == nil {
		return
	}
	folders, err := wt.db.Folders()
	if err != nil {
		return
	}
	want := map[string]string{}
	for _, f := range folders {
		if f.Access != store.AccessReadWatch {
			continue
		}
		root, err := realPath(f.Path)
		if err != nil {
			continue
		}
		ig := NewIgnorer(root, wt.cfg.IgnoreGlobs)
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			if path != root {
				rel, rerr := filepath.Rel(root, path)
				if rerr != nil || ig.Match(rel, true) || wt.guard.Denied(path) {
					return filepath.SkipDir
				}
			}
			want[path] = f.ID
			return nil
		})
	}

	wt.mu.Lock()
	defer wt.mu.Unlock()
	for dir := range wt.roots {
		if _, keep := want[dir]; !keep {
			_ = wt.w.Remove(dir)
			delete(wt.roots, dir)
		}
	}
	for dir, fid := range want {
		if _, have := wt.roots[dir]; have {
			continue
		}
		if err := wt.w.Add(dir); err != nil {
			continue
		}
		wt.roots[dir] = fid
	}
}

func (wt *Watcher) handle(ev fsnotify.Event) {
	if ev.Op&fsnotify.Chmod != 0 && ev.Op == fsnotify.Chmod {
		return
	}
	dir := filepath.Dir(ev.Name)
	wt.mu.Lock()
	fid, watched := wt.roots[dir]
	wt.mu.Unlock()
	if !watched {
		return
	}
	if wt.guard.Denied(ev.Name) {
		return
	}

	if ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		_ = wt.db.DeleteFile(ev.Name)
		_ = wt.db.DeleteFilesUnder(ev.Name + string(filepath.Separator))
		wt.bus.Emit("folder.changed", map[string]any{"folderId": fid, "path": Tilde(ev.Name), "op": "removed"})
		return
	}
	if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
		// A new subdirectory needs its own subscription.
		wt.Sync()
		return
	}

	wt.mu.Lock()
	wt.pending[ev.Name] = fid
	if wt.timer == nil {
		wt.timer = time.AfterFunc(debounce, wt.flush)
	} else {
		wt.timer.Reset(debounce)
	}
	wt.mu.Unlock()
}

func (wt *Watcher) flush() {
	wt.mu.Lock()
	batch := wt.pending
	wt.pending = map[string]string{}
	wt.timer = nil
	wt.mu.Unlock()

	for path, fid := range batch {
		wt.ix.IndexOne(path, fid)
	}
	if len(batch) > 0 {
		wt.bus.Emit("folder.changed", map[string]any{"files": len(batch), "op": "indexed"})
	}
}
