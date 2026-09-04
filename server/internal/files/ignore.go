package files

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Ignorer decides which entries the indexer skips: the configured glob list
// plus the .gitignore found at the root of an allowed folder.
//
// The .gitignore support is deliberately simple — plain patterns, directory
// patterns and negations at one level. It exists to keep build output and
// dependencies out of the index, not to be a faithful git implementation.
type Ignorer struct {
	globs   []string
	git     []gitRule
	rootLen int
}

type gitRule struct {
	pattern string
	negate  bool
	dirOnly bool
	rooted  bool
}

// NewIgnorer builds an ignorer for one folder root.
func NewIgnorer(root string, globs []string) *Ignorer {
	ig := &Ignorer{globs: globs, rootLen: len(root)}
	ig.loadGitignore(filepath.Join(root, ".gitignore"))
	return ig
}

func (ig *Ignorer) loadGitignore(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		r := gitRule{pattern: line}
		if strings.HasPrefix(r.pattern, "!") {
			r.negate = true
			r.pattern = r.pattern[1:]
		}
		if strings.HasSuffix(r.pattern, "/") {
			r.dirOnly = true
			r.pattern = strings.TrimSuffix(r.pattern, "/")
		}
		if strings.HasPrefix(r.pattern, "/") {
			r.rooted = true
			r.pattern = strings.TrimPrefix(r.pattern, "/")
		}
		if r.pattern == "" {
			continue
		}
		ig.git = append(ig.git, r)
	}
}

// Match reports whether an entry should be skipped. rel is the path relative
// to the folder root, using forward slashes.
func (ig *Ignorer) Match(rel string, isDir bool) bool {
	rel = filepath.ToSlash(rel)
	base := path_Base(rel)

	for _, g := range ig.globs {
		if ok, _ := filepath.Match(g, base); ok {
			return true
		}
		if strings.Contains(g, "/") {
			if ok, _ := filepath.Match(g, rel); ok {
				return true
			}
		}
	}

	ignored := false
	for _, r := range ig.git {
		if r.dirOnly && !isDir {
			continue
		}
		target := base
		if r.rooted || strings.Contains(r.pattern, "/") {
			target = rel
		}
		ok, _ := filepath.Match(r.pattern, target)
		if !ok && !r.rooted && !strings.Contains(r.pattern, "/") {
			// A bare pattern also matches any parent directory in the path.
			for _, seg := range strings.Split(rel, "/") {
				if m, _ := filepath.Match(r.pattern, seg); m {
					ok = true
					break
				}
			}
		}
		if ok {
			ignored = !r.negate
		}
	}
	return ignored
}

func path_Base(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
