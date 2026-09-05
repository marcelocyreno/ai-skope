# AI Skope — Privacy Policy

**Last updated: 5 September 2026**

## The short version

AI Skope has no backend. There is no AI Skope account, no AI Skope server on
the internet, and no telemetry. Nothing you do in the extension is sent to the
developer, because there is nowhere for it to be sent.

What the extension talks to is **AI Skope Server** (`aiss`) — a program you
install and run on your own computer. Your chats, notes and settings live in a
file on that computer.

## What the extension reads

**Nothing, until you act.** AI Skope requests no access to any website when you
install it. The first time you pick an element or select text on a site, your
browser asks whether to grant access to that site, and you decide. Access is
granted one site at a time.

Once a site is allowed, page content leaves your browser only when you:

- pick an element and ask about it,
- select text and add it to a question or a note,
- answer **yes** to "send this page's text with your question", or
- have set **Page access** to **Always** in settings.

Setting **Page access** to **Never** stops all of it. You can also block
individual sites, and AI Skope will not read or send anything from them
regardless of anything else.

Alongside page content, the extension reads the **URL and title of your active
tab**. This is how the pane knows which page you are asking about and keeps a
separate conversation per page.

## Where it goes

To `http://127.0.0.1` — AI Skope Server, on your own machine. That is the only
network destination the extension contacts.

The server then passes your question to whichever coding agent you configured:
Claude Code, Codex, pi, omp or opencode. **That agent may send your question,
and the page content attached to it, to whichever AI provider it is set up
with.** Which provider that is, and what its data practices are, is entirely
your own choice and configuration — the same one that already applies when you
use that agent from your terminal.

This is the one place data can leave your computer, and it leaves under an
arrangement you made before installing AI Skope.

## What is stored, and where

**In the browser**, via extension storage: the pairing token for your server,
the server address, and your appearance settings (theme, palette, text size).

**On your computer**, in a SQLite database owned by AI Skope Server: your
chats, your notes, and the index of files in folders you allowed.

**In your system keychain**, if you add provider keys through the server: the
keys themselves. The extension never receives them.

Nothing is stored anywhere else.

## Local files

AI Skope Server reads files only inside folders you have explicitly allowed. It
refuses everything outside them, and refuses credentials, keys, environment
files and shell history even inside them.

## Deleting your data

- Individual chats and notes: delete them in the pane.
- Everything the server holds: `aiss reset`.
- Everything the extension holds: remove the extension.
- A site's access: revoke it in your browser's extension settings.

## Children

AI Skope is a developer tool and is not directed at children under 13.

## Changes

Material changes to this policy will be reflected here with a new date, and in
the extension's release notes.

## Contact

Issues and questions: [REPO URL]/issues
