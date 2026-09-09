// Package files owns everything the server is allowed to read from disk: the
// folder allow-list, path resolution, the text index, and the watcher.
//
// Every read in the server funnels through Guard.Resolve. Nothing outside an
// allowed folder is ever opened, and a small deny-list (keys, credentials,
// shell history) is refused even inside one.
package files

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/store"
)

// Errors returned by Resolve. They map to HTTP 403/404 in the API.
var (
	ErrNotAllowed = errors.New("path is outside every allowed folder")
	ErrDenied     = errors.New("path is on the deny list")
	ErrTooLarge   = errors.New("file is larger than the configured limit")
	ErrBinary     = errors.New("file is not text")
	ErrNoFolders  = errors.New("no folders have been allowed yet")
)

// Guard resolves and authorises filesystem paths.
type Guard struct {
	db  *store.DB
	cfg config.Config
}

// NewGuard builds a Guard over the allow-list stored in db.
func NewGuard(db *store.DB, cfg config.Config) *Guard { return &Guard{db: db, cfg: cfg} }

// Expand turns a user-typed path into an absolute one, resolving a leading ~.
func Expand(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("empty path")
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// Tilde renders an absolute path with the home directory abbreviated, the way
// the UI shows it (~/dev/northwind/README.md).
func Tilde(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + string(filepath.Separator) + p[len(home)+1:]
	}
	return p
}

// realPath resolves symlinks. For a path that does not exist yet, the deepest
// existing ancestor is resolved instead, so a dangling name cannot be used to
// smuggle a symlinked parent past the allow-list.
func realPath(p string) (string, error) {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r, nil
	}
	dir, base := filepath.Split(p)
	dir = filepath.Clean(dir)
	if dir == p || dir == "" {
		return "", fmt.Errorf("cannot resolve %s", p)
	}
	rdir, err := realPath(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(rdir, base), nil
}

// within reports whether child is base or lives under it.
func within(base, child string) bool {
	if base == child {
		return true
	}
	rel, err := filepath.Rel(base, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Resolve authorises a path and returns its canonical form plus the allowed
// folder that covers it.
func (g *Guard) Resolve(path string) (string, store.Folder, error) {
	var zero store.Folder
	abs, err := Expand(path)
	if err != nil {
		return "", zero, err
	}
	real, err := realPath(abs)
	if err != nil {
		return "", zero, ErrNotAllowed
	}
	folders, err := g.db.Folders()
	if err != nil {
		return "", zero, err
	}
	if len(folders) == 0 {
		return "", zero, ErrNoFolders
	}
	for _, f := range folders {
		froot, err := realPath(f.Path)
		if err != nil {
			continue
		}
		if !within(froot, real) {
			continue
		}
		if g.Denied(real) {
			return "", zero, ErrDenied
		}
		return real, f, nil
	}
	return "", zero, ErrNotAllowed
}

// ResolveFileURL authorises a file:// URL, which is how the extension asks
// about a local page the browser has open.
func (g *Guard) ResolveFileURL(raw string) (string, store.Folder, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", store.Folder{}, err
	}
	if u.Scheme != "file" {
		return "", store.Folder{}, fmt.Errorf("not a file URL")
	}
	p := u.Path
	if runtime.GOOS == "windows" {
		p = strings.TrimPrefix(p, "/")
	}
	decoded, err := url.PathUnescape(p)
	if err != nil {
		return "", store.Folder{}, err
	}
	return g.Resolve(decoded)
}

// Denied reports whether any segment of the path matches the deny-list. The
// check runs on every segment so an allowed folder cannot contain a readable
// .ssh directory by accident.
func (g *Guard) Denied(path string) bool {
	segs := strings.Split(filepath.ToSlash(path), "/")
	for _, seg := range segs {
		if seg == "" {
			continue
		}
		for _, pat := range g.cfg.DenyGlobs {
			if ok, _ := filepath.Match(pat, seg); ok {
				return true
			}
		}
	}
	return false
}

// Roots returns the allowed folders.
func (g *Guard) Roots() ([]store.Folder, error) { return g.db.Folders() }

// RootPaths returns every allowed folder as a canonical path, for agents
// that take the directories they may read on the command line.
func (g *Guard) RootPaths() []string {
	folders, err := g.db.Folders()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(folders))
	for _, f := range folders {
		p, err := realPath(f.Path)
		if err != nil {
			p = f.Path
		}
		out = append(out, p)
	}
	return out
}

// WorkDirFor picks the directory an agent runs in for a turn. The first path
// that resolves inside the allow-list decides, and the agent runs in the
// project that holds it: the nearest ancestor with a .git entry, up to the
// allowed folder itself. A question about one file then sees the whole
// repository it belongs to, not just its own directory, which is what makes
// "how does this project do X" answerable. With no usable path the first
// allowed folder is used. Agents never run outside the allow-list.
func (g *Guard) WorkDirFor(paths []string) (string, error) {
	for _, p := range paths {
		if p == "" {
			continue
		}
		real, folder, err := g.Resolve(p)
		if err != nil {
			continue
		}
		root, err := realPath(folder.Path)
		if err != nil {
			root = folder.Path
		}
		dir := real
		if fi, err := os.Stat(real); err != nil || !fi.IsDir() {
			dir = filepath.Dir(real)
		}
		return projectRoot(dir, root), nil
	}
	if roots := g.RootPaths(); len(roots) > 0 {
		return roots[0], nil
	}
	return "", ErrNoFolders
}

// projectRoot walks up from dir to root looking for a repository marker and
// returns the first directory that has one, else root.
func projectRoot(dir, root string) string {
	for d := dir; within(root, d); d = filepath.Dir(d) {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		if d == root {
			break
		}
	}
	return root
}
