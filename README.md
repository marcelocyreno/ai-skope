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
| `extension/` | The **Chrome extension**: Vue 3 + TypeScript, MV3, Chrome Side Panel. The AI Pane, the element picker and selection toolbar, the local-file picker, history and the options page. |
| `server/` | The **AI Skope Server** (`aiss`), written in Go: HTTP + SSE API, runtime adapters, provider keychain, folder allow-list and file index, chats and notes. |
| `docs/SERVER-PLAN.md` | The plan the server was built from: architecture, security model, v1 API, milestones. |
| `docs/runtimes/COMPAT.md` | Which agent command lines are verified and which are still assumed. |
| `docs/PUBLISHING.md` | What it takes to put the extension on the Chrome Web Store, and what has to exist first. |
| `store/` | The Chrome Web Store submission: `SUBMIT.md` is the checklist, `LISTING.md` the text to paste, `screenshots/` the assets. |

**Documentation:** [install guide](https://marcelocyreno.github.io/ai-skope/install)
· [privacy policy](https://marcelocyreno.github.io/ai-skope/privacy)

## Try it

Install the server, then take a pairing code:

```
brew install marcelocyreno/tap/aiss
aiss start                       # listens on 127.0.0.1:7331
aiss folders add ~/dev --watch   # optional — let it read a folder
aiss pair                        # note the 8-character code
```

The extension is loaded unpacked, which means building it:

```
cd extension && npm install && npm run build
```

Then in Chrome: `chrome://extensions` → Developer mode → **Load unpacked** →
choose `extension/dist`. Click the AI Skope icon (or ⌘⇧A), enter the code, and
ask about the page you are on.

Building the server from source instead of installing it is `cd server && make
build`, which needs Go. The
[install guide](https://marcelocyreno.github.io/ai-skope/install) covers the
rest: what has to be installed first, the download without Homebrew, and what
to do when something does not work.

## Status

All three waves are done: the visual design (`design/`), the server
(`server/`) and the extension (`extension/`). All five supported agents have
been driven end to end for real — Claude Code, Codex, pi, omp and opencode —
attaching a picked element and a local file and streaming the answer back.
`docs/runtimes/COMPAT.md` records the exact invocation and output shape each
one was verified against, and the version it was verified on.

Since 0.1.0 the work has been on the edges. The default runtime, model and
effort are chosen in Options → Server & runtimes and survive the pane closing.
The right-click menu is opt-in and off by default. Notes is hidden until it is
page-linked, editable and properly undoable
([#11](https://github.com/marcelocyreno/ai-skope/issues/11)) — the surface is
gone, the plumbing is parked where it was.

The server is released for macOS and Linux, on both architectures. The
extension is not publicly listed on the Chrome Web Store yet, so it is loaded
unpacked; going public is the last listing decision left.

Everything is driven from the `Taskfile.yml` at the root:

```
task              # list every target
task up           # build both halves, start the server, print a pairing code
task test         # server + extension, unit through end-to-end (no tokens)
task real         # drive every installed agent through a real turn (costs tokens)
task doctor       # check the installation and explain what to fix
task folder -- ~/dev
```

## Licence

MIT — see [LICENSE](LICENSE).
