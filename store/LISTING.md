# Chrome Web Store listing

Everything the dashboard asks for, in the order it asks. Copy each block into
the matching field. Anything in **[brackets]** is a decision still to make.

Sibling files: `PRIVACY.md` (host it and link it), `screenshots/` (five frames
at 1280×800, regenerate with `task store:shots`).

---

## Package

Upload `store/ai-skope-<version>.zip`, built by `task store:package` — a zip of
the *contents* of `extension/dist`, manifest at the root, no source maps,
around 260 KB.

Before the first upload, confirm in `extension/manifest.json`:

- `version` matches what you want the store to show (currently **0.1.0**)
- `homepage_url` points at a page that exists

---

## Store listing tab

**Name**

```
AI Skope
```

**Short description** (132 characters max)

```
Aim at anything on a page and ask a local coding agent about it. Your agents, your machine, your keys.
```

**Category**

```
Developer Tools
```

**Language**

```
English (United States)
```

**Detailed description**

```
AI Skope splits your browser in two: the page on the left, a full-height AI pane on the right. Aim at what you want to ask about — pick an element, drag-select text, or attach a file from your own disk — and ask.

The answers come from coding agents already installed on your computer.

WHAT MAKES IT DIFFERENT

AI Skope has no cloud service behind it. It talks to AI Skope Server (aiss), a small program you install and run yourself, which drives the coding agents you already use: Claude Code, Codex, pi, omp and opencode. Whatever model those are configured with is what answers.

That means your provider keys stay where they already are. There is no account to create, nothing to sign into, and no server of ours in the path.

WHAT YOU CAN DO

• Ask about the page you are reading, grounded in what is actually on it
• Point at one element — a pricing table, an error message, a diff — and ask about just that
• Select text and turn it into a question or a note without leaving the page
• Ask about local files: a README, a Markdown note, an HTML file on your own disk, from folders you explicitly allow
• Keep a chat per page, with history, and notes that stay attached to where you made them
• Switch model or agent mid-conversation from the pane

WHAT IT READS, AND WHEN

Nothing, until you say so. The extension asks for no site access when you install it. The first time you pick an element or select text on a site, Chrome asks whether to allow that site — one site at a time, from your click.

Page content is sent only when you pick something, select something, or answer yes to "send this page's text". You can set that to Always or Never, and block individual sites outright.

BEFORE YOU INSTALL

This extension does nothing on its own. You need AI Skope Server running on the same computer, and at least one supported coding agent installed. Setup instructions: [INSTALL URL]

Requires macOS or Linux for the server. Windows is not supported yet.

OPEN SOURCE

[REPO URL]
```

> The last two sections are load-bearing. A user who installs this without the
> server will find a pairing screen and nothing else, so the listing has to say
> so before they click Add.

---

## Privacy tab

**Single purpose**

```
AI Skope lets a user ask a coding agent running on their own computer about the web page they are currently looking at.
```

**Permission justifications**

| Field | Text to paste |
|---|---|
| `sidePanel` | The extension's entire user interface is the side panel. There is no other surface. |
| `storage` | Stores the pairing token for the local server, the server address, and appearance settings (theme, palette, text size). Nothing is stored remotely. |
| `scripting` | Injects the element picker and the text-selection toolbar into the page, and reads the page's text when the user asks a question about it. Only ever on a site the user has separately allowed. |
| `tabs` | Reads the active tab's URL and title so the pane can describe the page the user is looking at, keep a separate conversation per page, and follow along when they navigate. |
| `contextMenus` | Adds "Ask AI Skope about this" to the right-click menu on selected text. |
| Host permission `http://127.0.0.1/*`, `http://localhost/*` | Communicates with AI Skope Server, a companion application the user installs and runs on their own computer. This is the only network destination the extension contacts. |
| Optional host permission `<all_urls>` | Requested per site, at the moment the user first picks an element or selects text on that site, from their own click. Nothing is requested at install time, and access to a site is never assumed. |

> The `<all_urls>` row is the one reviewers focus on. Being *optional* and
> requested from a user gesture is a far easier position to defend than a
> declared broad permission — say so plainly if asked.

**Remote code**

```
No, I am not using remote code.
```

Everything executes from the package. No `eval`, no remotely-hosted scripts.

**Data usage** — tick and certify:

| Question | Answer |
|---|---|
| Personally identifiable information | Not collected |
| Health information | Not collected |
| Financial and payment information | Not collected |
| Authentication information | Not collected |
| Personal communications | Not collected |
| Location | Not collected |
| Web history | Not collected |
| User activity | Not collected |
| Website content | **Collected** — see note below |

For "Website content", the honest position: page content is transmitted, but
only to a server on the user's own machine, only on their action, and it is
never sent to the developer. Then certify all three:

- not being sold to third parties
- not being used or transferred for purposes unrelated to the single purpose
- not being used or transferred to determine creditworthiness or for lending

**Privacy policy URL**

```
[PRIVACY POLICY URL]
```

Host `store/PRIVACY.md` — GitHub Pages is enough — and paste the URL.

---

## Distribution tab

**Visibility**

Unlisted or Public. The review is identical either way; see `docs/PUBLISHING.md`
for why unlisted first is the safer opening move while the server still has to
be built from source. Switching later keeps the same extension ID and existing
users.

**Regions**: all.
**Pricing**: free.

---

## Screenshots

Five frames in `screenshots/`, all 1280×800. Regenerate any time with
`task store:shots`.

| File | Shows |
|---|---|
| `1-answer.png` | A grounded answer beside the page it is about |
| `2-picker.png` | The reticle outlining an element, with its selector |
| `3-selection.png` | The selection toolbar: add to chat, ask, save note |
| `4-files.png` | The file picker searching folders the user allowed |
| `5-settings.png` | Folders, providers and privacy settings |

**The answers in these frames come from a scripted agent**, not a live model, so
the frames are reproducible and shooting them costs nothing. That is normal for
product screenshots, but if you would rather the listing show a real model's
words, reshoot against an installed agent:

```
SKOPE_SHOT_RUNTIME=claude-code SKOPE_SHOT_MODEL=sonnet task store:shots
```

The frames deliberately avoid absolute paths under your home directory — the
settings frame is scrolled past the runtimes table for exactly that reason.

**Small promo tile**, 440×280, is optional. Listings without one look
unfinished, but it is not required to submit.

---

## Account, once

- Register as a Chrome Web Store developer — US$5, one time, per account.
- Turn on 2-Step Verification; the dashboard requires it.
- Verify the publisher email that will appear on the listing.

---

## Still to decide

- **`[REPO URL]`** — the repository is not on GitHub yet. `homepage_url` in the
  manifest currently points at `https://github.com/marcelocyreno/ai-skope`.
- **`[INSTALL URL]`** and **`[PRIVACY POLICY URL]`** — both need somewhere to
  live. GitHub Pages on the same repository covers both.
- **A licence.** The repository has none. Not required to submit, but "open
  source" in the description is a claim you cannot back without one.
