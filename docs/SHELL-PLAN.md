# Thin extension shell, server-served UI

Status: proposed, not started. Written October 2026 against `main` at `bec6702`.

## Context

Every change to the side panel today needs a Chrome Web Store review (`store/SUBMIT.md`). 0.1.2 and 0.1.3 are still not uploaded. Yet ~5,000 of the extension's ~6,000 TS/Vue lines are plain UI and HTTP client code that never touches `chrome.*`. Only about 1,070 lines must live in an extension:
- `src/content/` (563)
- `src/worker/` (101)
- `src/stores/page.ts` (324)
- `src/stores/storage.ts` (76)
- some glue in `src/pane/App.vue`

**Goal:** the extension becomes a thin **shell**. It connects to `aiss`, pairs, explains what to do when the server is offline or the versions don't match, and holds the Chrome-only abilities. The real chat UI and the settings UI come from the server, which embeds them and serves them on 127.0.0.1. After this:
- The server can ship UI changes as fast as it likes.
- The extension needs a store release only when the shell or the bridge changes.
- The server declares the minimum extension ("client") version it needs.

**Chrome policy check.** MV3 forbids remotely hosted code in extension contexts. Chrome's docs exempt code running in contexts isolated from extension APIs, such as iframes ([Improving security in MV3](https://developer.chrome.com/docs/extensions/develop/migrate/improve-security)). A server page in an `<iframe>` with no `chrome.*` access, talking to the shell only over `postMessage`, fits that exemption. The "No remote code" answer in `store/LISTING.md:134-140` stays honest, as long as its explanation is updated.

**Decisions already taken:**
- The options page moves to the server too.
- The UI source goes in a new top-level `ui/`.
- Server-only for good: SPEC §8 Q4 "keys-only mode" is closed.
- One cut-over: the server ships first, then extension 0.2.0.

## Target architecture

```
extension/  (store-reviewed, slow)          ui/  (ships inside aiss, fast)        server/
 manifest, worker, content script            Vue app: chat pane + settings         serves /ui/* from embed
 src/shell/  state machine + screens  ⇄MessagePort⇄  src/bridge/client.ts          /v1/health: client.min, ui{…}, auth
 src/shell/page.ts, storage.ts (chrome)      api client, stores, components        Host check, CSP frame-ancestors
 src/contract/  bridge + health types  ──imported as @contract──▶
```

**Shell states** (`extension/src/shell/state.ts`), for both `sidepanel.html` and `options.html`:

| State | Trigger | Screen |
|---|---|---|
| `connecting` | start | — |
| `offline` | health fetch fails | Today's two screens from `Pairing.vue` (never paired / server not running), with backoff 1s–30s |
| `server-outdated` | health has no `ui` (a 0.1.x server), or `ui.protocol` < what the shell speaks | "AI Skope Server {version} is older than this extension needs", with `brew upgrade --cask aiss && aiss stop && aiss start` and a copy button |
| `client-outdated` | manifest version < `health.client.min` | "Update now" calls `chrome.runtime.requestUpdateCheck()`, waits for `onUpdateAvailable`, then `chrome.runtime.reload()`, and warns that the panel will close. `throttled` and `no_update` (store not published yet, or an unpacked copy) each get their own message. |
| `unpaired` | no token, `auth: "invalid"`, or `serverId` ≠ the stored one | Pairing form and base-URL editor (moved from `Pairing.vue`) |
| `ui-missing` | `ui.available === false`, or no handshake within 10s | "The server's interface didn't load", with Retry |
| `ready` | — | Full-bleed `<iframe id="skope-ui" src="{base}/ui/pane.html?b={ui.build}" allow="clipboard-write">` (options page: `options.html#section`) |

**Version handshake.** `GET /v1/health` (unauthenticated) is extended. It may also take an optional bearer token, so the token can be proved without freezing `/v1/runtimes` into the contract:

```json
{ "status":"ok", "version":"0.2.0", "apiVersion":1, "serverId":"…", "uptimeMs":0, "paired":true,
  "auth":"valid|invalid|absent",
  "client":{"min":"0.2.0"},
  "ui":{"url":"/ui/","build":"<vite build id>","protocol":1,"available":true,"dev":false} }
```

- `client.min` comes from `version.MinClient` in `server/internal/version/version.go`.
  - Tests can override it with `AISS_MIN_CLIENT`.
  - Versions are compared as Chrome's dot-separated integers, in Go and in `@contract`.
- `ui.build` comes from the Vite build id, not `version.Version`, because every dev build reports `0.1.0-dev`.
- Soft gating: the hello message carries the shell's `capabilities[]`, and the UI hides any feature the shell lacks.
- Hard gating: `client.min` and `ui.protocol`.
- The shell sends `X-Skope-Client: <manifest version>` on health and pair; the UI sends it on every request.
  - The server records the latest value per pairing.
  - `aiss pair --list` and `aiss doctor` show it, and flag 0.1.x pairings that never send it.
- **Frozen shell contract:** `/v1/health`, `/v1/pair`, and bridge protocol v1. Everything else under `/v1` ships together with the UI that calls it.

**Bridge v1.** Types and constants live in `extension/src/contract/bridge.ts`. They have no `chrome` references and no dependencies.

Handshake:
1. The UI starts. If it is top-level, it renders "Open AI Skope from Chrome". Otherwise it posts `{kind:"skope:ready", protocol, build}` to `location.ancestorOrigins[0]`, which must match `^chrome-extension://[a-p]{32}$`.
2. The shell accepts that message only if `e.source === iframe.contentWindow` and `e.origin === uiOrigin`. It then re-checks health (serverId, `auth`, `client.min`).
3. The shell posts `hello` with a fixed target origin and transfers a `MessageChannel` port. Hello carries `{protocol, client:{version, capabilities}, token, settings, page, surface:"pane"|"options", section}`.
4. The UI checks hello the same way (source is `window.parent`, origin is `ancestorOrigins[0]`) and replies `ack`.
5. The shell flushes queued commands. Every new `skope:ready` (for example after the UI reloads itself) starts a fresh session on a new port.

Requests from the UI (`{id, method, params}` → `{id, ok, result | error:{code}}`):
- `page.pick`, `page.cancelPick`, `page.selection`, `page.text`
- `settings.patch`: refuses `token` and `serverId`, and accepts only a loopback `baseUrl`
- `open.options(section)`, `open.shortcuts`
- `auth.lost`: the shell re-checks before clearing
- `pairing.forget`
- `clipboard.write`: fallback, used if the spike shows the iframe clipboard is flaky

Events from the shell: `page.changed`, `selection.action`, `command` (pick-element, add-selection, new-chat, copy-last-answer), `settings.changed`.

Error codes: `needs-access` (the shell draws an "Allow on ‹host›" bar, so the permission request runs from a click in the shell), `restricted`, `blocked`, `page-access-never`, `no-tab`.

**Security model.** The shell is the privilege boundary, and the UI is treated as less trusted: it renders page content, and anything listening on port 7331 could serve it.
- Bridge methods never take a `tabId`; the shell always uses its own window's tab.
- The shell itself enforces restricted URLs, `pageAccess`, and `blockedHosts`.
- The token is handed over only when `health.serverId` matches the stored one and the UI's origin equals the `baseUrl` origin. The only exception is a dev-mode UI URL on loopback.
- The token stays in the iframe's memory only. No cookies, no localStorage, no service worker.

On the server:
- **Host check:** every route accepts only Host `127.0.0.1|localhost|[::1]|cfg.Host` on the bound port, and answers 421 otherwise. This closes DNS-rebinding reads of `/v1/health` that work today.
- **`/ui/*` headers:**
  - CSP `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors <valid active paired origins>`.
  - `Referrer-Policy: no-referrer` and `nosniff`.
- **Own origin:** `originAllowed` accepts the server's own origin, computed from config and the listener, never from the Host header.
- **Pairing origin:** `/v1/pair` validates the origin. Today it trusts the request body (`handlers_core.go:42`), and a bad value would otherwise flow into `frame-ancestors`.
- **Extension CSP:** the manifest gains `content_security_policy.extension_pages` with `connect-src`/`frame-src http://127.0.0.1:* http://localhost:*`.

**Settings ownership.**
- **Shell-owned, typed, in `chrome.storage.local` `settings`** (needed offline, or enforced with Chrome APIs): `baseUrl`, `token`, `serverId`, `theme`, `palette`, `textSize`, `pageAccess`, `blockedHosts`, `contextMenu`. The key stays the same, so existing users keep their pairing.
- **UI-only preferences** (`conciseness`, and future ones) move to the server's free-form `/v1/settings` KV, for example `ui.conciseness`. Adding a new preference then never needs a store release.

## Steps (PR-sized, in order)

### 0. Prerequisites and spike (no merge)
- Merge open PR #29 first. It touches nearly every component that moves, and the moves below are `git mv` so history is kept.
- Close PR #28 as superseded; reuse its "assert exactly `['/v1/health']`" test pattern. This plan resolves issue #27.
- Spike on a throwaway branch, in Chrome and Brave, with an extension page that iframes a 127.0.0.1 page:
  1. Does `chrome.permissions.request` called from a bridge message (click inside the iframe) count as a user gesture? Test both `window.postMessage` and `MessagePort`. The `needs-access` bar is built either way.
  2. Any Local Network Access or Brave localhost prompt.
  3. Can the shell focus the composer inside the iframe?
  4. Clipboard inside the iframe, with `allow="clipboard-write"`.
  5. Do `target=_blank` links open tabs?
  6. Do `requestUpdateCheck` outcomes behave as described above?

### 1. `fix(server)`: hardening and version fields (works with 0.1.3, releasable alone)
- `server/internal/api/`
  - `server.go`: Host-check middleware right after `recover`. Allowed hosts come from the bound listener, so `httptest` passes its own.
  - `auth.go`: own-origin support; add `X-Skope-Client` to the CORS allowed headers (needed in dev mode); record the client version.
  - `handlers_core.go`:
    - health gains `auth` and `client.min`;
    - `pair` validates the origin (`^chrome-extension://[a-p]{32}$`, plus loopback http in dev mode only);
    - new `DELETE /v1/pair` revokes the caller's own pairing, so unpair is real (today `About.vue` only forgets the token locally).
- `server/internal/version/version.go`: `MinClient`, `Compare`. New `version_test.go`.
- `server/internal/config/config.go`: `AISS_MIN_CLIENT`.
- `server/internal/store/`: new `migrations/0002_pairing_client.sql` (`client_version`, `client_seen_at`) and `pairings.go`.
- `server/internal/cli/commands.go`: a CLIENT column in `pair --list`; `doctor` warning.
- Fix the fake origins that the new validation rejects: `scripts/e2e.sh` (`e2eextensionid…`) and `api_test.go` `testOrigin`.
- Tests in `api_test.go` and `store_test.go`:
  - Host `evil.test` gets 421; loopback variants pass.
  - Bad pairing origins are refused.
  - A golden key list for health.
  - `auth` is valid, invalid, or absent as expected.
  - The client version is recorded only when it changes.
  - A table of `Compare` cases.

### 2. `refactor(extension)`: bridge seam, in-process (no iframe yet; behaviour unchanged)
- New `extension/src/contract/{bridge,health,settings,version}.ts`. `ContextItem` and `PageSnapshot` move here from `api/types.ts`, which re-exports them.
- `git mv src/stores/page.ts → src/shell/page.ts` and `src/stores/storage.ts → src/shell/storage.ts`.
- New `src/shell/bridge-host.ts`:
  - request handlers, plus enforcement of restricted / `blockedHosts` / `pageAccess`;
  - a command queue until `ack`, dropped after 60s;
  - `storage.onChanged` on the session area. This fixes the bug where shortcuts do nothing while the pane is open: `drainPending` runs only on mount, `App.vue:153`.
  - Only the side-panel shell answers `received:true`, and it filters by the sender's `windowId`.
- New `src/bridge/client.ts`.
- New `src/stores/page.ts` and `src/stores/storage.ts` keep the **same exports**, implemented over the bridge. `chat.ts`, the components and the existing `vi.mock` paths stay untouched.
- The `chrome.*` glue in `src/pane/App.vue` and `options/sections/{Shortcuts,About}.vue` moves behind the bridge.
- `src/worker/service-worker.ts`: try `runtime.sendMessage({kind:"skope:command"})` before falling back to `storage.session`, as `deliverSelection` already does.
- The transport is a real `MessageChannel` inside the same document, so step 4 only has to transfer the port.
- Tests:
  - new `tests/bridge.test.ts`: request/response, events, queueing, blocked or never refused, no `tabId` accepted;
  - `tests/worker.test.ts`: the command path;
  - a vitest guard that fails on `chrome.` outside `shell/`, `content/`, `worker/`.
- The full existing e2e suite must pass unchanged. That is the proof this step changes no behaviour.

### 3. `feat(server)`: the `webui` package (serves whatever is built; nothing built yet)
- New `server/internal/webui/{webui.go,webui_test.go,assets/README.md}`.
  - `//go:embed all:assets` embeds the parent folder, so a committed README keeps it compiling. The build output goes to the gitignored `assets/dist/`, which Vite empties on each build.
  - `New(fs.FS, opts)` takes a filesystem, so tests use `fstest.MapFS`.
  - HTML is served `no-cache`; `assets/*` gets `max-age=31536000, immutable`; explicit content types, including `.woff2`; the CSP and headers above. `frame-ancestors` is recomputed from the valid active pairings, plus loopback in dev mode.
  - Without a built UI it serves a placeholder page and reports `ui.available=false`.
- `server/internal/api/server.go`: mount `/ui/`; `GET /` redirects to `/ui/pane.html`.
- `server/internal/api/handlers_core.go`: health gains `ui{…}`.
- `server/internal/config/config.go`: `AISS_UI_DIR` serves the UI from disk (e2e and development without rebuilding Go). `AISS_UI_URL` advertises a Vite dev server, in dev mode only.
- `.gitignore`: `server/internal/webui/assets/dist/`.
- Tests:
  - cache and content-type headers; path traversal;
  - the CSP lists exactly the valid pairings and follows pair and revoke;
  - placeholder plus `available=false`;
  - same-origin POST accepted, other http origins refused.

### 4. `feat`: the split (one PR, four reviewable commits)
1. **Pure `git mv` from `extension/` to `ui/`:**
   - `src/api/*`; `src/stores/{chat,models,history,files,notes,toast,announce,connection}.ts`; the bridge-backed `stores/{page,storage}.ts`; `src/bridge/client.ts`;
   - `src/pane/**` except `components/Pairing.vue`; `src/options/**`;
   - their unit tests (`chat, history, models, send, markdown, transcript, sse, caret`).
2. **`ui/` scaffold:**
   - `ui/{package.json, package-lock.json, tsconfig.json, vite.config.ts, vitest.config.ts, pane.html, options.html}`.
   - `tsconfig.json` leaves `chrome` out of `types`, so any `chrome.*` call fails to compile.
   - `vite.config.ts`: `base:"/ui/"`; `outDir:"../server/internal/webui/assets/dist"`; emits `build.json` with the build id and protocol; `@contract` alias.
   - No dynamic `import()`: a lazy chunk requested while the server is down would break the UI.
   - UI `connection.ts`: drops pairing and `setBaseUrl`, and gains a `reconnecting` state. Today `App.vue:340` swaps everything for `Pairing` when offline, so the offline `StatusStrip` never renders; with `reconnecting`, the transcript and draft survive a restart. When health shows a different `ui.build`, the UI calls `location.reload()`.
   - `client.ts`: relative URLs, `X-Skope-Client` on every request, and a 401 sends `auth.lost`.
   - `conciseness` moves to `/v1/settings`.
3. **The shell:**
   - New `extension/src/shell/{main.ts, App.vue, state.ts, server.ts, Frame.vue, AccessBar.vue, Outdated.vue}`; `git mv src/pane/components/Pairing.vue → src/shell/Pairing.vue`.
   - `sidepanel.html` and `options.html` load the shell with their surface. `vite.config.ts` entries are updated.
   - Styles: both the shell and `ui/` import `design/tokens/*.css` and the icon sprite through an `@design` alias, instead of keeping copies. That retires the "manual byte-identical copy" rule. Fonts from #29 move to `design/fonts/`.
   - `manifest.json`: the CSP; version 0.2.0; drop `web_accessible_resources: assets/*` if unused. Also bump `package.json` and the root entry of `package-lock.json`.
   - Build and release wiring:
     - `Taskfile.yml`: `build:ui`, `test:ui`; `build:server` and `install` depend on `build:ui`; `build:server` watches `internal/webui/assets/dist/**`; `release:check` asserts `MinClient` ≤ the manifest version.
     - `.goreleaser.yaml`: the before-hook runs `npm ci` and `npm run build` in `ui/`, then asserts `assets/dist/pane.html` exists.
     - `.github/workflows/release.yml`: add `setup-node`.
     - `.github/workflows/test.yml`: a new `ui` job (npm ci, vue-tsc, vitest, build).
4. **e2e** (`extension/tests/e2e/`):
   - `harness.ts` builds `ui/` before `go build` and fails fast if `GET /ui/pane.html` isn't 200.
   - It exposes `h.ui = panel.frameLocator('#skope-ui')`, plus `uiFrame()` for `evaluate` calls. Pairing stays on `panel`, so `tools/launch-brave.mjs` is unaffected.
   - A `serverEnv` fixture option (for example `AISS_MIN_CLIENT=99.0.0`), and a stub server with 0.1.x-shaped health.
   - Port every spec, plus `tests/store/screenshots.spec.ts`.
   - New `shell-states.spec.ts`: offline turns ready once the server starts; client outdated; server outdated; serverId mismatch leads to pairing; UI missing.
   - New `bridge.spec.ts`: pick through the frame; toolbar selection reaches the tray; a command while the pane is open.
   - A fixture page that iframes `/ui/pane.html` gets blocked by `frame-ancestors`.
- New unit tests:
  - `extension/tests/shell-state.test.ts`: a table of cases asserting the exact request list for each state;
  - `ui/tests/bridge-client.test.ts`: hello is rejected from the wrong source, from the wrong origin, or when the UI is top-level.

### 5. `docs`: the new shape (must land before submitting 0.2.0)
- `CLAUDE.md`:
  - three code homes (extension shell / ui / server) and that the `ui/` directory signals its release channel;
  - new invariants: `chrome.*` only in `extension/`; the shell enforces page access; the token handoff rules; the frozen shell contract; the `MinClient` rule;
  - the Markdown renderer path becomes `ui/src/pane/markdown.ts`;
  - the design-copy rule is retired;
  - the commit scope `ui` and label `area: ui`;
  - the Releasing section: versions are decoupled.
- `.claude/skills/issue-triage/SKILL.md`: add `area: ui`.
- `store/LISTING.md`:
  - keep "No remote code", adding the iframe explanation;
  - `sidePanel` justification: "hosts the interface served by AI Skope Server on this computer";
  - reviewer test instructions.
- `docs/privacy.md`: the interface is served by aiss on 127.0.0.1, still the only destination. Bump the date.
- `store/SUBMIT.md`:
  - release order;
  - "`client.min` only ever names a version already live in the store";
  - how long to wait between the server release and submitting the extension.
- `docs/PUBLISHING.md`: rewrite the versioning paragraph.
- `docs/SPEC.md`: update the §7 storage line; close Q4.
- `docs/SERVER-PLAN.md`: origin checks, `/ui`, and health.
- `README.md`, `docs/install.md`, `extension/README.md`: restart the server after `brew upgrade`; the new dev loop.

### 6. `chore`: dev loop
- `task dev:ui` runs Vite for `ui/`, with a `server.proxy` from `/v1` to aiss, and starts aiss with `AISS_DEV=1 AISS_UI_URL=http://localhost:5173/ui/`.
  - The real side panel then gets HMR without reloading the extension.
  - The UI uses relative URLs, so dev and production run the same code.
- Optional `ui/dev/mock-shell.html`: a fake bridge, for working on the UI in a plain tab.

## Release sequence
1. Merge steps 1–5, then tag server **v0.2.0**. It serves the UI and has `MinClient = "0.2.0"`.
   - 0.1.3 extensions ignore the new health fields and keep calling the unchanged `/v1`, so nothing breaks for them.
2. Wait for users to run `brew upgrade` (the maintainer decides how long), then submit extension **0.2.0**.
   - Chrome auto-updates the extension, while `aiss` only updates by hand; anyone on an old aiss sees the "server outdated" screen.
3. Once 0.2.0 is live in the store and `aiss doctor` no longer shows legacy pairings, `/v1` is free to evolve with the UI.

From then on:
- UI change → server release only.
- Bridge or shell change → extension release, and bump `MinClient` only after that version is live.

## Verification
- **Server:** `cd server && go vet ./... && go test -race ./...`, then `./scripts/e2e.sh`. Check:
  - Host returns 421;
  - health fields, `auth`, `client.min`, `ui`;
  - `/ui/pane.html` headers;
  - the CSP follows `aiss pair` / `aiss pair --revoke`.
- **Manual curl:**
  - `curl -H 'Host: evil.test' 127.0.0.1:7331/v1/health` → 421;
  - `curl -I 127.0.0.1:7331/ui/pane.html` → CSP with `frame-ancestors chrome-extension://…`.
- **UI and extension:** `npm run typecheck && npx vitest run && npm run build` in both `ui/` and `extension/`. The `chrome.` guard test and the `ui/` tsconfig both prove the UI has no Chrome access.
- **Browser:** `task build && npx playwright test` in `extension/`: all ported specs plus `shell-states`, `bridge` and `frame-ancestors`.
- **Real browser:** `task brave:test`. Then:
  - pair, ask, pick an element on a new site (the permission prompt appears from the click), select text through the toolbar, Ctrl+Shift+K with the pane open;
  - stop aiss → offline screen; start aiss → ready;
  - `AISS_MIN_CLIENT=9.9.9 aiss restart` → "update the extension".
- **Dev loop:** `task dev:ui`, then edit a component in `ui/` → HMR in the open side panel with no extension reload.
- **Release dry run:** `task release:check`. The goreleaser snapshot contains the UI, and `aiss` from the archive serves `/ui/pane.html`.

## Open questions for the maintainer
- How long to wait between the server v0.2.0 tag and submitting extension 0.2.0. Homebrew users must upgrade and restart aiss by hand.
- Whether the first `MinClient` should be `0.2.0`. Recommended: it equals the first shell release, so 0.1.x shells never see a gate.

## Found along the way
- **Blocked sites are never enforced** (`isBlocked`, `page.ts:51`, has no callers) even though Privacy says "never reads or sends anything here". Worth fixing on its own, ahead of this work; step 2's shell enforcement carries it forward.
- The "Open automatically" toggle (`General.vue:49-55`) has no consumer. Left as is.
