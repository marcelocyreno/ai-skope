package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// RuntimeOverrides returns the stored per-runtime preferences by id.
func (db *DB) RuntimeOverrides() (map[string]RuntimeOverride, error) {
	rows, err := db.sql.Query(`SELECT id, enabled, command FROM runtimes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]RuntimeOverride{}
	for rows.Next() {
		var r RuntimeOverride
		var enabled int
		if err := rows.Scan(&r.ID, &enabled, &r.Command); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		out[r.ID] = r
	}
	return out, rows.Err()
}

// SetRuntimeOverride stores the enabled flag and optional custom command.
func (db *DB) SetRuntimeOverride(r RuntimeOverride) error {
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	_, err := db.sql.Exec(`INSERT INTO runtimes (id, enabled, command, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET enabled = excluded.enabled, command = excluded.command, updated_at = excluded.updated_at`,
		r.ID, enabled, r.Command, Now())
	return err
}

func scanProvider(sc interface{ Scan(...any) error }) (Provider, error) {
	var p Provider
	var avail string
	var testOK int
	err := sc.Scan(&p.ID, &p.Kind, &p.Name, &p.BaseURL, &p.KeyRef, &p.KeyMasked, &avail,
		&p.CreatedAt, &p.UpdatedAt, &p.LastTestAt, &testOK, &p.LastTestMsg)
	if err != nil {
		return p, err
	}
	p.LastTestOK = testOK != 0
	if avail != "" {
		_ = json.Unmarshal([]byte(avail), &p.AvailableTo)
	}
	if p.AvailableTo == nil {
		p.AvailableTo = []string{}
	}
	return p, nil
}

const providerCols = `id, kind, name, base_url, key_ref, key_masked, available_to,
	created_at, updated_at, last_test_at, last_test_ok, last_test_msg`

// Providers lists every configured provider with its discovered models.
func (db *DB) Providers() ([]Provider, error) {
	rows, err := db.sql.Query(`SELECT ` + providerCols + ` FROM providers ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Provider
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		models, err := db.ProviderModels(out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Models = models
	}
	return out, nil
}

// Provider looks up one provider by id.
func (db *DB) Provider(id string) (Provider, error) {
	row := db.sql.QueryRow(`SELECT `+providerCols+` FROM providers WHERE id = ?`, id)
	p, err := scanProvider(row)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	p.Models, err = db.ProviderModels(id)
	return p, err
}

// SaveProvider inserts or updates a provider row.
func (db *DB) SaveProvider(p Provider) error {
	if p.AvailableTo == nil {
		p.AvailableTo = []string{}
	}
	avail, err := json.Marshal(p.AvailableTo)
	if err != nil {
		return err
	}
	testOK := 0
	if p.LastTestOK {
		testOK = 1
	}
	if p.CreatedAt == 0 {
		p.CreatedAt = Now()
	}
	p.UpdatedAt = Now()
	_, err = db.sql.Exec(`INSERT INTO providers (`+providerCols+`)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET kind=excluded.kind, name=excluded.name, base_url=excluded.base_url,
			key_ref=excluded.key_ref, key_masked=excluded.key_masked, available_to=excluded.available_to,
			updated_at=excluded.updated_at, last_test_at=excluded.last_test_at,
			last_test_ok=excluded.last_test_ok, last_test_msg=excluded.last_test_msg`,
		p.ID, p.Kind, p.Name, p.BaseURL, p.KeyRef, p.KeyMasked, string(avail),
		p.CreatedAt, p.UpdatedAt, p.LastTestAt, testOK, p.LastTestMsg)
	return err
}

// DeleteProvider removes a provider and its discovered models.
func (db *DB) DeleteProvider(id string) error {
	res, err := db.sql.Exec(`DELETE FROM providers WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ProviderModels lists the models discovered for a provider.
func (db *DB) ProviderModels(id string) ([]Model, error) {
	rows, err := db.sql.Query(`SELECT model, ctx FROM provider_models WHERE provider_id = ? ORDER BY model`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Model{}
	for rows.Next() {
		var m Model
		if err := rows.Scan(&m.Name, &m.Ctx); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ReplaceProviderModels swaps the model list discovered for a provider.
func (db *DB) ReplaceProviderModels(id string, models []Model) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM provider_models WHERE provider_id = ?`, id); err != nil {
		tx.Rollback()
		return err
	}
	for _, m := range models {
		if _, err := tx.Exec(`INSERT INTO provider_models (provider_id, model, ctx) VALUES (?, ?, ?)`,
			id, m.Name, m.Ctx); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
