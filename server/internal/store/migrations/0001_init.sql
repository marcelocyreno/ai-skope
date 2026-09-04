CREATE TABLE meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- Extension pairings. Tokens are stored hashed; the plaintext is shown once.
CREATE TABLE pairings (
  id           TEXT PRIMARY KEY,
  token_hash   TEXT NOT NULL UNIQUE,
  origin       TEXT NOT NULL,
  label        TEXT NOT NULL DEFAULT '',
  created_at   INTEGER NOT NULL,
  last_seen_at INTEGER NOT NULL DEFAULT 0,
  revoked_at   INTEGER NOT NULL DEFAULT 0
);

-- One-time pairing codes.
CREATE TABLE pair_codes (
  code       TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  used_at    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- Per-runtime overrides; detection results are not persisted.
CREATE TABLE runtimes (
  id         TEXT PRIMARY KEY,
  enabled    INTEGER NOT NULL DEFAULT 1,
  command    TEXT NOT NULL DEFAULT '',
  updated_at INTEGER NOT NULL
);

-- Providers whose keys the server holds on behalf of the runtimes.
CREATE TABLE providers (
  id            TEXT PRIMARY KEY,
  kind          TEXT NOT NULL,
  name          TEXT NOT NULL,
  base_url      TEXT NOT NULL DEFAULT '',
  key_ref       TEXT NOT NULL DEFAULT '',
  key_masked    TEXT NOT NULL DEFAULT '',
  available_to  TEXT NOT NULL DEFAULT '[]',
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  last_test_at  INTEGER NOT NULL DEFAULT 0,
  last_test_ok  INTEGER NOT NULL DEFAULT 0,
  last_test_msg TEXT NOT NULL DEFAULT ''
);

CREATE TABLE provider_models (
  provider_id TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
  model       TEXT NOT NULL,
  ctx         INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (provider_id, model)
);

-- Folders the server may read. Nothing outside these is ever opened.
CREATE TABLE folders (
  id              TEXT PRIMARY KEY,
  path            TEXT NOT NULL UNIQUE,
  access          TEXT NOT NULL DEFAULT 'read',
  file_count      INTEGER NOT NULL DEFAULT 0,
  last_indexed_at INTEGER NOT NULL DEFAULT 0,
  created_at      INTEGER NOT NULL
);

CREATE TABLE files (
  path       TEXT PRIMARY KEY,
  folder_id  TEXT NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  ext        TEXT NOT NULL DEFAULT '',
  size       INTEGER NOT NULL DEFAULT 0,
  mtime      INTEGER NOT NULL DEFAULT 0,
  is_dir     INTEGER NOT NULL DEFAULT 0,
  indexed_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX files_folder_idx ON files(folder_id);
CREATE INDEX files_name_idx   ON files(name);
CREATE INDEX files_mtime_idx  ON files(mtime DESC);

CREATE TABLE recent_files (
  path    TEXT PRIMARY KEY,
  used_at INTEGER NOT NULL
);

CREATE TABLE chats (
  id            TEXT PRIMARY KEY,
  title         TEXT NOT NULL DEFAULT '',
  url           TEXT NOT NULL DEFAULT '',
  host          TEXT NOT NULL DEFAULT '',
  page_title    TEXT NOT NULL DEFAULT '',
  favicon       TEXT NOT NULL DEFAULT '',
  runtime       TEXT NOT NULL DEFAULT '',
  variant       TEXT NOT NULL DEFAULT '',
  provider      TEXT NOT NULL DEFAULT '',
  model         TEXT NOT NULL DEFAULT '',
  effort        TEXT NOT NULL DEFAULT '',
  agent_session TEXT NOT NULL DEFAULT '',
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  deleted_at    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX chats_url_idx     ON chats(url);
CREATE INDEX chats_host_idx    ON chats(host);
CREATE INDEX chats_updated_idx ON chats(updated_at DESC);

CREATE TABLE messages (
  id         TEXT PRIMARY KEY,
  chat_id    TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  role       TEXT NOT NULL,
  text       TEXT NOT NULL DEFAULT '',
  tools      TEXT NOT NULL DEFAULT '[]',
  usage      TEXT NOT NULL DEFAULT '{}',
  error      TEXT NOT NULL DEFAULT '',
  model      TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE INDEX messages_chat_idx ON messages(chat_id, created_at);

CREATE TABLE context_items (
  id         TEXT PRIMARY KEY,
  message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  type       TEXT NOT NULL,
  payload    TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX context_message_idx ON context_items(message_id);

CREATE TABLE notes (
  id         TEXT PRIMARY KEY,
  url        TEXT NOT NULL DEFAULT '',
  host       TEXT NOT NULL DEFAULT '',
  title      TEXT NOT NULL DEFAULT '',
  favicon    TEXT NOT NULL DEFAULT '',
  quote      TEXT NOT NULL DEFAULT '',
  body       TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  deleted_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX notes_url_idx     ON notes(url);
CREATE INDEX notes_created_idx ON notes(created_at DESC);
