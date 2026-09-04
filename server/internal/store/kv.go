package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// Setting reads a server setting, returning def when unset.
func (db *DB) Setting(key, def string) string {
	var v string
	err := db.sql.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err != nil {
		return def
	}
	return v
}

// SetSetting writes a server setting.
func (db *DB) SetSetting(key, value string) error {
	_, err := db.sql.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// AllSettings returns every stored setting.
func (db *DB) AllSettings() (map[string]string, error) {
	rows, err := db.sql.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// SettingJSON decodes a JSON-encoded setting into v.
func (db *DB) SettingJSON(key string, v any) error {
	var raw string
	err := db.sql.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(raw), v)
}

// SetSettingJSON stores v as a JSON-encoded setting.
func (db *DB) SetSettingJSON(key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return db.SetSetting(key, string(b))
}

// Meta reads an internal server value (server id, and similar).
func (db *DB) Meta(key string) string {
	var v string
	if err := db.sql.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v); err != nil {
		return ""
	}
	return v
}

// SetMeta writes an internal server value.
func (db *DB) SetMeta(key, value string) error {
	_, err := db.sql.Exec(`INSERT INTO meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// ServerID returns a stable identifier for this installation, creating it once.
func (db *DB) ServerID() string {
	if id := db.Meta("server_id"); id != "" {
		return id
	}
	id := NewID()
	_ = db.SetMeta("server_id", id)
	return id
}
