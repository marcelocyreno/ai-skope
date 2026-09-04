package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"time"
)

// codeAlphabet omits characters that are easy to misread when typed by hand.
const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// NewPairCode mints a one-time pairing code valid for ttl.
func (db *DB) NewPairCode(ttl time.Duration) (string, error) {
	var sb strings.Builder
	for i := 0; i < 8; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(codeAlphabet))))
		if err != nil {
			return "", err
		}
		sb.WriteByte(codeAlphabet[n.Int64()])
	}
	code := sb.String()
	now := Now()
	if _, err := db.sql.Exec(`INSERT INTO pair_codes (code, created_at, expires_at) VALUES (?, ?, ?)`,
		code, now, now+ttl.Milliseconds()); err != nil {
		return "", err
	}
	return code, nil
}

// ErrBadCode is returned when a pairing code is unknown, used or expired.
var ErrBadCode = errors.New("pairing code is invalid or expired")

// HashToken hashes a bearer token for storage; the plaintext is never stored.
func HashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

// RedeemPairCode consumes a pairing code and returns a fresh bearer token
// bound to origin.
func (db *DB) RedeemPairCode(code, origin, label string) (string, *Pairing, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	now := Now()
	var expires, used int64
	err := db.sql.QueryRow(`SELECT expires_at, used_at FROM pair_codes WHERE code = ?`, code).Scan(&expires, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, ErrBadCode
	}
	if err != nil {
		return "", nil, err
	}
	if used != 0 || expires < now {
		return "", nil, ErrBadCode
	}

	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", nil, err
	}
	token := "aiss_" + base64.RawURLEncoding.EncodeToString(raw[:])

	p := &Pairing{ID: NewID(), Origin: origin, Label: label, CreatedAt: now, LastSeenAt: now}
	tx, err := db.sql.Begin()
	if err != nil {
		return "", nil, err
	}
	if _, err := tx.Exec(`UPDATE pair_codes SET used_at = ? WHERE code = ?`, now, code); err != nil {
		tx.Rollback()
		return "", nil, err
	}
	if _, err := tx.Exec(`INSERT INTO pairings (id, token_hash, origin, label, created_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)`, p.ID, HashToken(token), origin, label, now, now); err != nil {
		tx.Rollback()
		return "", nil, err
	}
	if err := tx.Commit(); err != nil {
		return "", nil, err
	}
	return token, p, nil
}

// PairingByToken resolves a bearer token to a live pairing and refreshes its
// last-seen timestamp.
func (db *DB) PairingByToken(token string) (*Pairing, error) {
	var p Pairing
	err := db.sql.QueryRow(`SELECT id, origin, label, created_at, last_seen_at, revoked_at
		FROM pairings WHERE token_hash = ? AND revoked_at = 0`, HashToken(token)).
		Scan(&p.ID, &p.Origin, &p.Label, &p.CreatedAt, &p.LastSeenAt, &p.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_, _ = db.sql.Exec(`UPDATE pairings SET last_seen_at = ? WHERE id = ?`, Now(), p.ID)
	return &p, nil
}

// Pairings lists every pairing, including revoked ones.
func (db *DB) Pairings() ([]Pairing, error) {
	rows, err := db.sql.Query(`SELECT id, origin, label, created_at, last_seen_at, revoked_at
		FROM pairings ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Pairing
	for rows.Next() {
		var p Pairing
		if err := rows.Scan(&p.ID, &p.Origin, &p.Label, &p.CreatedAt, &p.LastSeenAt, &p.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RevokePairing revokes one pairing by id, or all of them when id is empty.
func (db *DB) RevokePairing(id string) (int64, error) {
	var res sql.Result
	var err error
	if id == "" {
		res, err = db.sql.Exec(`UPDATE pairings SET revoked_at = ? WHERE revoked_at = 0`, Now())
	} else {
		res, err = db.sql.Exec(`UPDATE pairings SET revoked_at = ? WHERE id = ? AND revoked_at = 0`, Now(), id)
	}
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// HasPairings reports whether any live pairing exists.
func (db *DB) HasPairings() bool {
	var n int
	_ = db.sql.QueryRow(`SELECT COUNT(*) FROM pairings WHERE revoked_at = 0`).Scan(&n)
	return n > 0
}
