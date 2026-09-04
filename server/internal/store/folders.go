package store

import (
	"database/sql"
	"errors"
	"strings"
)

// Folders lists the allowed folders.
func (db *DB) Folders() ([]Folder, error) {
	rows, err := db.sql.Query(`SELECT id, path, access, file_count, last_indexed_at, created_at
		FROM folders ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Folder{}
	for rows.Next() {
		var f Folder
		if err := rows.Scan(&f.ID, &f.Path, &f.Access, &f.FileCount, &f.LastIndexedAt, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Folder looks up one allowed folder by id.
func (db *DB) Folder(id string) (Folder, error) {
	var f Folder
	err := db.sql.QueryRow(`SELECT id, path, access, file_count, last_indexed_at, created_at
		FROM folders WHERE id = ?`, id).
		Scan(&f.ID, &f.Path, &f.Access, &f.FileCount, &f.LastIndexedAt, &f.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return f, ErrNotFound
	}
	return f, err
}

// AddFolder records a new allowed folder.
func (db *DB) AddFolder(path, access string) (Folder, error) {
	if access != AccessReadWatch {
		access = AccessRead
	}
	f := Folder{ID: NewID(), Path: path, Access: access, CreatedAt: Now()}
	_, err := db.sql.Exec(`INSERT INTO folders (id, path, access, created_at) VALUES (?, ?, ?, ?)`,
		f.ID, f.Path, f.Access, f.CreatedAt)
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		// Adding the same folder twice is not an error; return the existing row.
		var ex Folder
		if e := db.sql.QueryRow(`SELECT id, path, access, file_count, last_indexed_at, created_at
			FROM folders WHERE path = ?`, path).
			Scan(&ex.ID, &ex.Path, &ex.Access, &ex.FileCount, &ex.LastIndexedAt, &ex.CreatedAt); e == nil {
			return ex, nil
		}
	}
	return f, err
}

// UpdateFolderAccess changes a folder's access level.
func (db *DB) UpdateFolderAccess(id, access string) error {
	if access != AccessReadWatch {
		access = AccessRead
	}
	res, err := db.sql.Exec(`UPDATE folders SET access = ? WHERE id = ?`, access, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteFolder removes a folder and everything indexed under it.
func (db *DB) DeleteFolder(id string) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM files_fts WHERE path IN (SELECT path FROM files WHERE folder_id = ?)`, id); err != nil && db.hasFTS {
		tx.Rollback()
		return err
	}
	res, err := tx.Exec(`DELETE FROM folders WHERE id = ?`, id)
	if err != nil {
		tx.Rollback()
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		tx.Rollback()
		return ErrNotFound
	}
	return tx.Commit()
}

// SetFolderIndexed records the outcome of an index pass.
func (db *DB) SetFolderIndexed(id string, count int64, at int64) error {
	_, err := db.sql.Exec(`UPDATE folders SET file_count = ?, last_indexed_at = ? WHERE id = ?`, count, at, id)
	return err
}
