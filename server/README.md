# AI Skope Server (`aiss`)

The local half of [AI Skope](../README.md). The browser extension does not call
model providers itself: it talks to this server, which

- drives the **coding agents already installed on the machine** (Claude Code,
  Codex, opencode, pi, omp, or a custom command) as the things that answer,
- reads the **folders you allow**, so you can ask about local HTML, Markdown,
  repos and notes,
- owns your **chats, notes and provider keys** in one SQLite file you can back
  up or delete.

It listens on `127.0.0.1` only, and refuses to start on a non-loopback address.

## Install and run

```
make build && ./aiss start      # or: go install ./cmd/aiss
aiss pair                       # shows a one-time code for the extension
aiss folders add ~/dev --watch  # allow a folder and index it
aiss doctor                     # check everything and explain what to fix
```

`aiss start` detaches and waits until the server answers; `--foreground` keeps
it attached. `aiss stop` shuts it down.

| Command | What it does |
|---|---|
| `aiss status` | is it running, which folders are indexed |
| `aiss pair [--list\|--revoke ID]` | pairing codes and paired browsers |
| `aiss runtimes list\|enable ID\|disable ID\|command ID CMD` | the agents it can drive |
| `aiss providers list\|add --kind K --for pi,opencode\|test ID\|remove ID` | provider keys held by the server |
| `aiss folders list\|add PATH [--watch]\|remove ID\|reindex` | the read allow-list |
| `aiss models [--set RUNTIME MODEL]` | what the switcher offers, and the default |
| `aiss logs [-f]`, `aiss config show\|set K V` | diagnostics |

## Layout

```
cmd/aiss              entry point
internal/config       config.yaml + AISS_* overrides
internal/store        SQLite: migrations, chats, notes, folders, index, pairings
internal/files        allow-list guard, HTML→text, indexer, fsnotify watcher
internal/provider     provider catalogue, keychain, model discovery, env injection
internal/runtime      agent specs, process supervisor, tolerant JSONL parser
internal/chat         context packing, turn orchestration, transcripts
internal/api          HTTP + SSE
internal/cli          the aiss command
testdata/fakes        fake agents used by the tests
scripts/e2e.sh        end-to-end test against a real server
```

## Security

- **Loopback only**, bearer token from a one-time pairing code, and the paired
  `Origin` is checked on every request, so a page on the open web cannot use
  the server even with a token.
- **Nothing outside an allowed folder is ever read.** Paths are canonicalised
  (symlinks resolved, including through a symlinked parent) and a per-segment
  deny-list refuses keys, credentials and shell history even inside an allowed
  folder.
- **Prompts go to agents on stdin**, never on argv where `ps` would show them.
  Agents start with a scrubbed environment plus only the credentials the
  provider registry injects for that runtime, with their working directory
  inside the allow-list, and are killed as a process group on cancellation.
- **Provider keys live in the OS keychain** (encrypted file fallback); the API
  and the database only ever hold a masked form.

## Testing

```
make test    # unit + API integration (fake agents, no network, no real keychain)
make race    # the same under the race detector
make e2e     # builds the binary, starts a real server on a scratch HOME,
             # pairs, indexes, runs a turn, checks it over HTTP, shuts down
```

## Status

The API and the machinery are complete and tested. What is **not** verified is
the exact command line of each real agent — see `../docs/runtimes/COMPAT.md`;
those flags live in `internal/runtime/specs.go` and are meant to be corrected
against installed versions without touching anything else.
