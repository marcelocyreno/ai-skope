package store

import (
	"database/sql"
	"errors"
	"strings"
)

// NoteFilter narrows a notes listing.
type NoteFilter struct {
	URL   string
	Host  string
	Query string
	Limit int
}

// Notes lists notes, newest first.
func (db *DB) Notes(f NoteFilter) ([]Note, error) {
	where := []string{"deleted_at = 0"}
	var args []any
	if f.URL != "" {
		where = append(where, "url = ?")
		args = append(args, f.URL)
	}
	if f.Host != "" {
		where = append(where, "host = ?")
		args = append(args, f.Host)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		where = append(where, "(lower(body) LIKE ? OR lower(quote) LIKE ? OR lower(title) LIKE ?)")
		like := "%" + strings.ToLower(q) + "%"
		args = append(args, like, like, like)
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	args = append(args, limit)
	rows, err := db.sql.Query(`SELECT id, url, host, title, favicon, quote, body, created_at, updated_at
		FROM notes WHERE `+strings.Join(where, " AND ")+` ORDER BY created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Note{}
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.URL, &n.Host, &n.Title, &n.Favicon, &n.Quote, &n.Body,
			&n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// CreateNote stores a note.
func (db *DB) CreateNote(n Note) (Note, error) {
	if n.ID == "" {
		n.ID = NewID()
	}
	n.CreatedAt = Now()
	n.UpdatedAt = n.CreatedAt
	_, err := db.sql.Exec(`INSERT INTO notes (id, url, host, title, favicon, quote, body, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		n.ID, n.URL, n.Host, n.Title, n.Favicon, n.Quote, n.Body, n.CreatedAt, n.UpdatedAt)
	return n, err
}

// Note looks up one note.
func (db *DB) Note(id string) (Note, error) {
	var n Note
	err := db.sql.QueryRow(`SELECT id, url, host, title, favicon, quote, body, created_at, updated_at
		FROM notes WHERE id = ? AND deleted_at = 0`, id).
		Scan(&n.ID, &n.URL, &n.Host, &n.Title, &n.Favicon, &n.Quote, &n.Body, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return n, ErrNotFound
	}
	return n, err
}

// UpdateNote patches a note's text.
func (db *DB) UpdateNote(n Note) error {
	res, err := db.sql.Exec(`UPDATE notes SET title = ?, quote = ?, body = ?, updated_at = ?
		WHERE id = ? AND deleted_at = 0`, n.Title, n.Quote, n.Body, Now(), n.ID)
	if err != nil {
		return err
	}
	if k, _ := res.RowsAffected(); k == 0 {
		return ErrNotFound
	}
	return nil
}

// SoftDeleteNote marks a note deleted, keeping Undo possible.
func (db *DB) SoftDeleteNote(id string) error {
	res, err := db.sql.Exec(`UPDATE notes SET deleted_at = ? WHERE id = ? AND deleted_at = 0`, Now(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// RestoreNote undoes a soft delete.
func (db *DB) RestoreNote(id string) error {
	_, err := db.sql.Exec(`UPDATE notes SET deleted_at = 0 WHERE id = ?`, id)
	return err
}

// PurgeNotes permanently removes notes soft-deleted before the cutoff.
func (db *DB) PurgeNotes(before int64) (int64, error) {
	res, err := db.sql.Exec(`DELETE FROM notes WHERE deleted_at != 0 AND deleted_at < ?`, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
