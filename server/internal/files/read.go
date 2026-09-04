package files

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Content is the readable form of one file.
type Content struct {
	Path      string `json:"path"`
	Display   string `json:"display"`
	Name      string `json:"name"`
	Ext       string `json:"ext"`
	Size      int64  `json:"size"`
	MTime     int64  `json:"mtime"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated,omitempty"`
	Title     string `json:"title,omitempty"`
}

// Read authorises path, then returns its text. HTML is converted to readable
// text; binary files are refused; anything over the configured cap is
// truncated rather than silently dropped.
func (g *Guard) Read(path string) (Content, error) {
	real, _, err := g.Resolve(path)
	if err != nil {
		return Content{}, err
	}
	fi, err := os.Stat(real)
	if err != nil {
		return Content{}, err
	}
	if fi.IsDir() {
		return Content{}, fmt.Errorf("%s is a directory", Tilde(real))
	}
	limit := g.cfg.MaxFileBytes
	f, err := os.Open(real)
	if err != nil {
		return Content{}, err
	}
	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return Content{}, err
	}
	truncated := int64(len(raw)) > limit
	if truncated {
		raw = raw[:limit]
	}
	if isBinary(raw) {
		return Content{}, ErrBinary
	}

	c := Content{
		Path:      real,
		Display:   Tilde(real),
		Name:      filepath.Base(real),
		Ext:       strings.ToLower(filepath.Ext(real)),
		Size:      fi.Size(),
		MTime:     fi.ModTime().UnixMilli(),
		Truncated: truncated,
	}
	switch c.Ext {
	case ".html", ".htm", ".xhtml":
		text, err := HTMLToText(bytes.NewReader(raw))
		if err != nil {
			return Content{}, err
		}
		c.Text = text
		c.Title = Title(string(raw))
	default:
		c.Text = string(raw)
	}
	return c, nil
}

// isBinary reports whether the buffer looks like binary data: a NUL byte or
// invalid UTF-8 in the first block is enough to refuse it as text.
func isBinary(b []byte) bool {
	head := b
	if len(head) > 8192 {
		head = head[:8192]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return true
	}
	if !utf8.Valid(head) {
		// Trailing bytes of a truncated rune at the boundary are acceptable.
		if len(head) < len(b) {
			for i := len(head) - 1; i >= 0 && i > len(head)-5; i-- {
				if utf8.RuneStart(head[i]) {
					return !utf8.Valid(head[:i])
				}
			}
		}
		return true
	}
	return false
}

// Entry is one item in a directory listing.
type Entry struct {
	Path    string `json:"path"`
	Display string `json:"display"`
	Name    string `json:"name"`
	Ext     string `json:"ext,omitempty"`
	Size    int64  `json:"size"`
	MTime   int64  `json:"mtime"`
	IsDir   bool   `json:"isDir"`
}

// Browse lists one directory inside an allowed folder, applying the ignore
// and deny rules so the picker never shows what cannot be read.
func (g *Guard) Browse(path string) ([]Entry, error) {
	real, folder, err := g.Resolve(path)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(real)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", Tilde(real))
	}
	froot, err := realPath(folder.Path)
	if err != nil {
		froot = folder.Path
	}
	ig := NewIgnorer(froot, g.cfg.IgnoreGlobs)
	des, err := os.ReadDir(real)
	if err != nil {
		return nil, err
	}
	out := []Entry{}
	for _, de := range des {
		full := filepath.Join(real, de.Name())
		rel, _ := filepath.Rel(froot, full)
		if ig.Match(rel, de.IsDir()) || g.Denied(full) {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		out = append(out, Entry{
			Path:    full,
			Display: Tilde(full),
			Name:    de.Name(),
			Ext:     strings.ToLower(filepath.Ext(de.Name())),
			Size:    info.Size(),
			MTime:   info.ModTime().UnixMilli(),
			IsDir:   de.IsDir(),
		})
	}
	return out, nil
}
