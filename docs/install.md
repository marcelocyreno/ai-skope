---
title: Installing AI Skope
---

# Installing AI Skope

AI Skope is two halves, and it does nothing with only one of them:

- **the extension** — the pane in your browser;
- **AI Skope Server (`aiss`)** — a small program on your own computer that
  drives the coding agents you already have.

There is no cloud service. Nothing works until the server is running on the
same machine as the browser.

## What you need first

At least one supported coding agent, already installed and working from your
terminal:

| Agent | Command |
|---|---|
| Claude Code | `claude` |
| Codex | `codex` |
| pi | `pi` |
| omp | `omp` |
| opencode | `opencode` |

Whatever model that agent is configured with is what answers your questions.
AI Skope does not ask you for a provider key, and it does not have one.

macOS or Linux. Windows is not supported yet.

## 1. Build and install the server

> There is no packaged release yet, so this step needs [Go 1.25 or
> newer](https://go.dev/dl/). A `brew install` and a one-line installer are
> planned; until then, this is the honest instruction.

```sh
git clone https://github.com/marcelocyreno/ai-skope.git
cd ai-skope/server
go build -o aiss ./cmd/aiss
sudo mv aiss /usr/local/bin/    # or anywhere on your PATH
```

Check it can see your agents:

```sh
aiss doctor
```

That prints what it found and what to fix. Then start it:

```sh
aiss start
```

It listens on `127.0.0.1:7331` and accepts connections from nowhere else.

## 2. Install the extension

Until the Chrome Web Store listing is live, load it unpacked. This works in
Chrome, Brave and any other Chromium browser.

```sh
cd ai-skope/extension
npm install
npm run build
```

Then:

1. open `chrome://extensions` (or `brave://extensions`)
2. turn on **Developer mode**, top right
3. click **Load unpacked**
4. choose `ai-skope/extension/dist`

Developer mode has to stay on for as long as the extension is loaded this way.

## 3. Pair the browser with the server

The extension cannot talk to the server until you prove the two belong to the
same person. In a terminal:

```sh
aiss pair
```

That prints a one-time, eight-character code. Open the AI Skope pane — the
toolbar icon, or `⌘⇧A` / `Ctrl+Shift+A` — and type the code in.

**Why a code exists:** the server can read the folders you allow it and can run
your coding agents. Without pairing, any other program on your computer could
do both. The code is exchanged once for a token; you will not be asked again on
that browser.

## 4. Allow a folder, if you want to ask about local files

```sh
aiss folders add ~/dev/some-project
```

The server reads files only inside folders you have added, and refuses
credentials, keys, `.env` files and shell history even inside them.

## Using it

- **Ask about the page** — type a question. The first time on a site, the pane
  asks whether to send the page's text.
- **Point at one thing** — the reticle button, or `⌘⇧K`, then click an element.
- **Quote something** — select text on the page and a small toolbar appears.
- **Ask about a file** — the folder button lists what you have allowed.

The first time you pick or select on a site, your browser will ask whether to
grant AI Skope access to it. It asks once per site, and the extension requests
nothing at install time.

## When something is wrong

| What you see | What it means |
|---|---|
| "The server isn't reachable" | `aiss start` has not been run, or the server stopped. `aiss status` will say. |
| "Pair this browser" after you already paired | The token was cleared — reinstalling the extension does that. Run `aiss pair` again. |
| An answer that ignores the page | Page access may be set to **Never** in settings, or the site is on your blocked list. |
| The agent errors immediately | Run `aiss doctor`, then try the same agent from your terminal. AI Skope surfaces whatever it says. |

Logs: `aiss logs --tail 50`.

## Removing everything

```sh
aiss reset
```

That deletes the chats, notes, file index, settings and any provider keys the
server holds, including those in your OS keychain. Removing the extension
clears what the browser stored.

## Questions

[github.com/marcelocyreno/ai-skope/issues](https://github.com/marcelocyreno/ai-skope/issues)
