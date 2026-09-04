package files

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/status"
	"github.com/ai-skope/aiss/internal/store"
)

// Indexer walks the allowed folders and keeps the searchable file index up to
// date. Passes are incremental: unchanged files (same size and mtime) are
// touched, not re-read.
type Indexer struct {
	db    *store.DB
	cfg   config.Config
	guard *Guard
	bus   *status.Bus

	mu      sync.Mutex
	running map[string]bool
}

// NewIndexer builds an indexer.
func NewIndexer(db *store.DB, cfg config.Config, guard *Guard, bus *status.Bus) *Indexer {
	return &Indexer{db: db, cfg: cfg, guard: guard, bus: bus, running: map[string]bool{}}
}

// IndexAll reindexes every allowed folder.
func (ix *Indexer) IndexAll(ctx context.Context) {
	folders, err := ix.db.Folders()
	if err != nil {
		slog.Error("index: list folders", "err", err)
		return
	}
	for _, f := range folders {
		if err := ix.IndexFolder(ctx, f); err != nil && ctx.Err() == nil {
			slog.Error("index folder", "path", f.Path, "err", err)
		}
	}
}

// IndexFolder walks one folder. Concurrent passes over the same folder are
// skipped rather than queued.
func (ix *Indexer) IndexFolder(ctx context.Context, f store.Folder) error {
	ix.mu.Lock()
	if ix.running[f.ID] {
		ix.mu.Unlock()
		return nil
	}
	ix.running[f.ID] = true
	ix.mu.Unlock()
	defer func() {
		ix.mu.Lock()
		delete(ix.running, f.ID)
		ix.mu.Unlock()
	}()

	root, err := realPath(f.Path)
	if err != nil {
		return err
	}
	started := store.Now()
	ig := NewIgnorer(root, ix.cfg.IgnoreGlobs)
	indexExt := map[string]bool{}
	for _, e := range ix.cfg.IndexExts {
		indexExt[e] = true
	}

	ix.bus.Emit("index.progress", map[string]any{"folderId": f.ID, "path": Tilde(f.Path), "state": "started"})

	var count int64
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped, not fatal
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if path == root {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		if ig.Match(rel, d.IsDir()) || ix.guard.Denied(path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}

		rec := store.File{
			Path:     path,
			FolderID: f.ID,
			Name:     d.Name(),
			Ext:      strings.ToLower(filepath.Ext(d.Name())),
			Size:     info.Size(),
			MTime:    info.ModTime().UnixMilli(),
		}
		count++

		// Unchanged files keep their indexed body; only metadata is refreshed.
		if old, err := ix.db.FileByPath(path); err == nil && old.Size == rec.Size && old.MTime == rec.MTime {
			return ix.db.UpsertFileMeta(rec)
		}
		body := ""
		if indexExt[rec.Ext] && rec.Size <= ix.cfg.MaxIndexBytes {
			body = ix.textOf(path, rec.Ext)
		}
		if err := ix.db.UpsertFile(rec, body); err != nil {
			slog.Warn("index file", "path", path, "err", err)
		}
		if count%500 == 0 {
			ix.bus.Emit("index.progress", map[string]any{
				"folderId": f.ID, "path": Tilde(f.Path), "state": "running", "files": count,
			})
		}
		return nil
	})
	if err != nil && ctx.Err() != nil {
		return err
	}

	if perr := ix.db.PruneFolderFiles(f.ID, started); perr != nil {
		slog.Warn("index prune", "folder", f.Path, "err", perr)
	}
	n, _ := ix.db.CountFiles(f.ID)
	_ = ix.db.SetFolderIndexed(f.ID, n, store.Now())
	ix.bus.Emit("index.progress", map[string]any{
		"folderId": f.ID, "path": Tilde(f.Path), "state": "done", "files": n,
		"ms": store.Now() - started,
	})
	slog.Info("indexed folder", "path", f.Path, "files", n, "ms", store.Now()-started)
	return nil
}

// IndexOne refreshes a single file, used by the watcher.
func (ix *Indexer) IndexOne(path string, folderID string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}
	ext := strings.ToLower(filepath.Ext(path))
	body := ""
	for _, e := range ix.cfg.IndexExts {
		if e == ext && info.Size() <= ix.cfg.MaxIndexBytes {
			body = ix.textOf(path, ext)
			break
		}
	}
	_ = ix.db.UpsertFile(store.File{
		Path:     path,
		FolderID: folderID,
		Name:     filepath.Base(path),
		Ext:      ext,
		Size:     info.Size(),
		MTime:    info.ModTime().UnixMilli(),
	}, body)
}

// textOf extracts indexable text, converting HTML and refusing binaries.
func (ix *Indexer) textOf(path, ext string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, ix.cfg.MaxIndexBytes))
	if err != nil || isBinary(raw) {
		return ""
	}
	switch ext {
	case ".html", ".htm", ".xhtml":
		text, err := HTMLToText(strings.NewReader(string(raw)))
		if err != nil {
			return ""
		}
		return text
	default:
		return string(raw)
	}
}

// StartPeriodic reindexes every folder on an interval until ctx is done. A
// first pass runs immediately so a fresh install has a usable picker.
func (ix *Indexer) StartPeriodic(ctx context.Context, every time.Duration) {
	go func() {
		ix.IndexAll(ctx)
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				ix.IndexAll(ctx)
			}
		}
	}()
}
