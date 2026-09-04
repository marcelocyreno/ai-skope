# Runtime compatibility

The server drives coding agents through their non-interactive JSON modes. Those
flags change between releases, so this file records what has actually been
verified and what is still assumed. Anything marked **assumed** is written from
the documented shape and is covered by a fake agent in the tests, not by the
real binary.

The parser (`internal/runtime/parse.go`) is deliberately tolerant: it accepts
the union of the shapes below and falls back to treating a line as plain text,
so a flag or field drifting does not break a turn outright.

| runtime | invocation | resume | effort | status |
|---|---|---|---|---|
| `claude-code` | `claude -p --output-format stream-json --verbose --permission-mode plan [--model M] [--effort E]` | `--resume <session_id>` | `--effort low\|medium\|high\|max` | **assumed** |
| `codex` | `codex exec --json --sandbox read-only --skip-git-repo-check [--model M] [-c model_reasoning_effort=E] -` | `codex exec resume <id> --json …` | `-c model_reasoning_effort=` | **assumed** |
| `opencode` | `opencode run --print-logs [--model provider/model] [--session id]` | `--session <id>` | provider-dependent | **assumed** |
| `pi` | `pi --json [--model provider/model] [--session id]` | `--session <id>` | provider-dependent | **assumed — binary and JSON mode unconfirmed** |
| `omp` | `omp --json [--model provider/model] [--session id]` | `--session <id>` | provider-dependent | **unknown — confirm the binary name and whether it has a JSON mode** |
| `custom:<name>` | whatever the user configures; must read the prompt on stdin | — | — | supported |

## Contract every adapter must satisfy

- The **prompt is written to stdin**, never to argv: arguments are visible to
  every process on the machine through `ps`. A test enforces this for all
  built-in specs.
- Output is read line by line. JSON objects are parsed; anything else is
  treated as answer text.
- The process runs with a **scrubbed environment** (see `BaseEnv`) plus the
  credentials the provider registry injects for that runtime, and with its
  working directory inside an allowed folder.
- Cancellation kills the whole process group, sweeps again shortly after to
  catch a child forked during the race, and gives up on stray descendants via
  `WaitDelay`.

## Output shapes understood

- Anthropic/Claude Code: `{"type":"system","session_id":…}`,
  `{"type":"assistant","message":{"content":[{"type":"text"|"tool_use",…}]}}`,
  `{"type":"content_block_delta","delta":{"text":…}}`, `{"type":"result",…}`.
- Codex: `{"type":"thread.started","thread_id":…}`,
  `{"type":"item.started"|"item.completed","item":{…}}`, and the older
  `{"msg":{"type":"agent_message","message":…}}`.
- Generic: any object carrying `text`, `delta`, `content`, `usage`, or an
  `error`; plain text lines.

## How to verify a runtime

```
aiss runtimes detect          # version and path per runtime
aiss doctor                   # PATH, keychain, folders, index health
```

Then run one real turn and compare the transcript against the agent's own
output. When a flag turns out to be wrong, change it in
`internal/runtime/specs.go` — the process machinery does not need to change —
and move the row above to **verified**, noting the version.
