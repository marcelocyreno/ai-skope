package files

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai-skope/aiss/internal/config"
	"github.com/ai-skope/aiss/internal/status"
	"github.com/ai-skope/aiss/internal/store"
)

func setup(t *testing.T) (*Guard, *store.DB, string) {
	t.Helper()
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	root := t.TempDir()
	// t.TempDir on macOS lives under /var, a symlink to /private/var; resolve
	// it so the test compares canonical paths the way the guard does.
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	if _, err := db.AddFolder(root, store.AccessRead); err != nil {
		t.Fatal(err)
	}
	return NewGuard(db, config.Default()), db, root
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveInsideFolder(t *testing.T) {
	g, _, root := setup(t)
	p := filepath.Join(root, "docs", "readme.md")
	write(t, p, "# hi")
	got, folder, err := g.Resolve(p)
	if err != nil || got != p || folder.Path != root {
		t.Fatalf("resolve: %v %q %+v", err, got, folder)
	}
}

func TestResolveRejectsEscapes(t *testing.T) {
	g, _, root := setup(t)
	outside := filepath.Join(filepath.Dir(root), "outside.md")
	write(t, outside, "secret")
	t.Cleanup(func() { os.Remove(outside) })

	cases := []string{
		outside,
		filepath.Join(root, "..", filepath.Base(outside)),
		filepath.Join(root, "..", "..", "etc", "passwd"),
		"/etc/passwd",
	}
	for _, c := range cases {
		if _, _, err := g.Resolve(c); err == nil {
			t.Fatalf("expected %q to be refused", c)
		}
	}
}

func TestResolveRejectsSymlinkEscape(t *testing.T) {
	g, _, root := setup(t)
	outside := filepath.Join(t.TempDir(), "secret.md")
	write(t, outside, "secret")
	link := filepath.Join(root, "link.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, _, err := g.Resolve(link); err == nil {
		t.Fatal("a symlink pointing outside the allow-list must be refused")
	}

	// A symlinked *directory* is the classic escape: the name sits inside the
	// folder but resolves outside it.
	dirOutside := t.TempDir()
	write(t, filepath.Join(dirOutside, "x.md"), "secret")
	dlink := filepath.Join(root, "dlink")
	if err := os.Symlink(dirOutside, dlink); err != nil {
		t.Skip()
	}
	if _, _, err := g.Resolve(filepath.Join(dlink, "x.md")); err == nil {
		t.Fatal("a path through a symlinked directory must be refused")
	}
}

func TestDenyList(t *testing.T) {
	g, _, root := setup(t)
	for _, name := range []string{".env", "id_rsa", "deploy.pem"} {
		p := filepath.Join(root, name)
		write(t, p, "secret")
		if _, _, err := g.Resolve(p); err != ErrDenied {
			t.Fatalf("%s must be denied, got %v", name, err)
		}
	}
	nested := filepath.Join(root, ".ssh", "config")
	write(t, nested, "secret")
	if _, _, err := g.Resolve(nested); err != ErrDenied {
		t.Fatal("a file under .ssh must be denied")
	}
}

func TestNoFoldersMeansNoReads(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	g := NewGuard(db, config.Default())
	if _, _, err := g.Resolve("/etc/hosts"); err != ErrNoFolders {
		t.Fatalf("with no allowed folders every read must fail, got %v", err)
	}
}

func TestReadTextAndHTML(t *testing.T) {
	g, _, root := setup(t)
	md := filepath.Join(root, "a.md")
	write(t, md, "# Title\n\nBody text.")
	c, err := g.Read(md)
	if err != nil || !strings.Contains(c.Text, "Body text") {
		t.Fatalf("read markdown: %v %+v", err, c)
	}
	if c.Display == "" || c.Name != "a.md" {
		t.Fatalf("metadata: %+v", c)
	}

	page := filepath.Join(root, "p.html")
	write(t, page, `<html><head><title>Pricing</title></head><body>
		<nav>Skip me</nav><script>var x = 1;</script>
		<h1>Plans</h1><p>Growth is $149.</p><ul><li>25M events</li></ul></body></html>`)
	c, err = g.Read(page)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.Text, "Growth is $149") || !strings.Contains(c.Text, "# Plans") {
		t.Fatalf("html text lost structure: %q", c.Text)
	}
	if strings.Contains(c.Text, "Skip me") || strings.Contains(c.Text, "var x") {
		t.Fatalf("nav/script leaked into text: %q", c.Text)
	}
	if c.Title != "Pricing" {
		t.Fatalf("title: %q", c.Title)
	}
}

func TestReadRefusesBinaryAndTruncates(t *testing.T) {
	g, _, root := setup(t)
	bin := filepath.Join(root, "b.bin")
	write(t, bin, "abc\x00def")
	if _, err := g.Read(bin); err != ErrBinary {
		t.Fatalf("binary must be refused, got %v", err)
	}

	cfg := config.Default()
	cfg.MaxFileBytes = 16
	db, _ := store.OpenMemory()
	defer db.Close()
	if _, err := db.AddFolder(root, store.AccessRead); err != nil {
		t.Fatal(err)
	}
	small := NewGuard(db, cfg)
	big := filepath.Join(root, "big.txt")
	write(t, big, strings.Repeat("x", 100))
	c, err := small.Read(big)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Truncated || len(c.Text) != 16 {
		t.Fatalf("expected truncation at 16 bytes, got %d truncated=%v", len(c.Text), c.Truncated)
	}
}

func TestBrowseHidesIgnoredAndDenied(t *testing.T) {
	g, _, root := setup(t)
	write(t, filepath.Join(root, "keep.md"), "x")
	write(t, filepath.Join(root, ".env"), "x")
	write(t, filepath.Join(root, "node_modules", "dep", "index.js"), "x")
	entries, err := g.Browse(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name == ".env" || e.Name == "node_modules" {
			t.Fatalf("%s must not be listed", e.Name)
		}
	}
	if len(entries) != 1 || entries[0].Name != "keep.md" {
		t.Fatalf("entries: %+v", entries)
	}
}

func TestResolveFileURL(t *testing.T) {
	g, _, root := setup(t)
	p := filepath.Join(root, "my page.html")
	write(t, p, "<p>hi</p>")
	got, _, err := g.ResolveFileURL("file://" + strings.ReplaceAll(p, " ", "%20"))
	if err != nil || got != p {
		t.Fatalf("file URL: %v %q want %q", err, got, p)
	}
	if _, _, err := g.ResolveFileURL("file:///etc/passwd"); err == nil {
		t.Fatal("file URL outside the allow-list must be refused")
	}
}

func TestIgnorerGitignore(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".gitignore"), "*.log\nbuild/\n/only-root.txt\n")
	ig := NewIgnorer(root, config.Default().IgnoreGlobs)
	cases := []struct {
		rel   string
		isDir bool
		want  bool
	}{
		{"app.log", false, true},
		{"src/app.log", false, true},
		{"build", true, true},
		{"only-root.txt", false, true},
		{"src/only-root.txt", false, false},
		{"src/main.go", false, false},
		{"node_modules", true, true},
		{"bundle.min.js", false, true},
	}
	for _, c := range cases {
		if got := ig.Match(c.rel, c.isDir); got != c.want {
			t.Errorf("Match(%q, dir=%v) = %v, want %v", c.rel, c.isDir, got, c.want)
		}
	}
}

func TestPrimaryRootStaysInsideAllowList(t *testing.T) {
	g, _, root := setup(t)
	deep := filepath.Join(root, "pkg", "x.go")
	write(t, deep, "package x")
	got, err := g.PrimaryRoot([]string{"/etc/passwd", deep})
	if err != nil || got != filepath.Dir(deep) {
		t.Fatalf("primary root: %v %q", err, got)
	}
	got, err = g.PrimaryRoot(nil)
	if err != nil || got != root {
		t.Fatalf("fallback root: %v %q", err, got)
	}
}

func TestIndexerIndexesAndPrunes(t *testing.T) {
	g, db, root := setup(t)
	write(t, filepath.Join(root, "README.md"), "The export format writes CSV and JSON per month.")
	write(t, filepath.Join(root, "site", "index.html"), "<h1>Plans</h1><p>Growth is $149.</p>")
	write(t, filepath.Join(root, "node_modules", "dep", "index.js"), "should be ignored")
	write(t, filepath.Join(root, ".env"), "SECRET=1")

	bus := status.NewBus()
	ix := NewIndexer(db, config.Default(), g, bus)
	folders, _ := db.Folders()
	if err := ix.IndexFolder(context.Background(), folders[0]); err != nil {
		t.Fatal(err)
	}

	hits, err := db.SearchFiles("export", 10)
	if err != nil || len(hits) == 0 {
		t.Fatalf("indexed body not searchable: %v %+v", err, hits)
	}
	// HTML is indexed as extracted text.
	if hits, _ := db.SearchFiles("Growth", 10); len(hits) == 0 {
		t.Fatal("html body should be searchable as text")
	}
	// Ignored and denied paths never enter the index.
	all, _ := db.SearchFiles("ignored", 10)
	for _, f := range all {
		if strings.Contains(f.Path, "node_modules") {
			t.Fatal("node_modules must not be indexed")
		}
	}
	if _, err := db.FileByPath(filepath.Join(root, ".env")); err == nil {
		t.Fatal(".env must not be indexed")
	}

	// A deleted file disappears on the next pass.
	if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	folders, _ = db.Folders()
	if err := ix.IndexFolder(context.Background(), folders[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.FileByPath(filepath.Join(root, "README.md")); err == nil {
		t.Fatal("removed file must be pruned from the index")
	}
}
