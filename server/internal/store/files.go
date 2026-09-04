package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
)

// UpsertFile records or refreshes one indexed file, with optional text body
// for the full-text index.
func (db *DB) UpsertFile(f File, body string) error {
	isDir := 0
	if f.IsDir {
		isDir = 1
	}
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO files (path, folder_id, name, ext, size, mtime, is_dir, indexed_at)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(path) DO UPDATE SET folder_id=excluded.folder_id, name=excluded.name, ext=excluded.ext,
			size=excluded.size, mtime=excluded.mtime, is_dir=excluded.is_dir, indexed_at=excluded.indexed_at`,
		f.Path, f.FolderID, f.Name, f.Ext, f.Size, f.MTime, isDir, Now()); err != nil {
		tx.Rollback()
		return err
	}
	if db.hasFTS && !f.IsDir {
		if _, err := tx.Exec(`DELETE FROM files_fts WHERE path = ?`, f.Path); err != nil {
			tx.Rollback()
			return err
		}
		if body != "" {
			if _, err := tx.Exec(`INSERT INTO files_fts (path, name, body) VALUES (?, ?, ?)`,
				f.Path, f.Name, body); err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	return tx.Commit()
}

// DeleteFile drops one path from the index.
func (db *DB) DeleteFile(path string) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	if db.hasFTS {
		if _, err := tx.Exec(`DELETE FROM files_fts WHERE path = ?`, path); err != nil {
			tx.Rollback()
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM files WHERE path = ?`, path); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// DeleteFilesUnder drops every indexed path under a prefix (a removed directory).
func (db *DB) DeleteFilesUnder(prefix string) error {
	like := strings.ReplaceAll(prefix, "%", `\%`) + "%"
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	if db.hasFTS {
		if _, err := tx.Exec(`DELETE FROM files_fts WHERE path LIKE ? ESCAPE '\'`, like); err != nil {
			tx.Rollback()
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM files WHERE path LIKE ? ESCAPE '\'`, like); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// PruneFolderFiles removes indexed rows for a folder that were not touched by
// the latest pass (files deleted while the server was not watching).
func (db *DB) PruneFolderFiles(folderID string, before int64) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	if db.hasFTS {
		if _, err := tx.Exec(`DELETE FROM files_fts WHERE path IN
			(SELECT path FROM files WHERE folder_id = ? AND indexed_at < ?)`, folderID, before); err != nil {
			tx.Rollback()
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM files WHERE folder_id = ? AND indexed_at < ?`, folderID, before); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// CountFiles returns how many entries a folder has indexed.
func (db *DB) CountFiles(folderID string) (int64, error) {
	var n int64
	err := db.sql.QueryRow(`SELECT COUNT(*) FROM files WHERE folder_id = ? AND is_dir = 0`, folderID).Scan(&n)
	return n, err
}

// FileByPath returns one indexed entry.
func (db *DB) FileByPath(path string) (File, error) {
	var f File
	var isDir int
	err := db.sql.QueryRow(`SELECT path, folder_id, name, ext, size, mtime, is_dir FROM files WHERE path = ?`, path).
		Scan(&f.Path, &f.FolderID, &f.Name, &f.Ext, &f.Size, &f.MTime, &isDir)
	if errors.Is(err, sql.ErrNoRows) {
		return f, ErrNotFound
	}
	f.IsDir = isDir != 0
	return f, err
}

// SearchFiles ranks indexed files against a query, using FTS5 when available
// and a substring scan otherwise.
func (db *DB) SearchFiles(q string, limit int) ([]File, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return db.RecentFiles(limit)
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if db.hasFTS {
		if out, err := db.searchFTS(q, limit); err == nil && len(out) > 0 {
			return out, nil
		} else if err != nil && !isFTSSyntaxErr(err) {
			return nil, err
		}
	}
	return db.searchLike(q, limit)
}

func isFTSSyntaxErr(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "fts5")
}

// ftsQuery turns free text into a safe FTS5 prefix query: every term is
// quoted, so user input can never be read as FTS operators.
func ftsQuery(q string) string {
	fields := strings.FieldsFunc(q, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '"' || r == '(' || r == ')' || r == '*' || r == ':'
	})
	var terms []string
	for _, f := range fields {
		if f == "" {
			continue
		}
		terms = append(terms, `"`+f+`"*`)
	}
	return strings.Join(terms, " ")
}

func (db *DB) searchFTS(q string, limit int) ([]File, error) {
	fq := ftsQuery(q)
	if fq == "" {
		return nil, nil
	}
	rows, err := db.sql.Query(`
		SELECT f.path, f.folder_id, f.name, f.ext, f.size, f.mtime,
		       snippet(files_fts, 2, '', '', '…', 12)
		FROM files_fts
		JOIN files f ON f.path = files_fts.path
		WHERE files_fts MATCH ?
		ORDER BY bm25(files_fts, 0.0, 4.0, 1.0), f.mtime DESC
		LIMIT ?`, fq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []File{}
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.Path, &f.FolderID, &f.Name, &f.Ext, &f.Size, &f.MTime, &f.Snippet); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (db *DB) searchLike(q string, limit int) ([]File, error) {
	like := "%" + strings.ToLower(q) + "%"
	rows, err := db.sql.Query(`SELECT path, folder_id, name, ext, size, mtime, is_dir
		FROM files
		WHERE lower(name) LIKE ? OR lower(path) LIKE ?
		ORDER BY is_dir, mtime DESC LIMIT ?`, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []File{}
	for rows.Next() {
		var f File
		var isDir int
		if err := rows.Scan(&f.Path, &f.FolderID, &f.Name, &f.Ext, &f.Size, &f.MTime, &isDir); err != nil {
			return nil, err
		}
		f.IsDir = isDir != 0
		out = append(out, f)
	}
	return out, rows.Err()
}

// TouchRecentFile records that a file was attached or opened.
func (db *DB) TouchRecentFile(path string) error {
	_, err := db.sql.Exec(`INSERT INTO recent_files (path, used_at) VALUES (?, ?)
		ON CONFLICT(path) DO UPDATE SET used_at = excluded.used_at`, path, Now())
	return err
}

// RecentFiles lists the most recently attached files that are still indexed.
func (db *DB) RecentFiles(limit int) ([]File, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	// A file the user attached is recent whether or not the indexer has
	// reached it yet, so this does not require an index row.
	rows, err := db.sql.Query(`SELECT r.path, COALESCE(f.folder_id, ''), COALESCE(f.name, ''),
		COALESCE(f.ext, ''), COALESCE(f.size, 0), COALESCE(f.mtime, 0)
		FROM recent_files r LEFT JOIN files f ON f.path = r.path
		ORDER BY r.used_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []File{}
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.Path, &f.FolderID, &f.Name, &f.Ext, &f.Size, &f.MTime); err != nil {
			return nil, err
		}
		if f.Name == "" {
			f.Name = filepath.Base(f.Path)
			f.Ext = strings.ToLower(filepath.Ext(f.Path))
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Fall back to recently modified indexed files so the picker is never empty.
	if len(out) == 0 {
		return db.newestFiles(limit)
	}
	return out, nil
}

func (db *DB) newestFiles(limit int) ([]File, error) {
	rows, err := db.sql.Query(`SELECT path, folder_id, name, ext, size, mtime
		FROM files WHERE is_dir = 0 ORDER BY mtime DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []File{}
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.Path, &f.FolderID, &f.Name, &f.Ext, &f.Size, &f.MTime); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// UpsertFileMeta refreshes metadata for a file whose content is unchanged,
// leaving the indexed body in place.
func (db *DB) UpsertFileMeta(f File) error {
	isDir := 0
	if f.IsDir {
		isDir = 1
	}
	_, err := db.sql.Exec(`INSERT INTO files (path, folder_id, name, ext, size, mtime, is_dir, indexed_at)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(path) DO UPDATE SET folder_id=excluded.folder_id, name=excluded.name, ext=excluded.ext,
			size=excluded.size, mtime=excluded.mtime, is_dir=excluded.is_dir, indexed_at=excluded.indexed_at`,
		f.Path, f.FolderID, f.Name, f.Ext, f.Size, f.MTime, isDir, Now())
	return err
}
