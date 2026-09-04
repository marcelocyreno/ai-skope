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
| `server/` | The **AI Skope Server** (`aiss`), written in Go: HTTP + SSE API, runtime adapters, provider keychain, folder allow-list and file index, chats and notes. Built and tested, including an end-to-end script. |
| `docs/SERVER-PLAN.md` | The plan the server was built from: architecture, security model, v1 API, milestones. |
| `docs/runtimes/COMPAT.md` | Which agent command lines are verified and which are still assumed. |

## Status

- **Wave 1 — visual design:** complete (`design/`).
- **Wave 2 — the server:** complete (`server/`). API, runtimes, providers,
  files, chats and notes, with unit, integration and end-to-end tests. The
  exact command line of each real coding agent still needs verifying against
  installed versions — see `docs/runtimes/COMPAT.md`.
- **Next — the extension:** not started. `docs/SPEC.md` carries the decisions.

```
cd design && node build.mjs        # rebuild the design kit and prototype
open design/preview/ai-skope.html

cd server && make test && make e2e # server tests, then a live end-to-end run
make build && ./aiss start && ./aiss pair
```
