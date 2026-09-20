# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

AI Skope is two halves plus a design kit:

- **`extension/`** — a Chrome MV3 side-panel extension (Vue 3 + TypeScript, Vite).
  The user *aims* at a page (picks an element, selects text, attaches a local
  file) and asks about it.
- **`server/`** — `aiss`, a Go daemon on `127.0.0.1`. The browser never calls a
  model provider; the server drives **coding agents already installed on the
  machine** (Claude Code, Codex, pi, omp, opencode) and reads only folders the
  user has allowed.
- **`design/`** — the source of truth for the visual system (tokens, six
  palettes, `sk-*` components, previews, prototype).

`docs/SPEC.md` is the product spec, `docs/SERVER-PLAN.md` the server's design,
`docs/runtimes/COMPAT.md` the verified agent command lines, `store/SUBMIT.md`
the Chrome Web Store release ledger and checklist.

## Commands

Everything is driven from the root `Taskfile.yml` (`task` lists all targets).

```
task up            # build both halves, start the server, print a pairing code
task test          # server + extension, unit through end-to-end, no tokens
task restart       # rebuild the server and restart it
task doctor        # diagnose an installation
task folder -- ~/dev
task real          # drive every installed agent through a real turn (costs tokens)
```

Per half:

```
# server/
make build         # ./aiss          make run   # foreground, debug logging
go test ./...      # unit + API integration (fake agents, no network)
go test -race ./...
./scripts/e2e.sh   # real server on a scratch HOME, real HTTP, fake agent
go test ./internal/chat -run TestPackBudget    # one test

# extension/
npm run build      # three Vite passes into dist/
npm run dev        # rebuild on change (still press Reload on the Chrome card)
npm run typecheck  # vue-tsc --noEmit
npx vitest run tests/sse.test.ts -t "reassembles"   # one test
npx playwright test tests/e2e/copy.spec.ts          # one e2e spec
```

`task brave:test` builds, starts the server, and launches Brave on a separate
profile with the extension loaded and paired — the fastest way to see a change
in a real browser without touching your everyday profile.

CI (`.github/workflows/test.yml`) runs `go vet`, `go test -race`, `vue-tsc`,
`vitest` and the extension build. The browser and real-agent suites stay local.

## Architecture

### The turn, end to end

Composer → `POST /v1/chats/{id}/messages` → `chat.Service.Send` →
`chat.Pack` builds one prompt from question + context chips + index hits →
`runtime.Registry` spawns the agent → its JSONL is normalised into events →
SSE back to the panel, which appends deltas to a reactive message.

Event names (`turn.start`, `tool`, `text.delta`, `text.done`, `usage`, `error`,
`turn.end`) are declared in `server/internal/chat/service.go` and mirrored in
`extension/src/api/types.ts`. Changing one means changing both.

### Server invariants

- **Every filesystem read goes through `files.Guard.Resolve`.** Paths are
  canonicalised (symlinks resolved, including through a symlinked parent) and a
  per-segment deny-list refuses keys, credentials and shell history even inside
  an allowed folder. Do not add a read path that bypasses it.
- **Runtime adapters are declarative.** One `runtime.Spec` per agent in
  `internal/runtime/specs.go` says how to ask for a version, build the argv,
  and read the output. Adapting to a new agent release should be a field edit —
  not a change to `proc.go` or `parse.go`. Any such change also updates
  `docs/runtimes/COMPAT.md` (version + exact invocation) and usually a fake in
  `server/testdata/fakes/`.
- **The parser is deliberately tolerant** (`internal/runtime/parse.go`): it
  accepts the union of every agent's shapes and falls back to plain text, so a
  moved field degrades instead of breaking a turn. Agents emit the answer up to
  three times (deltas, assembled message, final result) — the service keeps the
  most granular and drops the rest. A tool call arrives across several frames
  and is stitched by its **id**, never its name.
- **Agents run read-only through tool sets, not plan mode**, with the prompt on
  stdin (never argv, where `ps` would show it), a scrubbed environment plus only
  the credentials the provider registry injects, a working directory inside the
  allow-list, and killed as a process group on cancel.
- **Storage**: one SQLite file (`internal/store`), driver `modernc.org/sqlite`
  — pure Go, which is why `CGO_ENABLED=0` cross-compiles every release target.
  Migrations are embedded numbered SQL files applied forward-only; add a new
  file, never edit an applied one. Provider keys live in the OS keychain
  (encrypted file fallback); the API and DB hold only a masked form.
- Config: `config.yaml` under XDG dirs, overridable with `AISS_HOST`,
  `AISS_PORT`, `AISS_LOG_LEVEL`, `AISS_DEV`. Tests and scripts set scratch XDG
  dirs and `AISS_KEYSTORE=file` so they never touch the real keychain.

### Extension invariants

- **The server connection lives in the side panel document, not the service
  worker.** MV3 stops an idle worker, which would cut a streaming answer off
  mid-sentence. The worker does only what must outlive the panel: opening it,
  keyboard commands, the opt-in context menu.
- **Streaming is `fetch` + `ReadableStream`, not `EventSource`** — `EventSource`
  cannot send `Authorization` and cannot POST. `src/api/sse.ts` parses frames
  for both the turn stream and `/v1/events`.
- **No host permission at install time.** The content script is injected with
  `chrome.scripting.executeScript` only after `chrome.permissions.request` runs
  from a user click; it draws in a Shadow DOM and never writes to the document.
- **The Markdown renderer (`src/pane/markdown.ts`) is hand-written on purpose.**
  Input is escaped *first* and the renderer only emits tags it wrote itself —
  answers carry page content, so there must be no path from input to markup. Do
  not replace it with a library, and keep new syntax inside that discipline.
- State is plain `reactive()` modules in `src/stores/` (no Pinia). Vue is the
  only runtime dependency.
- **Three build passes, three configs**: `vite.config.ts` (the two pages, ES
  modules), `vite.content.config.ts` (content script, single IIFE — MV3 forbids
  a module content script), `vite.worker.config.ts` (service worker, single ES
  file). `manifest.json` lives at `extension/` root and is copied into `dist/`
  by a plugin in the first pass.

### Design kit

`extension/src/styles/{tokens,components,themes}.css` are a **manual copy** of
`design/tokens/*.css` and are currently byte-identical. Change styles in
`design/`, run `node build.mjs` there (it inlines tokens into every preview),
then copy the three files across — or the prototype and the extension drift.

## Conventions

- Conventional commits with a scope naming the half: `feat(extension):`,
  `fix(server):`, `chore(design):`, `docs(store):`. Subjects are plain prose
  sentences; bodies explain *why*, at length when the reason is worth keeping.
- Comments in this codebase explain the reason a thing is the way it is, not
  what the line does. Match that when adding code.
- **Issues** are titled `Area: imperative summary` — the area matching the
  commit scope that would fix it (`Composer:`, `Model switcher:`, `README:`,
  `Server:`). No bare nouns, no "Need improvement": the title says what should
  change.
- **A screenshot is evidence, not a description.** It leads the body, with alt
  text saying what it shows, and an image-only issue gets prose written for it.
  Two shapes, both read off the issues written by hand here:
  - a defect — `## What the screenshot shows` (transcribe the evidence, so the
    issue outlives the image) → `## Why` (root cause, offending code quoted and
    cited `file.ts:87`) → `## Acceptance`
  - a change — `## Today` (what already exists, by `file:line`) → `## Wanted` →
    `## Sketch` (paths, grouped per half when the work spans them) →
    `## Acceptance criteria`

  Add `## Open questions` whenever a decision is the maintainer's — name it
  rather than settling it. `## Resolution` is written by whoever closes the
  issue, never at triage.
- **Issue labels.** Every triaged issue carries one type, at least one area,
  and `triaged`:
  - type — `bug`, `enhancement`, `documentation`, `question`
  - area — `area: extension`, `area: server`, `area: design`, `area: docs`,
    mirroring the commit scopes; an issue that spans halves gets all of them,
    which is the signal that a PR will too
  - `ux` — interaction or interface quality, not a functional defect; sits
    alongside a type, never instead of one
  - `triaged` — the title and body have been reviewed and rewritten. Its
    absence is the work queue: `gh issue list --search "-label:triaged"`
  - `good first issue` — self-contained, one half, no design decision pending
- `/issue-triage` (`.claude/skills/issue-triage/SKILL.md`) is all of the above
  as an executable procedure, including the step easiest to skip: downloading
  an issue's screenshots and looking at them before rewriting a word.
- **Releasing** (see `store/SUBMIT.md` for the full checklist): bump the version
  in `extension/manifest.json`, `extension/package.json` and the root entry of
  `extension/package-lock.json` — Chrome rejects a package that is not strictly
  higher, and only dot-separated integers. Then `task test && task store:package`,
  upload, and move the server tag with it: `git tag vX.Y.Z && git push origin
  vX.Y.Z` triggers goreleaser (root `.goreleaser.yaml`, which also writes the
  Homebrew cask).
