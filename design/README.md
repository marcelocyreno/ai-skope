# AI Skope — design kit (wave 1: visuals)

AI Skope is a Chrome extension that splits the browser canvas into the real page
(**Content Pane**, left) and a full-height **AI Pane** (right): a page-aware chat
where you *aim* at the page — pick an element or select text — and it lands in
the conversation as context. Notes tab, model chip with live status, quick model
switcher, settings. Light and dark.

This folder is the visual design: tokens, components, screens and an interactive
prototype. No extension code yet.

## Direction — "optical instrument"

- **Neutrals** are cool graphite with a blue-teal bias (never pure grey); dark
  surfaces step *lighter* as they come forward.
- **One accent, amber**, is reserved for *aim*: picked-element outline, selected
  text, context chips, the send button. It is never used for decoration.
- **Teal** is only for keyboard focus and links. **Green / amber / red** are
  status (model online / degraded / offline) and never double as the accent.
- **Type**: Manrope (UI/display) · IBM Plex Sans (body, chat) · IBM Plex Mono
  (selectors, latency, shortcuts). Scale 11 / 12 / 13 / 14 / 16 / 18 / 22.
- **Signature**: the app mark and the picker cursor are the same reticle. In
  pick mode the hovered element gets a 2 px amber outline with corner ticks and a
  mono tag reading `article.pg-tier.featured · 320 × 412` — the instrument
  reading of the page.

## Model source (the key architectural idea)

The extension does not call AI providers itself by default. It talks to the
**AI Skope Server (AISS)**, a local server that drives coding agents installed on
the machine — Claude Code, Codex, pi, omp, opencode — and can read local folders,
so you can ask about local HTML/Markdown files and repos. The model picker is
therefore three levels deep:

    Source (AI Skope Server | API keys)
      └ Runtime (Claude Code · Codex · pi/omp/opencode, with a runtime chooser)
          └ provider / model  ·  Effort (Low / Medium / High / Max)

The model chip lives in the **composer** (model is a per-message decision) and
shows runtime glyph + model (+ effort tag when the pane is ≥ 440 px); the
switcher rises from it. Latency lives in the switcher and the chip tooltip. The
top bar holds only navigation: New chat, History, Settings. "Add a source" goes through
the server by default (the key is stored by the server, offered to the runtimes
you tick); runtime detection and a direct API key are the other two paths.
Direct API keys remain supported as a secondary source.

The composer's **Add file** button opens a server-side picker (recent files and
the folders the server may read); a file becomes a context chip like a picked
element — this is the local HTML/Markdown use case.

## Settings — two tiers

The in-pane sheet keeps only quick settings (theme, color, text size, pane,
model source toggle, summaries with "Manage"). Heavy configuration lives on a
full **settings page** (the extension's options tab, shown in the prototype
with a tab strip): setup checklist, server & runtimes, folders, providers &
keys (Add a source as a dialog), privacy, shortcuts, about.

## Color palettes

Six palettes, each with light + dark: Graphite (default), Nocturne (indigo +
brass), Sage (green-grey + terracotta), Ember (warm charcoal + coral), Arctic
(ice blue + cool amber), Mono (greys + electric blue). `tokens/themes.css`
overrides only color tokens under `[data-palette="…"]`; Settings → Color and the
demo rail switch them.

## Layout

Content Pane `flex: 1` · 6 px draggable divider · AI Pane 400 px (320–640),
full height, own scroll. The pane is a stack: top bar → tabs → body → composer.
Radii 6 / 10 / 14 (controls / cards / popovers). Shadows only on floating things.

## Files

```
tokens/tokens.css        all tokens; three theme states (light, system-dark, explicit dark)
tokens/components.css    every component (sk-* classes), colors only via tokens
tokens/themes.css        six color palettes (light + dark each)
icons/skope-icons.svg    icon sprite (reticle, select-text, providers, status…)
components/*.html        one self-contained preview per component, light + dark side by side
screens/*.html           twelve full split-canvas screens, light + dark stacked
preview/ai-skope.html    interactive prototype (single file) — the review surface
src/                     sources; build.mjs inlines tokens, CSS, sprite and partials
```

Rebuild after editing anything under `src/` or `tokens/`:

```
node build.mjs
```

## Theme contract

Tokens are declared on bare `:root` (light), redefined under
`@media (prefers-color-scheme: dark)` guarded as `:root:not([data-theme="light"])`,
and again under `[data-theme="dark"]`. They are also scoped to any element
carrying `data-theme`, which is how a screen shows both palettes on one sheet.
Components never set a color directly — only through tokens.

## Interactive prototype

Open `preview/ai-skope.html`. The rail at the bottom-left jumps between states
(Chat · Picking · Selection · Notes · History · Switcher · Settings · Add source · All settings · Add file · Offline · First run)
and themes. Everything works locally: pick an element on the example page, drag
to select text and use the floating toolbar, switch models, resize the divider,
send a message (canned reply), start a **New chat** (the current one moves to
**History**, grouped this page → this site → everywhere; delete lives there with
undo). Nothing is persisted; no model is called.

## Pushing to Claude Design

Each component and screen starts with a `<!-- @dsCard group="…" -->` marker, so
the Design System pane builds its cards from these files directly.

1. `/design-login`
2. `/design-sync` and point it at this `design/` directory (create the
   "AI Skope" project when asked).

## Further reading

- `docs/SPEC.md` — the product as understood so far, decisions and open questions.
- `docs/SERVER-PLAN.md` — the plan for the Go server (AISS).

## Out of scope for this wave

Manifest, content scripts, real model calls, persistence.
