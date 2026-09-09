# Chrome Web Store — where this stands

**Last updated: 8 September 2026.** Pick this up cold: everything below is
either done, or a step you can do in one sitting.

`LISTING.md` holds the text to paste, field by field, in the order the
dashboard asks. This file is the checklist around it.

---

## Live

**AI Skope 0.1.0 is published, unlisted, since 7 September 2026.** The item
lives in the [developer dashboard](https://chrome.google.com/webstore/devconsole).
It was built from the package of 5 September, so the store build predates
everything merged after that day — see "Next: 0.1.1" below.

Unlisted was the choice. The review is identical to public, and switching
later is a dropdown on the Distribution tab that keeps the extension ID and
the existing users. Going public is the only listing decision left.

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
- [x] **Six screenshots** at 1280×800 in `store/screenshots/`.
- [x] **Listing text written** — descriptions, single purpose, seven permission
      justifications, data-usage answers. All in `LISTING.md`.
- [x] **First run explains itself** — an unpaired pane with no server shows the
      install command rather than assuming the reader has `aiss`.
- [x] **Developer account registered** — US$5 paid, 2-Step Verification on,
      publisher email verified.
- [x] **First submission** — package uploaded, store listing, five
      screenshots, privacy tab (single purpose, seven justifications, no
      remote code, data usage, privacy policy URL) and distribution (unlisted,
      all regions, free) filled in. Review passed; published 7 September 2026.

## Updating — the checklist for every new version

Nothing here is new work, but the order matters. The store rejects a package
whose `version` is not higher than the one it already has, and it reviews
every update.

- [ ] **1. Bump the version** in `extension/manifest.json`,
      `extension/package.json` and the root entry of
      `extension/package-lock.json`. One to four dot-separated integers only —
      Chrome does not accept `0.1.1-beta`. The server tag moves with it
      (step 7).
- [ ] **2. Build the package.** `task test && task store:package` writes
      `store/ai-skope-<version>.zip`. Delete the previous zip by hand; the
      task only removes one with the same name.
- [ ] **3. Upload.** Dashboard → AI Skope → **Package** tab → **Upload new
      package**.
- [ ] **4. Listing and privacy tabs.** Leave them alone unless something
      changed. A new permission needs a new justification on the Privacy tab
      and triggers a fuller review — add them rarely. A new keyboard shortcut
      is not a permission.
- [ ] **5. Screenshots** — only if the UI changed visibly. `task store:shots`
      recaptures all six; upload the same five as the first time:
      `1-answer`, `2-picker`, `3-selection`, `4-files`, `6-first-run`.
- [ ] **6. Submit for review.** "Publish automatically after it has passed
      review" is ticked by default; untick it to pick the moment yourself.
      Installed users update on their own within a few hours of publish.
- [ ] **7. Release the server** at the same version:
      `git tag vX.Y.Z && git push origin vX.Y.Z && task release`.

### Next: 0.1.1

What 0.1.0 on the store does not have:

- Markdown tables in the transcript (#1)
- Copy any message from the chat, plus the `copy-last-answer` shortcut (#3)
- The model switcher: agent-reported models, runtime toggles, a Mask panic (#2)
- Server: answer read-only from the allowed folders instead of plan mode (#4)

Permissions are unchanged, so the seven justifications in `LISTING.md` still
hold and the review should be the quick kind.

- [x] Version bumped to 0.1.1 — 8 September 2026
- [x] Package built — `store/ai-skope-0.1.1.zip`, 84 KB, manifest at the
      root, no source maps
- [ ] Uploaded and submitted
- [ ] Server `v0.1.1` tagged and released

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

### Going public

A dropdown on the **Distribution** tab. No new review; the extension ID and
the existing users stay. The case for waiting was only that nobody who did
not build it had installed it. Once a few people have used it from the
unlisted link and nothing came back, flip it.

---

## Still to do on the release side

- [ ] **`HOMEBREW_TAP_GITHUB_TOKEN` secret** so CI can cut releases without a
      workstation. Steps in `../docs/PUBLISHING.md` → "The tap token".
      Until then: `task release` after pushing a tag.
- [ ] **Support URL** on the Store listing tab — GitHub issues. Not filled in
      at the first submission.
- [ ] **Say when the server is older than the extension expects.**
      `/v1/capabilities` already reports `apiVersion`, which is what the
      extension should check.

## Still open in the product

Not blocking anything, in rough order of value:

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
