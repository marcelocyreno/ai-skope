// Package store owns the SQLite database: schema migrations and typed access
// to pairings, settings, runtimes, providers, folders, the file index, chats
// and notes. Everything the user can lose lives here, in one file they can
// back up or delete.
package store

import (
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/base32"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver, no cgo
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// ErrNotFound is returned by lookups that match no row.
var ErrNotFound = errors.New("not found")

// DB wraps the SQLite handle.
//
// The pool is limited to a single connection: this is a local, low-traffic
// server, and one writer removes every "database is locked" class of bug.
// Long operations (indexing) are split into small transactions so they never
// hold the connection for long.
type DB struct {
	sql    *sql.DB
	hasFTS bool
	path   string
}

// Open opens (creating if needed) the database at path and migrates it.
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	sdb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sdb.SetMaxOpenConns(1)
	sdb.SetConnMaxLifetime(0)
	db := &DB{sql: sdb, path: path}
	if err := db.migrate(); err != nil {
		sdb.Close()
		return nil, err
	}
	db.initFTS()
	return db, nil
}

// OpenMemory opens a private in-memory database, used by tests.
func OpenMemory() (*DB, error) {
	sdb, err := sql.Open("sqlite", "file::memory:?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	sdb.SetMaxOpenConns(1)
	db := &DB{sql: sdb, path: ":memory:"}
	if err := db.migrate(); err != nil {
		sdb.Close()
		return nil, err
	}
	db.initFTS()
	return db, nil
}

// SQL exposes the underlying handle for the few places that need it.
func (db *DB) SQL() *sql.DB { return db.sql }

// Path is the database file path.
func (db *DB) Path() string { return db.path }

// HasFTS reports whether full-text search is available; when false, file
// search degrades to a LIKE scan.
func (db *DB) HasFTS() bool { return db.hasFTS }

// Close closes the database.
func (db *DB) Close() error { return db.sql.Close() }

func (db *DB) migrate() error {
	if _, err := db.sql.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return err
	}
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		var seen string
		err := db.sql.QueryRow(`SELECT name FROM schema_migrations WHERE name = ?`, name).Scan(&seen)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := db.sql.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (name, applied_at) VALUES (?, ?)`, name, Now()); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		slog.Info("applied migration", "name", name)
	}
	return nil
}

// initFTS creates the full-text index. FTS5 is compiled into the driver, but
// the server stays usable without it, so a failure only disables the feature.
func (db *DB) initFTS() {
	_, err := db.sql.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS files_fts USING fts5(
		path UNINDEXED, name, body, tokenize='unicode61 remove_diacritics 2'
	)`)
	if err != nil {
		slog.Warn("full-text search unavailable, falling back to substring search", "err", err)
		return
	}
	db.hasFTS = true
}

// Now is the current Unix time in milliseconds, the unit every timestamp
// column uses.
func Now() int64 { return time.Now().UnixMilli() }

var idEnc = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// NewID returns a short, URL-safe, random identifier.
func NewID() string {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is fatal for a server that mints tokens.
		panic("store: crypto/rand unavailable: " + err.Error())
	}
	return idEnc.EncodeToString(b[:])
}
