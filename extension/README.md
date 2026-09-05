# AI Skope — Chrome extension

The browser half of [AI Skope](../README.md). It puts the AI Pane in Chrome's
side panel, lets you aim at the page (pick an element, select text) or attach a
local file, and streams the answer from [`aiss`](../server/README.md) — the
local server that drives the coding agents installed on your machine.

## Try it

```
cd server && make build && ./aiss start   # start the server
./aiss folders add ~/dev --watch          # let it read a folder
./aiss pair                               # note the 8-character code

cd ../extension && npm install && npm run build
```

Then, in Chrome:

1. open `chrome://extensions`, turn on **Developer mode**,
2. **Load unpacked** → choose `extension/dist`,
3. click the AI Skope icon (or ⌘⇧A) to open the side panel,
4. type the pairing code.

The panel opens beside the page — drag Chrome's own edge to resize it.

### Or let it start Brave for you

```
task brave:test
```

That builds the extension, starts the server if it is not running, launches
Brave on a **separate profile** with the extension loaded, and pairs it — so it
is ready to use without touching your everyday browser or its tabs. The profile
lives in `~/.local/share/ai-skope/brave-test-profile` and persists, so the
pairing and your chats survive between runs.

It uses a separate profile for a reason: `--load-extension` only takes effect
at startup, so loading into your everyday Brave would mean quitting it, and
Brave's default is to open a fresh New Tab page rather than restore what you
had open.

## How it is put together

| Piece | Where | What it does |
|---|---|---|
| Side panel | `src/pane/` | the AI Pane: top bar, tabs, transcript, composer, switcher, history, quick settings |
| Options page | `src/options/` | server, runtimes, folders, providers, privacy, shortcuts |
| Content script | `src/content/` | element picker, selection toolbar, page text — injected on demand, drawn in a Shadow DOM |
| Service worker | `src/worker/` | opens the panel, keyboard commands, context menu |
| API client | `src/api/` | typed wrapper over the server, plus the SSE parser |
| Stores | `src/stores/` | reactive state: connection, models, chat, history, notes, files, page |
| Styles | `src/styles/` | copied unchanged from `design/tokens` — tokens, six palettes, every `sk-*` component |

**The connection lives in the side panel, not the service worker.** MV3 stops an
idle worker, which would cut a streaming answer off mid-sentence; the panel
document lives exactly as long as the panel is open.

**Streaming uses `fetch` + `ReadableStream`, not `EventSource`**, because
`EventSource` cannot send an `Authorization` header and cannot POST — and a
turn is a POST that streams.

## What it asks for

At install time: `storage`, `sidePanel`, `scripting`, `tabs`, `contextMenus`,
and access to `127.0.0.1` (the server). **No access to any website.**

The first time you pick an element or select text on a site, Chrome asks
whether AI Skope may read that site. That prompt *is* the design's "Page
access: Ask" setting. Only what you aim at is sent, unless you set page access
to *Always*.

## The icon

The action icon is the reticle the product is built on — the same mark the pane
shows and the element picker uses as its cursor. `tools/make-icons.py` draws it
at every size Chrome asks for, with proportions tuned per size rather than one
geometry scaled down: at 16px the centre dot is dropped and the stroke thickened,
because a ring, four ticks and a dot cannot all survive sixteen pixels.

```
python3 tools/make-icons.py    # rewrites icons/icon{16,32,48,128}.png
```

## Development

```
npm run dev        # rebuild on change (reload the extension in Chrome to pick it up)
npm run typecheck  # vue-tsc
npm test           # unit tests: SSE parser, selector generation
npm run test:e2e   # real Chrome + real server + a fake agent
```

The end-to-end suite builds `aiss`, starts it on a scratch profile, loads the
built extension into a real Chrome, pairs it, and drives the panel: a streamed
answer, history, a local file, and picking an element on a fixture page.

Two things it deliberately does not cover, because Chrome will not let a test
drive them — both are on the manual checklist:

- the **optional-permission prompt** (the e2e copy declares host access up front),
- the **real side panel** opening from the toolbar, since Playwright can only
  open the panel document as an ordinary tab.

## Status

Built against the server's v1 API. The agent command lines the server uses are
still unverified against real installs — see `../docs/runtimes/COMPAT.md`.
