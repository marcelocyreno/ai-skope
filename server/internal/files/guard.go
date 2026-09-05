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

// Roots returns the allowed folders as canonical paths.
func (g *Guard) Roots() ([]store.Folder, error) { return g.db.Folders() }

// PrimaryRoot picks the working directory an agent should run in for a set of
// context paths: the folder holding the first file, else the first allowed
// folder. Agents never get a working directory outside the allow-list.
func (g *Guard) PrimaryRoot(paths []string) (string, error) {
	for _, p := range paths {
		if real, folder, err := g.Resolve(p); err == nil {
			if fi, err := os.Stat(real); err == nil && fi.IsDir() {
				return real, nil
			}
			_ = folder
			return filepath.Dir(real), nil
		}
	}
	folders, err := g.db.Folders()
	if err != nil {
		return "", err
	}
	if len(folders) > 0 {
		return folders[0].Path, nil
	}
	return "", ErrNoFolders
}
