# AI Skope Server (AISS) — implementation plan

Status: plan only, no code yet (2026-09-04). Language: **Go**. Companion to
`docs/SPEC.md` (product) and `design/` (visuals).

## 1. Purpose and principles

AISS is a small local daemon that gives the AI Skope extension three things the
browser cannot do on its own:

1. **Drive coding agents installed on the machine** — Claude Code, Codex, pi,
   omp, opencode — as *runtimes* that answer chats. The agents bring their own
   provider access; AISS never re-implements a model API.
2. **Read local files** the user explicitly allowed (HTML, Markdown, repos,
   notes) so questions about local pages and projects are possible.
3. **Own the user's data** — chats, notes, folders, server-side provider keys —
   in one SQLite file the user can back up or delete.

Principles: local-only by default (binds `127.0.0.1`), explicit allow-lists for
everything that touches the filesystem, a single static binary with no runtime
dependencies, boring standard-library-first Go, and every long operation
streamable and cancellable.

## 2. Architecture

```
Chrome extension (pane + content script)
        │  HTTP/JSON + SSE, Bearer <pairing token>, Origin = chrome-extension://…
        ▼
┌──────────────────────────── aiss (Go) ─────────────────────────────┐
│ api/        HTTP router, auth, CORS, SSE hub, request validation   │
│ chat/       sessions, message assembly, context packing, streaming │
│ runtime/    adapters: claudecode, codex, pi, omp, opencode, custom  │
│ provider/   server-side keys, catalogs, key tests, env injection    │
│ files/      folder allow-list, walker, ignore rules, FTS index,     │
│             fsnotify watcher, safe reads                            │
│ store/      SQLite (modernc, cgo-free): chats, notes, folders, …    │
│ status/     runtime probes, latency, degraded/offline, event bus    │
│ config/     ~/.config/ai-skope/config.yaml, env overrides           │
│ cli/        aiss start|stop|status|pair|doctor|folders|runtimes|logs│
└─────────────────────────────────────────────────────────────────────┘
        │ spawns subprocesses (stdin/stdout JSON), cwd = an allowed folder
        ▼
   claude / codex / pi / omp / opencode  →  their own provider access
```

**Module layout** (single Go module `ai-skope/server`, binary `aiss`):

```
server/
  cmd/aiss/main.go            CLI entry (cobra or stdlib flag + subcommands)
  internal/api/               router.go, auth.go, sse.go, handlers_*.go, dto.go
  internal/chat/              session.go, packer.go (context → prompt), stream.go
  internal/runtime/           runtime.go (interface), registry.go, proc.go (supervisor)
  internal/runtime/claudecode internal/runtime/codex internal/runtime/pi
  internal/runtime/omp        internal/runtime/opencode internal/runtime/custom
  internal/provider/          registry.go, catalog.go, keys.go (keyring+file), test.go
  internal/files/             folders.go, walker.go, ignore.go, index.go, watch.go, read.go, html.go
  internal/store/             db.go, migrations/*.sql, chats.go, notes.go, …
  internal/status/            probe.go, bus.go
  internal/config/            config.go, paths.go
  internal/version/
  testdata/fakes/             fake agent binaries (shell/Go) used in tests
```

**Dependencies** (keep the list short): `modernc.org/sqlite` (no cgo),
`github.com/fsnotify/fsnotify`, `github.com/zalando/go-keyring` (OS keychain,
with encrypted-file fallback), `golang.org/x/net/html` (HTML → text),
`github.com/google/uuid`, `log/slog` (stdlib). HTTP via `net/http` with Go 1.22+
pattern routing. Streaming via **SSE** (simple, proxies well, one direction is
enough; cancel is a POST). WebSocket not needed in v1.

## 3. Security model

- **Bind** `127.0.0.1:7331` only (configurable port). Never `0.0.0.0`.
- **Pairing**: first run prints a one-time 8-character code (`aiss pair` shows a
  new one). The extension posts it to `POST /v1/pair` and receives a long-lived
  bearer token bound to the extension's origin. Tokens stored hashed in SQLite;
  revocable via `aiss pair --revoke`. Everything except `/v1/health` and
  `/v1/pair` requires the token.
- **Origin check**: requests must carry `Origin: chrome-extension://<id>` from
  the allow-list recorded at pairing (plus `http://localhost:*` in dev mode).
  CORS preflight answered only for those origins.
- **Filesystem**: absolute, symlink-resolved (`filepath.EvalSymlinks`) paths must
  sit under an allowed folder; every read re-checks. Deny-list of sensitive
  names always ignored (`.env`, `*.pem`, `id_rsa*`, `.ssh/`, keychains). Size cap
  per read (default 2 MB) and per response.
- **Subprocesses**: runtimes run with `cwd` inside an allowed folder, a scrubbed
  environment (only what the adapter needs + injected provider keys), no shell,
  a per-message timeout, and the agent's own *read-only / plan* permission mode
  where available. Writes by agents are off in v1.
- **Secrets**: provider keys in the OS keychain (fallback: AES-GCM file with a
  key in the keychain); never logged; API responses return masked keys only.
- **Rate limiting** per token on chat endpoints; request body caps.

## 4. HTTP API (v1)

All JSON. Errors: `{ "error": { "code": "folder_not_allowed", "message": "…" } }`.

| Method & path | Purpose |
|---|---|
| `GET /v1/health` | `{status, version, uptime}` — unauthenticated, used by the chip |
| `POST /v1/pair` | `{code, origin}` → `{token, serverId}` |
| `GET /v1/capabilities` | feature flags, limits, api version |
| `GET /v1/events` | **SSE**: `runtime.status`, `index.progress`, `folder.changed`, `server.notice` |
| `GET /v1/runtimes` · `POST /v1/runtimes/detect` · `PATCH /v1/runtimes/{id}` | list (id, name, version, path, enabled, status, latency, effortLevels), re-detect, enable/disable/custom command |
| `GET /v1/models` | flattened: `{runtime, variant?, provider?, model, label, ctx, status, latencyMs, effortLevels[], default}` |
| `PUT /v1/models/default` | default runtime/model/effort |
| `GET/POST/PATCH/DELETE /v1/providers` · `POST /v1/providers/{id}/test` | server-side providers: kind (zai, fireworks, openrouter, groq, together, ollama, openai-compatible), masked key, baseURL, `availableTo[]` runtimes; test returns models found |
| `GET/POST/PATCH/DELETE /v1/folders` · `POST /v1/folders/{id}/reindex` | allow-list with access `read` / `read+watch`, counts, lastIndexed |
| `GET /v1/files/search?q=&limit=` · `GET /v1/files/recent` · `GET /v1/files/browse?path=` · `GET /v1/files/read?path=` | FTS search, recents, directory listing, safe read (text or HTML→text) |
| `POST /v1/files/resolve` | `{url: "file:///…"}` → allowed path or 403 |
| `GET /v1/chats?url=&host=&q=` · `POST /v1/chats` · `GET/PATCH/DELETE /v1/chats/{id}` | history (grouping is client-side from `url`/`host`), create, rename, delete (soft, for undo) |
| `POST /v1/chats/{id}/messages` | send a message → **SSE stream** (below) |
| `POST /v1/chats/{id}/cancel` | cancel the running turn |
| `GET/POST/PATCH/DELETE /v1/notes` | page-linked notes with optional quote |
| `GET/PATCH /v1/settings` | server-side settings (page access default, retention, ignore rules) |
| `GET /v1/logs?tail=` | recent redacted log lines for the settings page |

### Message request

```json
{
  "text": "Is the Growth plan enough for 40M events a month?",
  "page": { "url": "https://…/pricing", "title": "…", "text": "…optional per page-access…" },
  "context": [
    { "type": "element", "selector": "article.pg-tier.featured", "html": "<article…>", "text": "…", "rect": [320, 412] },
    { "type": "text", "quote": "Growth is free for 12 months…", "before": "…", "after": "…" },
    { "type": "file", "path": "/Users/me/dev/northwind/README.md" }
  ],
  "model": { "runtime": "pi", "provider": "z.ai", "model": "GLM 5.3", "effort": "high" }
}
```

### Stream events (SSE `event:` / `data:`)

`turn.start {messageId}` · `tool {name, target, state: running|done, detail}`
(e.g. `Read table.pg-table · 7 rows`) · `text.delta {text}` · `text.done` ·
`usage {inputTokens, outputTokens, ms}` · `error {code, message, retryable}` ·
`turn.end`. The extension renders the tool line, streaming cursor and error row
exactly as designed.

## 5. Runtime adapters

```go
type Runtime interface {
    ID() string                       // "claude-code", "codex", "pi", "omp", "opencode", "custom:<name>"
    Detect(ctx) (Info, error)         // version, path, ok
    Models(ctx) ([]Model, error)      // static catalog + provider registry
    EffortLevels() []string           // nil if unsupported
    Start(ctx, Session) (Turn, error) // spawn/attach, returns a streaming turn
}
type Turn interface { Events() <-chan Event; Cancel() }
```

Each adapter maps a message to the agent's non-interactive/JSON mode, keeps the
agent's own session id for continuity, and normalises its output into the event
stream. Known/expected invocation shapes — **verify each against the installed
version during M1/M3**, flags drift:

| runtime | invocation (expected) | session continuity | effort | notes |
|---|---|---|---|---|
| Claude Code | `claude -p --output-format stream-json --model <m> [--permission-mode plan]` | `--resume <session_id>` | low/medium/high/max via `--effort` if present, else settings | tool events map 1:1 |
| Codex | `codex exec --json --model <m>` | `codex exec resume <id>` | low/medium/high (reasoning effort) | read-only sandbox flag |
| opencode | `opencode run --format json --model <provider>/<model>` | `--session <id>` | provider-dependent | provider registry from opencode config |
| pi | `pi --json …` (confirm) | tbd | provider-dependent | shares provider registry |
| omp | **unknown** — confirm binary and JSON mode | tbd | tbd | treated as pi-family in the design |
| custom | user command, must speak the same JSONL contract | none | none | escape hatch |

Shared pieces: a **process supervisor** (spawn, pipes, timeout, kill on cancel,
exit-code → error), a **JSONL reader**, **model catalogs** (static per runtime
+ `provider/` registry for the pi family, refreshed from provider `/models`
endpoints when a key is present), and a **prompt packer** that turns context
items into a compact preamble (element HTML trimmed to N KB, file paths passed
as paths so the agent reads them itself inside its cwd).

**Working directory**: the allowed folder that contains any file context; else
the first allowed folder; else a per-user scratch dir. Agents never get a cwd
outside the allow-list.

## 6. Provider registry (server-side keys)

- Kinds: `zai`, `fireworks`, `openrouter`, `groq`, `together`, `ollama`,
  `openai-compatible` (custom base URL).
- Stored: id, kind, display name, baseURL, key (keychain ref), `availableTo`
  runtimes, discovered models + timestamps.
- **Injection**: when a runtime in `availableTo` starts, AISS sets the env vars
  that runtime expects (`ZAI_API_KEY`, `FIREWORKS_API_KEY`, `OPENAI_BASE_URL`…)
  and, where an agent needs config files instead (opencode's `opencode.json`,
  pi's config), writes a per-run temp config and points the agent at it — the
  user's own config files are never modified.
- `POST /providers/{id}/test` calls the provider's `/models` (or a 1-token
  completion) and returns the models found; this powers "Key works · 4 models".

## 7. Files: folders, index, reads

- **Allow-list** rows: path, access (`read` | `read+watch`), ignore rules
  (default: `.gitignore` honoured + `node_modules`, `.git`, `dist`, `build`,
  `target`, `.next`, `*.min.*`, binaries).
- **Walker** indexes metadata (path, size, mtime, kind) for everything and text
  for `md, mdx, txt, html, htm, rst, adoc, json, yaml, toml, csv` plus common
  source extensions, capped per file. HTML → text via `x/net/html` (keep
  headings/paragraphs/lists, drop nav/script/style).
- **Index**: SQLite FTS5 (`files_fts(path, title, body)`) — no external search
  engine. Ranking by BM25, boosted by recency and path depth.
- **Watch** (`read+watch` folders): fsnotify with debounce; incremental
  re-index; `index.progress` events for the settings page.
- **Recents**: last opened/attached files per user, used by the picker's
  "Recent" group.
- **Reads** go through one function that enforces the allow-list, symlink
  resolution, deny-list and size cap; used by both the API and the prompt packer.

## 8. Storage (SQLite, single file `~/.local/share/ai-skope/aiss.db`)

Tables: `pairings`, `settings`, `runtimes`, `providers`, `provider_models`,
`folders`, `files`, `files_fts`, `chats` (id, title, url, host, favicon,
runtime, model, effort, created, updated, deleted_at), `messages` (chat_id,
role, text, tool_json, usage_json, created), `context_items` (message_id, type,
payload_json), `notes` (url, title, quote, body, created), `recent_files`.
Migrations embedded (`embed.FS`), applied at start. Soft-delete for chats/notes
to back the UI's Undo; hard-purge after retention.

## 9. Status and health

- Probe each enabled runtime on start and every N minutes (`--version`, then a
  tiny prompt when a key/provider is configured) → `ok / degraded / offline`
  with latency; pushed on `/v1/events`, shown as the chip dot and the offline
  strip ("AI Skope Server isn't reachable" is the extension's own detection of
  `/health` failing).
- `aiss doctor`: checks port, pairing, runtimes on PATH, keychain access,
  folder permissions, index health, and prints fixes.

## 10. CLI, packaging, lifecycle

- `aiss start [--foreground]`, `stop`, `status`, `pair [--revoke]`,
  `folders add|rm|list`, `runtimes list|detect|enable|disable`, `providers …`,
  `logs [-f]`, `doctor`, `version`, `update`.
- Runs as a user service: `launchd` plist on macOS, `systemd --user` on Linux,
  a scheduled task/service on Windows (later).
- Distribution: Homebrew tap (`brew install ai-skope/tap/aiss`), GitHub
  releases (goreleaser, static binaries for darwin/linux/windows), signed
  macOS build. The extension's Setup checklist shows the brew command and waits
  for `/health`.
- Config: `~/.config/ai-skope/config.yaml` (port, log level, ignore rules,
  timeouts); env `AISS_*` overrides; `aiss config get|set`.

## 11. Observability

`slog` JSON logs to `~/.local/state/ai-skope/aiss.log` with rotation; redaction
of keys and file contents; request ids; per-turn timing (spawn, first token,
total). No telemetry leaves the machine.

## 12. Testing strategy

- Unit: adapters against **fake agent binaries** in `testdata/fakes` that emit
  canned JSONL (including malformed lines, slow streams, exit codes); prompt
  packer golden files; path-safety table tests (symlinks, `..`, case, unicode);
  FTS ranking snapshots.
- Integration: spin the server on a random port with a temp DB and temp allowed
  folder; drive the API with `net/http/httptest`; assert SSE sequences.
- Contract: an OpenAPI 3 document generated from the DTOs and checked in;
  the extension's client is generated from it.
- Manual: `aiss doctor` and a `--demo` flag that seeds the design's example
  data (Northwind chats, notes, northwind files) for UI work.

## 13. Milestones

| # | Deliverable | Done when |
|---|---|---|
| M0 | Skeleton: config, SQLite + migrations, `/health`, pairing, auth, CORS, SSE hub, CLI start/stop/status/pair, launchd | extension can pair and see "connected" |
| M1 | Runtime detection + **Claude Code adapter** + chats/messages streaming + cancel | a page question streams an answer through Claude Code |
| M2 | Folders allow-list, walker, FTS index, watch, files API, `file://` resolve, prompt packer with file context | "Add file" works end-to-end on a local Markdown file |
| M3 | Provider registry + keychain + key test + env/config injection; **opencode**, **Codex**, **pi** adapters; `omp` after confirmation | switcher shows z.ai / GLM via pi with effort |
| M4 | Notes, history search, settings API, events for status/index, `doctor`, logs endpoint, `--demo` seed | full settings page is live data |
| M5 | Hardening: rate limits, size caps, redaction audit, goreleaser + Homebrew, signed builds, docs | v0.3 public build |

## 14. Open questions to settle before M1

1. Exact non-interactive flags and effort controls for each agent on the
   installed versions (table in §5) — write a `runtimes/COMPAT.md` as they are
   verified.
2. What **omp** is (binary, JSON mode) — blocks its adapter only.
3. Whether agents may **write** to allowed folders in a later wave (v1: no).
4. Multi-profile / multiple browsers pairing to one server (v1: one token per
   extension origin, several allowed).
5. Windows support timing (v1 targets macOS + Linux).
