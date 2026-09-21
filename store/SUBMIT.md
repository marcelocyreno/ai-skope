# Chrome Web Store — where this stands

**Last updated: 20 September 2026.** Pick this up cold: everything below is
either done, or a step you can do in one sitting.

`LISTING.md` holds the text to paste, field by field, in the order the
dashboard asks. This file is the checklist around it.

---

## Live

**AI Skope is published, unlisted, since 7 September 2026.** The item lives in
the [developer dashboard](https://chrome.google.com/webstore/devconsole).

**Which version the store serves is not recorded here.** 0.1.0 went up on 7
September. Whether the 0.1.1 package followed was never ticked off below, and
it cannot be read from the repository. Check the item's Package tab before
uploading — the store only accepts a version higher than the one it holds.

Unlisted was the choice. The review is identical to public, and switching
later is a dropdown on the Distribution tab that keeps the extension ID and
the existing users. Going public is the only listing decision left.

## Released

| Tag | Cut | GitHub release | Homebrew tap |
|---|---|---|---|
| `v0.1.0` | 5 September 2026 | four archives + `checksums.txt` | cask written |
| `v0.1.1` | 10 September 2026 | four archives + `checksums.txt` | **never written** |
| `v0.1.2` | 16 September 2026 | four archives + `checksums.txt` | written by hand |

Both workflows failed *after* publishing the release: goreleaser could not
write the cask, `401 Bad credentials` against `marcelocyreno/homebrew-tap`. The
CI log shows `HOMEBREW_TAP_GITHUB_TOKEN:` empty, so the secret was never set
rather than expired.

For `v0.1.2` the cask was written by hand instead — the four archives were
downloaded, checked against the release's own `checksums.txt`, and the cask
updated from 0.1.0 straight to 0.1.2. `brew upgrade --cask aiss` now offers
0.1.0 → 0.1.2. **0.1.1 never reached the tap and never will**; anyone on brew
goes from 0.1.0 to 0.1.2 in one step.

The cask Homebrew currently serves calls `postflight`, which Homebrew has
deprecated in favour of the declarative `postflight_steps`, so every install
prints a warning. `../.goreleaser.yaml` has been changed to emit the new stanza
from `custom_block` — goreleaser hardcodes `postflight` for its `hooks`, so the
stanza is written out by hand there. **The next release carries the fix; the
published cask still warns.** The new stanza parses and loads without the
warning, but it has not been watched actually running during an install.

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

### Next: 0.1.2

`v0.1.1` is taken — the tag is pushed and the release is published — so the
next package and the next server tag are both **0.1.2**. Eight commits are
waiting on `main`, none of them pushed:

- Show the model's name, not the path in front of it
- Give the inverted surfaces an accent of their own
- The right-click menu is opt-in, off by default
- Server: tie a tool call's frames together by its id
- Notes hidden until the feature is finished
- Walk the sent messages with Up and Down
- Choose the default runtime, model and effort
- The design kit rebuilt so the previews match the tokens

Permissions are unchanged, so the seven justifications in `LISTING.md` still
hold and the review should be the quick kind. `LISTING.md` already carries two
of these: the `contextMenus` justification says the entries are off by default,
and Notes is gone from the feature list.

- [x] Version bumped to 0.1.2 — 15 September 2026
- [ ] **Recapture the screenshots** — `task store:shots`. All six frames are
      from 5 September, before Notes was hidden. `3-selection.png` still shows
      a "save note" button the extension no longer has, and the pane still
      carries a Notes tab. The listing text was corrected for this; the images
      were not.
- [x] Package built — `store/ai-skope-0.1.2.zip`, 85 KB, manifest at the
      root, no source maps
- [ ] **Two end-to-end tests fail on `main` — do not upload until they pass.**
      `the model switcher lists what the server offers and changes the chip`
      and `the options page manages folders against the real server`, both in
      `extension/tests/e2e/design-states.spec.ts`. They fail on every run, not
      intermittently. The switcher no longer offers a provider's models, and
      the folders table resolves to seven rows where the test expects one.
      Suspect "choose the default runtime, model and effort", the newest
      commit: it added a Default model control and changed how effort is
      stored, and it is the only recent change to that area that left the spec
      untouched. The packaged zip above was built from this same code.
- [ ] Uploaded and submitted — **the only step left**
- [x] `main` pushed, `v0.1.2` tagged, server released, tap updated —
      16 September 2026

### Next: 0.1.3

`v0.1.2` is cut and tagged, so the next package and the next server tag are
both **0.1.3**. Twelve commits are on `main` since the tag; the ones a reader
of the listing would notice:

- The right-click menu now works while the pane is open, and its toggle moved
  from the options page into the pane's quick settings ([#22](https://github.com/marcelocyreno/ai-skope/issues/22))
- A 1-5 answer-length control in the composer, per message ([#16](https://github.com/marcelocyreno/ai-skope/issues/16))
- The composer's keyboard legends dropped; Copy and Clear chat kept
- The model switcher saves the default model
- The pane no longer repeats the title bar Chrome already draws
- The README leads with the Homebrew install

Permissions are unchanged, so the seven justifications still hold and the
review should be the quick kind. `LISTING.md`'s `contextMenus` justification
was corrected for the moved toggle: it said the entries appear "in the options
page", which stopped being true with #22, and it said "entries" where the
extension only ever creates one.

- [x] Version bumped to 0.1.3 in all three files — 20 September 2026
- [x] Package built — `store/ai-skope-0.1.3.zip`, 85 KB, manifest at the root,
      no source maps
- [x] `LISTING.md` and `PUBLISHING.md` corrected for the moved toggle
- [ ] **The same two end-to-end tests still fail — do not upload.** `the model
      switcher lists what the server offers and changes the chip` and `the
      options page manages folders against the real server`, both in
      `extension/tests/e2e/design-states.spec.ts`. Unchanged since this was
      first written on 15 September: they fail on every run, and they fail on
      `main` as well as on the release branch, so they are not the release's
      doing. The switcher still does not offer a provider's models. The
      failure dumps `/v1/providers` and `/v1/models`: the provider is
      registered and healthy there (`lastTestOk: true`, "2 models",
      `glm-5.3` and `glm-5.3-flash`), but the assertion truncates
      `/v1/models` at 300 characters, so whether those two reach that
      endpoint at all is still unknown. Establishing that is the first step —
      it decides whether the bug is server-side in the `/v1/models` merge or
      panel-side in the switcher.
- [ ] **Recapture the screenshots** — `task store:shots`. Still the 5 September
      frames, now two releases stale: `3-selection.png` shows a "save note"
      button that no longer exists, the pane carries a Notes tab that is gone,
      and the settings frames predate the answer-length control and the moved
      right-click toggle.
- [ ] Uploaded and submitted
- [ ] `v0.1.3` tagged and the server released

`internal/files` carries a separate, unrelated flake worth an issue of its
own: `TestIndexerIndexesAndPrunes` fails when two index passes of the same
folder land in the same millisecond. `IndexFolder` stamps rows with
`started := store.Now()` (`index.go:70`, millisecond resolution) and prunes
`WHERE indexed_at < started` (`store/files.go:99`), so a pass that begins in
the same millisecond as the previous one prunes nothing. Real folders take
longer than a millisecond to walk, which is why only the test sees it.

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

- [ ] **`HOMEBREW_TAP_GITHUB_TOKEN` secret** — never set, and it has now cost
      two releases. Steps in `../docs/PUBLISHING.md` → "The tap token". Until
      it is set, the tap has to be written by hand after every tag, and it
      lags the release page until someone does.
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
