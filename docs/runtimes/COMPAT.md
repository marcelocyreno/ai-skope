# Runtime compatibility

The server drives coding agents through their non-interactive JSON modes. Those
flags change between releases, so this file records what has been verified and
against which version.

Every runtime below was **verified end to end** on 2026-09-04 with
`server/scripts/real-agent.sh`, which pairs a real server, attaches a picked
element and a local file, streams a real answer, and asks a follow-up in the
same agent session.

| runtime | version | invocation | resume | effort | status |
|---|---|---|---|---|---|
| `claude-code` | 2.1.261 | `claude -p --output-format stream-json --verbose --include-partial-messages --permission-mode plan [--model M] [--effort E]` | `--resume <session_id>` | `--effort low\|medium\|high\|xhigh\|max` | **verified** |
| `pi` | 0.84.3 | `pi -p --mode json --tools read,grep,find,ls [--model P/M] [--session-id ID] [--thinking E]` | `--session-id <id>` | `--thinking off\|minimal\|low\|medium\|high\|xhigh\|max` | **verified** |
| `omp` | 18.1.6 | `omp -p --mode json --no-tools [--model P/M] [--resume ID] [--thinking E]` | `--resume <id>` | `--thinking …\|auto` | **verified** |
| `opencode` | 1.18.20 | `opencode run --format json [--model P/M] [--session ID] [--variant E]` | `--session <id>` | `--variant minimal\|low\|medium\|high\|max` | **verified** |
| `codex` | 0.152.1 | `codex exec --json --sandbox read-only --skip-git-repo-check [--model M] [-c model_reasoning_effort=E] -` | `codex exec resume <id>` | `-c model_reasoning_effort=low\|medium\|high` | **verified** |
| `custom:<name>` | — | whatever the user configures; must read the prompt on stdin | — | — | supported |

## What each agent's output looks like

The parser (`internal/runtime/parse.go`) accepts the union of these shapes and
falls back to treating a line as plain text, so a field moving does not break a
turn. Each shape has a fake in `server/testdata/fakes/` and a case in
`TestAgentOutputShapes`.

- **Claude Code** — `{"type":"system","subtype":"init","session_id":…}`, token
  deltas as `{"type":"stream_event","event":{"type":"content_block_delta",…}}`,
  the assembled `{"type":"assistant","message":{"content":[…]}}`, and finally
  `{"type":"result","result":"…","usage":{"input_tokens":…}}`.
  **The answer arrives three times** — as deltas, as the whole message, and
  again in the result. The service keeps the most granular and drops the rest.
- **pi and omp** — `{"type":"session","id":…}` (a plain `id`), text inside
  `{"type":"message_update","assistantMessageEvent":{"type":"text_delta",…}}`,
  and usage on `turn_end.message.usage` as `{input, output}`.
  `thinking_*` events are the model reasoning to itself and are dropped.
- **Codex** — `{"type":"thread.started","thread_id":…}`, then items:
  `{"type":"item.completed","item":{"type":"agent_message","text":…}}`, with
  errors as `{"type":"item.completed","item":{"type":"error","message":…}}` or
  a bare `{"type":"error","message":"<json>"}`. Like opencode it returns the
  answer **in one piece**. Its model list is fixed by the account: on a ChatGPT
  plan only `gpt-5.5` was accepted, and `model_reasoning_effort=minimal` is
  rejected, so `low` is the cheapest setting.
- **opencode** — `{"type":"text","sessionID":…,"part":{"text":…}}` (note the
  `sessionID` casing), reasoning in its own parts, usage on
  `step_finish.part.tokens`. It returns the answer **in one piece**, not token
  by token; its streaming API is `opencode serve`, which the server does not
  use yet.

## Asking an agent which models it can reach

`pi`, `omp` and `opencode` carry their own credentials, so they know models the
server holds no key for. Detection asks them, and the switcher falls back to
that answer whenever no provider in the registry is scoped to the runtime. A
provider that *is* scoped to it stays an override and wins outright.

| runtime | command | shape | verified |
|---|---|---|---|
| `opencode` | `opencode models` | one bare `provider/model` per line | 1.18.20 — 14 models |
| `pi` | `pi --list-models` | aligned table, columns `provider model context …`; `--json` is accepted and ignored | 0.84.3 — 38 models |
| `omp` | `omp models --json` | `{"models":[{"provider","id","contextWindow"}]}` | 18.1.6 — 18 models |

Parsers live in `internal/runtime/models.go` with fixtures in
`models_test.go`. None of these is a stable contract, so every parser skips
what it does not recognise: a listing that changes shape costs rows, never the
whole switcher. `pi`'s table is the fragile one — a row is only accepted when
its third column parses as a context size, which is what keeps prose and error
text off the list.

The probe runs with **the same scrubbed environment a turn gets**, for the
`XDG_*` reason below: inheriting the server's environment points opencode at
the server's data directory, and it then reports only its unauthenticated
models (7 of 14, in the case that caught this).

## The contract every adapter satisfies

- The **prompt goes to stdin**, never argv, where `ps` would expose it. A test
  enforces this for every built-in spec.
- The process runs **read-only**: `--permission-mode plan` (Claude Code), a
  read-only tool allowlist (pi), no tools at all (omp, whose tool names depend
  on installed extensions), and no `--auto` (opencode).
- It starts with a **scrubbed environment** plus only the credentials the
  provider registry injects for that runtime. **`XDG_*` is deliberately not
  inherited**: those point at the *server's* config and data, and agents keep
  their own credentials under the same paths — passing ours down makes an
  authenticated agent look unauthenticated (opencode fails outright). Anyone
  who needs one can name it in `passthroughEnv`.
- Its working directory is inside an allowed folder, and cancelling a turn
  kills the whole process group.

## Verifying a runtime yourself

```
aiss runtimes detect                              # version and path per runtime
aiss doctor                                       # PATH, keychain, folders, index
./scripts/real-agent.sh claude-code sonnet        # a real turn, real tokens
./scripts/real-agent.sh pi glm-5.3-flash zai low
./scripts/real-agent.sh omp glm-5.3-flash zai low
./scripts/real-agent.sh opencode glm-5.3 zai-coding-plan
./scripts/real-agent.sh codex gpt-5.5 "" low
```

Or through the Taskfile at the repository root: `task real` runs all five.

`real-agent.sh` calls a real model, so it needs the agent installed and
authenticated, and it costs tokens. `scripts/e2e.sh` covers the same ground
with a fake agent and no network.

## A note on provider names

For `pi`, `omp` and `opencode` a model is addressed as `<provider>/<model>`
using **the agent's own provider id** — `zai` for pi and omp,
`zai-coding-plan` for opencode. The server therefore sends a provider's *kind*,
not the display name a user typed when adding the key.
