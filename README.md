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
| `docs/SERVER-PLAN.md` | The plan for the Go server: architecture, security model, v1 HTTP/SSE API, runtime adapters, provider registry, file index, storage, packaging, milestones M0–M5. |

## Status

Wave 1 — **visual design** — is complete. No extension code and no server code
exist yet; both are planned in the documents above.

```
cd design && node build.mjs      # rebuild components/, screens/ and the prototype
open design/preview/ai-skope.html
```
