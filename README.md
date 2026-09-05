# AI Skope

A Chrome extension that splits the browser canvas in two: the real page on the
left, and a page-aware AI chat on the right. You *aim* at what you want to ask
about — pick an element, drag-select text, or attach a local file — and it lands
in the conversation as context.

Models do not come from the browser. AI Skope talks to a local **AI Skope Server
(AISS)** that drives coding agents already installed on the machine (Claude
Code, Codex, pi, omp, opencode) and can read folders you allow — which is what
makes asking about **local HTML, Markdown, repos and notes** possible. Direct
provider API keys are supported as a secondary source.

## Repository

| Path | What it holds |
|---|---|
| `design/` | The visual design system: tokens, six palettes, 20 component previews, 12 screens, and an interactive prototype (`design/preview/ai-skope.html`). Start there. |
| `docs/SPEC.md` | The product as specified so far: anatomy, context types, chats & history rules, model-source hierarchy, settings tiers, visual system, implementation assumptions, open questions. |
| `extension/` | The **Chrome extension**: Vue 3 + TypeScript, MV3, Chrome Side Panel. The AI Pane, the element picker and selection toolbar, the local-file picker, notes, history and the options page. |
| `server/` | The **AI Skope Server** (`aiss`), written in Go: HTTP + SSE API, runtime adapters, provider keychain, folder allow-list and file index, chats and notes. |
| `docs/SERVER-PLAN.md` | The plan the server was built from: architecture, security model, v1 API, milestones. |
| `docs/runtimes/COMPAT.md` | Which agent command lines are verified and which are still assumed. |
| `docs/PUBLISHING.md` | What it takes to put the extension on the Chrome Web Store, and what has to exist first. |

## Try it

```
cd server && make build && ./aiss start   # start the server
./aiss folders add ~/dev --watch          # let it read a folder
./aiss pair                               # note the 8-character code

cd ../extension && npm install && npm run build
```

Then in Chrome: `chrome://extensions` → Developer mode → **Load unpacked** →
choose `extension/dist`. Click the AI Skope icon (or ⌘⇧A), enter the code, and
ask about the page you are on.

## Status

- **Wave 1 — visual design:** complete (`design/`).
- **Wave 2 — the server:** complete (`server/`).
- **Wave 3 — the extension:** complete (`extension/`).

All four coding agents installed here have been driven end to end for real —
Claude Code with Sonnet, pi and omp with z.ai GLM 5.3 Flash, opencode with
GLM 5.3 — attaching a picked element and a local file and streaming the answer
back. `docs/runtimes/COMPAT.md` records the exact invocation and output shape
of each. Codex is still unverified: it is not installed on this machine.

Everything is driven from the `Taskfile.yml` at the root:

```
task              # list every target
task up           # build both halves, start the server, print a pairing code
task test         # server + extension, unit through end-to-end (no tokens)
task real         # drive every installed agent through a real turn (costs tokens)
task doctor       # check the installation and explain what to fix
task folder -- ~/dev
```
