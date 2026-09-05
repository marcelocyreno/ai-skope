# Publishing AI Skope to the Chrome Web Store

## The thing to understand first

AI Skope is not a self-contained extension. It does nothing at all until the
user installs and runs `aiss` on their own machine, and pairs it with a code
typed from a terminal. That single fact shapes everything below:

- **The store is not the gate — the server is.** Anyone who installs the
  extension and finds a pairing screen with no way forward will uninstall it
  and rate it one star. The companion binary has to be installable in one
  command, on a machine that has never seen this project, before the listing
  goes anywhere near a public audience.
- **Review will look hard at the permissions**, because `<all_urls>` and a
  localhost host permission together describe an extension that can read any
  page and talk to a local process. Both are justifiable here, and the
  justification has to be written plainly.

So the plan is: make the server installable, make the first run explain itself,
then publish — initially **unlisted**, so the link can be shared while the
rough edges are found by people who were told what this is.

## Decisions to make before starting

| Decision | Recommendation |
|---|---|
| Public listing or unlisted? | **Unlisted first.** A dev tool needing a local daemon is not ready for drive-by installs. Go public once the install path is proven on someone else's machine. |
| Which platforms does the server support? | **macOS and Linux at first** (what is tested). Windows needs work: process-group handling and the keychain fallback are untested there. Say so in the listing. |
| Who publishes? | A **personal developer account** is fine to start ($5 one-off). A group publisher matters only if others need dashboard access later. |
| Open source? | **Yes, and say so in the listing.** For an extension that reads pages and runs local agents, a public repository is the strongest trust signal there is — and reviewers can be pointed at it. |

## Phase 1 — Make the server installable (the real gate)

Nothing else matters until a stranger can do this:

```
brew install ai-skope/tap/aiss     # or: curl … | sh
aiss start && aiss pair
```

- **goreleaser** in `server/`: static builds for darwin/arm64, darwin/amd64,
  linux/amd64, linux/arm64; version stamped through the existing
  `-X …/internal/version.Version` ldflag the Makefile already uses.
- **macOS signing and notarisation.** Without it Gatekeeper refuses to run the
  binary and the user sees a scary dialog. Needs an Apple Developer ID
  ($99/year). This is the most likely thing to stall the release.
- **A Homebrew tap** (`ai-skope/homebrew-tap`), published by goreleaser.
- **GitHub Releases** with checksums, plus a one-line install script for people
  without Homebrew.
- A **service file** so the server survives a reboot: launchd plist on macOS,
  systemd user unit on Linux. `aiss start` already detaches, but a user who
  restarts their Mac should not have to think about it.

## Phase 2 — Make the first run explain itself

Today an unpaired pane says "The server isn't running. Start it with
`aiss start`." That is enough for you and useless to anyone else.

- **Install step in the pane** when no server answers: the actual install
  command with a copy button, a link to the docs, and a "check again" that
  polls. This is the single highest-value change for store users.
- **Options page onboarding**: reuse the design's setup checklist — install the
  server, allow a folder, choose a runtime — driven by real state.
- **Explain the pairing code** where it is asked for: why a code exists (so a
  web page cannot pair itself), and where it comes from.
- **A first-run tab** on install (`chrome.runtime.onInstalled`) pointing at the
  install guide. Most extensions that need a companion do this.

## Phase 3 — The listing itself

**Account and package**

- Developer account, $5 one-time registration fee.
- Upload a zip of the **contents** of `extension/dist` (manifest at the root).
  The build already emits no source maps and comes to ~256 KB.
- `manifest.json` needs a `homepage_url` before submission; the rest (MV3,
  no remote code, `minimum_chrome_version`) is already right.

**Assets to produce**

| Asset | Size | Notes |
|---|---|---|
| Icon | 128×128 | done — `icons/icon128.png` |
| Screenshots | 1280×800, up to 5 | capture real ones: chat with an answer, the picker outlining an element, the selection toolbar, the file picker, the options page |
| Small promo tile | 440×280 | optional, but listings without one look unfinished |

The screenshots should show the reticle picking something on a real page —
that is the product in one image, and it is what the design was built around.

**Text to write**

- **Single purpose**, stated in one sentence: *ask a local AI agent about the
  page you are looking at.* The store's single-purpose policy is enforced, and
  everything the extension does has to serve that sentence.
- **Permission justifications**, one per permission. Draft them honestly:

  | Permission | Justification |
  |---|---|
  | `sidePanel` | The extension's entire interface is the side panel. |
  | `storage` | Remembers the pairing token, server address and appearance. |
  | `scripting` | Injects the element picker and selection toolbar into the page, only after the user allows that site. |
  | `tabs` | Reads the active tab's URL and title so answers are about the page in front of the user. |
  | `contextMenus` | "Ask AI Skope about this" on a text selection. |
  | `host_permissions` for 127.0.0.1 / localhost | Talks to the AI Skope Server, a companion application the user installs and runs on their own machine. |
  | `optional_host_permissions` `<all_urls>` | Requested **per site, at the moment the user picks an element or selects text**. Nothing is requested at install time. |

  That last row is the one to get right: an *optional* broad permission,
  requested from a user gesture, is a much easier story than a declared one.

- **Privacy policy**, hosted (GitHub Pages is fine) and linked from the
  dashboard. It has to say, plainly:
  - the extension sends nothing to us — there is no backend of ours;
  - page content leaves the browser only when the user picks an element,
    selects text, or allows the page, and then it goes to the server on their
    own machine;
  - that server passes the question to whichever coding agent and model the
    user configured, and *that* agent may send it to a cloud provider — which
    provider is the user's own choice;
  - chats, notes and provider keys live in a SQLite file on the user's machine
    and can be deleted with `aiss reset`.

- **Data usage disclosures** in the dashboard: answer them against the list
  above. The honest answers are that no data is collected by us, none is sold,
  and none is used for anything unrelated to the single purpose.

## Phase 4 — Submit and survive review

- Submit as **unlisted**. Expect days rather than hours; broad host permissions
  and a companion app both slow review.
- Be ready to answer: *why does this need `<all_urls>`?* and *what is the
  localhost connection?* Point at the repository and at the permission
  justifications above.
- Have a rejection plan. The two most likely reasons are the broad permission
  and the companion-app dependency. Neither is fatal, but both are easier to
  argue with a public repository and a working install path — which is why
  Phase 1 comes first.

## Phase 5 — After it is live

- **Versioning**: bump `manifest.json` and the server together; the extension
  should say when the server is older than it expects (`/v1/capabilities`
  already reports `apiVersion`, which is exactly what this is for).
- **Support**: GitHub issues, linked from the listing.
- **Updates**: the store reviews every update; a new permission triggers a
  fuller review, so add permissions rarely and deliberately.

## What is missing in the repository today

- `LICENSE` — none. Pick one before publishing anything.
- `homepage_url` in the manifest.
- `server/.goreleaser.yaml`, the Homebrew tap, and notarisation.
- A hosted privacy policy and install guide.
- Store screenshots.
- Windows support in the server, if it is to be claimed.

## An honest alternative

For a tool whose audience is developers who already have Claude Code, Codex or
pi installed, the Chrome Web Store may not be the right channel at all. Load
unpacked plus a good README reaches exactly that audience with no review, no
$5, no notarisation, and no listing to maintain. The store earns its keep when
you want people who are *not* already in that world — and by then the install
path has to be effortless anyway, which is Phase 1 either way.
