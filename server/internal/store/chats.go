package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

const chatCols = `id, title, url, host, page_title, favicon, runtime, variant, provider, model,
	effort, agent_session, created_at, updated_at, deleted_at`

func scanChat(sc interface{ Scan(...any) error }) (Chat, error) {
	var c Chat
	err := sc.Scan(&c.ID, &c.Title, &c.URL, &c.Host, &c.PageTitle, &c.Favicon, &c.Runtime, &c.Variant,
		&c.Provider, &c.Model, &c.Effort, &c.AgentSession, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt)
	return c, err
}

// ChatFilter narrows a history listing.
type ChatFilter struct {
	URL            string
	Host           string
	Query          string
	IncludeDeleted bool
	Limit          int
}

// Chats lists conversations, newest first. The extension groups them into
// "this page / this site / everywhere" from URL and Host.
func (db *DB) Chats(f ChatFilter) ([]Chat, error) {
	var where []string
	var args []any
	if !f.IncludeDeleted {
		where = append(where, "deleted_at = 0")
	}
	if f.URL != "" {
		where = append(where, "url = ?")
		args = append(args, f.URL)
	}
	if f.Host != "" {
		where = append(where, "host = ?")
		args = append(args, f.Host)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		where = append(where, "(lower(title) LIKE ? OR lower(url) LIKE ? OR lower(model) LIKE ?)")
		like := "%" + strings.ToLower(q) + "%"
		args = append(args, like, like, like)
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	q := `SELECT ` + chatCols + `, (SELECT COUNT(*) FROM messages m WHERE m.chat_id = chats.id) FROM chats`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Chat{}
	for rows.Next() {
		var c Chat
		err := rows.Scan(&c.ID, &c.Title, &c.URL, &c.Host, &c.PageTitle, &c.Favicon, &c.Runtime, &c.Variant,
			&c.Provider, &c.Model, &c.Effort, &c.AgentSession, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
			&c.MessageCount)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Chat looks up a single conversation.
func (db *DB) Chat(id string) (Chat, error) {
	row := db.sql.QueryRow(`SELECT `+chatCols+` FROM chats WHERE id = ?`, id)
	c, err := scanChat(row)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// CreateChat inserts a conversation.
func (db *DB) CreateChat(c Chat) (Chat, error) {
	if c.ID == "" {
		c.ID = NewID()
	}
	c.CreatedAt = Now()
	c.UpdatedAt = c.CreatedAt
	_, err := db.sql.Exec(`INSERT INTO chats (`+chatCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.ID, c.Title, c.URL, c.Host, c.PageTitle, c.Favicon, c.Runtime, c.Variant, c.Provider,
		c.Model, c.Effort, c.AgentSession, c.CreatedAt, c.UpdatedAt, 0)
	return c, err
}

// UpdateChat patches the mutable fields of a conversation.
func (db *DB) UpdateChat(c Chat) error {
	_, err := db.sql.Exec(`UPDATE chats SET title=?, url=?, host=?, page_title=?, favicon=?, runtime=?,
		variant=?, provider=?, model=?, effort=?, agent_session=?, updated_at=? WHERE id=?`,
		c.Title, c.URL, c.Host, c.PageTitle, c.Favicon, c.Runtime, c.Variant, c.Provider, c.Model,
		c.Effort, c.AgentSession, Now(), c.ID)
	return err
}

// TouchChat bumps a conversation's updated_at.
func (db *DB) TouchChat(id string) error {
	_, err := db.sql.Exec(`UPDATE chats SET updated_at = ? WHERE id = ?`, Now(), id)
	return err
}

// SetChatTitle renames a conversation.
func (db *DB) SetChatTitle(id, title string) error {
	_, err := db.sql.Exec(`UPDATE chats SET title = ?, updated_at = ? WHERE id = ?`, title, Now(), id)
	return err
}

// SetChatAgentSession records the agent's own session id, so the next turn
// can resume the same conversation inside the runtime.
func (db *DB) SetChatAgentSession(id, session string) error {
	_, err := db.sql.Exec(`UPDATE chats SET agent_session = ? WHERE id = ?`, session, id)
	return err
}

// SoftDeleteChat marks a conversation deleted; it stays recoverable so the
// UI's Undo works.
func (db *DB) SoftDeleteChat(id string) error {
	res, err := db.sql.Exec(`UPDATE chats SET deleted_at = ? WHERE id = ? AND deleted_at = 0`, Now(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// RestoreChat undoes a soft delete.
func (db *DB) RestoreChat(id string) error {
	res, err := db.sql.Exec(`UPDATE chats SET deleted_at = 0 WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// PurgeChats permanently removes conversations soft-deleted before the cutoff,
// and (when retainMS > 0) conversations untouched for longer than that.
func (db *DB) PurgeChats(deletedBefore, retainMS int64) (int64, error) {
	var total int64
	res, err := db.sql.Exec(`DELETE FROM chats WHERE deleted_at != 0 AND deleted_at < ?`, deletedBefore)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	total += n
	if retainMS > 0 {
		res, err = db.sql.Exec(`DELETE FROM chats WHERE updated_at < ?`, Now()-retainMS)
		if err != nil {
			return total, err
		}
		n, _ = res.RowsAffected()
		total += n
	}
	return total, nil
}

// AddMessage stores a message and its context items.
func (db *DB) AddMessage(m Message) (Message, error) {
	if m.ID == "" {
		m.ID = NewID()
	}
	if m.CreatedAt == 0 {
		m.CreatedAt = Now()
	}
	tools, err := json.Marshal(orEmpty(m.Tools))
	if err != nil {
		return m, err
	}
	usage := "{}"
	if m.Usage != nil {
		b, err := json.Marshal(m.Usage)
		if err != nil {
			return m, err
		}
		usage = string(b)
	}
	tx, err := db.sql.Begin()
	if err != nil {
		return m, err
	}
	if _, err := tx.Exec(`INSERT INTO messages (id, chat_id, role, text, tools, usage, error, model, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		m.ID, m.ChatID, m.Role, m.Text, string(tools), usage, m.Error, m.Model, m.CreatedAt); err != nil {
		tx.Rollback()
		return m, err
	}
	for i := range m.Context {
		it := m.Context[i]
		if it.ID == "" {
			it.ID = NewID()
		}
		payload, err := json.Marshal(it)
		if err != nil {
			tx.Rollback()
			return m, err
		}
		if _, err := tx.Exec(`INSERT INTO context_items (id, message_id, type, payload) VALUES (?,?,?,?)`,
			it.ID, m.ID, it.Type, string(payload)); err != nil {
			tx.Rollback()
			return m, err
		}
		m.Context[i] = it
	}
	if _, err := tx.Exec(`UPDATE chats SET updated_at = ? WHERE id = ?`, Now(), m.ChatID); err != nil {
		tx.Rollback()
		return m, err
	}
	return m, tx.Commit()
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// FinishMessage updates an assistant message once its turn has ended.
func (db *DB) FinishMessage(m Message) error {
	tools, err := json.Marshal(orEmpty(m.Tools))
	if err != nil {
		return err
	}
	usage := "{}"
	if m.Usage != nil {
		b, err := json.Marshal(m.Usage)
		if err != nil {
			return err
		}
		usage = string(b)
	}
	_, err = db.sql.Exec(`UPDATE messages SET text = ?, tools = ?, usage = ?, error = ?, model = ? WHERE id = ?`,
		m.Text, string(tools), usage, m.Error, m.Model, m.ID)
	if err != nil {
		return err
	}
	return db.TouchChat(m.ChatID)
}

// Messages returns a conversation's messages in order, with their context.
func (db *DB) Messages(chatID string) ([]Message, error) {
	rows, err := db.sql.Query(`SELECT id, chat_id, role, text, tools, usage, error, model, created_at
		FROM messages WHERE chat_id = ? ORDER BY created_at, rowid`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var tools, usage string
		if err := rows.Scan(&m.ID, &m.ChatID, &m.Role, &m.Text, &tools, &usage, &m.Error, &m.Model, &m.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(tools), &m.Tools)
		var u Usage
		if err := json.Unmarshal([]byte(usage), &u); err == nil && (u.InputTokens|u.OutputTokens|u.MS) != 0 {
			m.Usage = &u
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		items, err := db.contextItems(out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Context = items
	}
	return out, nil
}

func (db *DB) contextItems(messageID string) ([]ContextItem, error) {
	rows, err := db.sql.Query(`SELECT payload FROM context_items WHERE message_id = ? ORDER BY rowid`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ContextItem{}
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var it ContextItem
		if err := json.Unmarshal([]byte(payload), &it); err != nil {
			continue
		}
		out = append(out, it)
	}
	return out, rows.Err()
}
