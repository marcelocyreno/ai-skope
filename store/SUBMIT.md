# Submitting to the Chrome Web Store — where this stands

**Last updated: 5 September 2026.** Pick this up cold: everything below is
either done, or a step you can do in one sitting.

`LISTING.md` holds the text to paste, field by field, in the order the
dashboard asks. This file is the checklist around it.

---

## Done — do not redo

- [x] **Repository public and MIT licensed** —
      [github.com/marcelocyreno/ai-skope](https://github.com/marcelocyreno/ai-skope).
      History audited before the first push: no credentials, no personal
      paths, single author identity, no session URLs.
- [x] **Server released** — `brew install marcelocyreno/tap/aiss` works, and
      was verified by uninstalling and reinstalling from the tap.
      [v0.1.0](https://github.com/marcelocyreno/ai-skope/releases/tag/v0.1.0),
      four platforms plus `checksums.txt`.
- [x] **Docs published** — [install guide](https://marcelocyreno.github.io/ai-skope/install)
      and [privacy policy](https://marcelocyreno.github.io/ai-skope/privacy),
      both returning 200.
- [x] **Package built** — `store/ai-skope-0.1.0.zip`, 80 KB, manifest at the
      root, no source maps.
- [x] **Six screenshots** at 1280×800 in `store/screenshots/`.
- [x] **Listing text written** — descriptions, single purpose, seven permission
      justifications, data-usage answers. All in `LISTING.md`.
- [x] **First run explains itself** — an unpaired pane with no server shows the
      install command rather than assuming the reader has `aiss`.

## Pending — the actual submission

Nothing here needs code. It is roughly twenty minutes of forms.

- [ ] **1. Register.** [chrome.google.com/webstore/devconsole](https://chrome.google.com/webstore/devconsole)
      — US$5 one-time. Turn on 2-Step Verification (required) and verify the
      publisher email that will show on the listing.
- [ ] **2. Upload the package.** `store/ai-skope-0.1.0.zip`.
      Rebuild first if the extension changed: `task store:package`.
- [ ] **3. Store listing tab.** Name, short description, detailed description,
      category **Developer Tools**, language **English (United States)** — all
      under "Store listing tab" in `LISTING.md`.
- [ ] **4. Screenshots.** Upload five, in this order:
      `1-answer`, `2-picker`, `3-selection`, `4-files`, `6-first-run`.
      `5-settings` is the spare — see `LISTING.md` for why the first-run frame
      earns its place over it.
- [ ] **5. Privacy tab.** Single purpose, one justification per permission
      (seven of them), remote code = **No**, data usage: tick **Website
      content** and **Web history** as collected, then certify all three
      statements. Privacy policy URL:
      `https://marcelocyreno.github.io/ai-skope/privacy`
- [ ] **6. Distribution tab.** Visibility (see below), all regions, free.
- [ ] **7. Submit.**

### The one decision left

**Unlisted or public.** The review is identical — same policies, same
justifications, same privacy requirement. Visibility is a dropdown, and
switching later keeps the extension ID and existing users.

The case for unlisted first is no longer about the install path, which is now
one Homebrew command. It is only that this has never been installed by anyone
who did not build it. Unlisted lets you hand the link to a few people first.

### If review pushes back

The `<all_urls>` permission is what they will ask about. The answer:

> It is an **optional** host permission, requested per site at the moment the
> user first picks an element or selects text on that site, from their own
> click. Nothing is requested at install time.

That is a much easier position than a declared broad permission. If they ask
how to test the extension, point them at
<https://marcelocyreno.github.io/ai-skope/install> — the server installs with
one command, so they can exercise it.

The second likely question is the companion application. It is a normal
pattern (password managers, hardware wallets), the code is public, and the
extension contacts nothing but `127.0.0.1`.

---

## After it is live

- [ ] **`HOMEBREW_TAP_GITHUB_TOKEN` secret** so CI can cut releases without a
      workstation. Steps in `../docs/PUBLISHING.md` → "The tap token".
      Until then: `task release` after pushing a tag.
- [ ] **Version bumps go together.** `extension/manifest.json` and the server
      tag should move as a pair. `/v1/capabilities` already reports
      `apiVersion`, which is what the extension should check when the server is
      older than it expects.
- [ ] **Support** — GitHub issues, linked from the listing.
- [ ] Adding a permission later triggers a fuller review. Add them rarely.

## Still open in the product

Not blocking submission, in rough order of value:

- [ ] **A service file** so the server survives a reboot — launchd plist on
      macOS, systemd user unit on Linux.
- [ ] **A `curl … | sh` installer** for people without Homebrew. The release
      archives cover them; this is convenience.
- [ ] **Windows support** in the server. Process-group handling and the
      keychain fallback are untested there. The listing says macOS and Linux.
- [ ] **Direct provider API keys in the extension**, for when the server is
      down. The design allows it; the extension is server-only today.
- [ ] **A 440×280 promo tile.** Optional, but listings without one look
      unfinished.

---

## Commands

```sh
task store:package    # rebuild the upload zip
task store:shots      # recapture all six screenshots
task test             # everything that costs nothing
task release          # cut a release from a tag already pushed
```

Screenshots use a scripted agent so they are reproducible. To shoot them
against a real model instead:

```sh
SKOPE_SHOT_RUNTIME=claude-code SKOPE_SHOT_MODEL=sonnet task store:shots
```
