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

## Phase 1 — Make the server installable (the real gate) — **done**

Nothing else mattered until a stranger could do this, and now they can:

```
brew install marcelocyreno/tap/aiss
aiss start && aiss pair
```

- **goreleaser** (`.goreleaser.yaml`, at the repository root so it can reach the
  LICENSE and the install guide): static builds for darwin and linux on both
  architectures, version stamped through the `-X …/internal/version.Version`
  ldflag. Cross-compiling is free — the SQLite driver is pure Go, so everything
  builds with `CGO_ENABLED=0`.
- **A Homebrew tap** — `marcelocyreno/homebrew-tap`, cask written by goreleaser
  on each release.
- **GitHub Releases** with `checksums.txt`, for people without Homebrew.
- **`.github/workflows/release.yml`** cuts a release on any `v*` tag.

**Signing and notarisation were not needed.** The earlier draft of this plan
called them the most likely thing to stall the release. That was wrong: macOS
applies the quarantine attribute from whichever application downloads a file,
and neither `curl` nor Homebrew does, so Gatekeeper never intervenes. The cask
strips the attribute after staging as a belt-and-braces measure. An Apple
Developer ID becomes necessary only for a `.dmg`, a `.pkg`, or a binary people
download in a browser — none of which is planned.

Still open from this phase, and not blocking:

- A **service file** so the server survives a reboot: launchd plist on macOS,
  systemd user unit on Linux. `aiss start` already detaches, but a user who
  restarts their Mac should not have to think about it.
- A **one-line `curl … | sh` installer** for people without Homebrew. The
  release archives cover them today; this is only convenience.

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

## What is ready, and what is missing

Phase 3 is prepared. Everything the dashboard asks for lives in `store/`:

| | |
|---|---|
| `store/LISTING.md` | every field, in the order the dashboard asks — name, descriptions, single purpose, one justification per permission, data-usage answers, distribution |
| `docs/privacy.md` | the privacy policy, published at marcelocyreno.github.io/ai-skope/privacy |
| `store/screenshots/` | five frames at 1280×800, shot from the running extension — `task store:shots` |
| `store/ai-skope-<version>.zip` | the upload — `task store:package` |
| `homepage_url` | in the manifest |

Done since: the repository is public and MIT licensed, GitHub Pages serves the
install guide and the privacy policy, and every URL in the listing resolves.

Phase 1 is done too: `aiss` installs with one Homebrew command, so a reviewer
can actually exercise the extension.

Still missing:

- **A `HOMEBREW_TAP_GITHUB_TOKEN` secret** on this repository, so the release
  workflow can update the tap on its own. v0.1.0 was released from a
  workstation, which needed no secret; the next tag will not be. See below.
- **Phase 2** — the pane's first run still assumes the reader knows what `aiss`
  is. Now that there is a one-line install command, it should show it.
- **Windows support in the server**, if it is to be claimed.

### The tap token

`GITHUB_TOKEN` in Actions cannot write to another repository, so pushing the
cask to `marcelocyreno/homebrew-tap` needs a token of its own:

1. github.com/settings/personal-access-tokens → **Generate new token**
   (fine-grained)
2. Repository access: **only** `marcelocyreno/homebrew-tap`
3. Permissions: **Contents → Read and write**. Nothing else.
4. On `marcelocyreno/ai-skope`: Settings → Secrets and variables → Actions →
   **New repository secret**, named `HOMEBREW_TAP_GITHUB_TOKEN`

Until that exists, release from a workstation instead:

```
git tag -a v0.2.0 -m "…" && git push origin v0.2.0
GITHUB_TOKEN=$(gh auth token) HOMEBREW_TAP_GITHUB_TOKEN=$(gh auth token) \
  goreleaser release --clean
```

## A note on visibility

Listed and unlisted go through the same review, with the same policies,
justifications and privacy-policy requirement — visibility is a dropdown, not a
review track, and switching later keeps the extension ID and its existing
users. The argument for unlisted first is about product readiness, not process:
until `aiss` installs in one command, a public listing collects one-star
reviews from people who install, find a pairing screen, and leave.

## An honest alternative

For a tool whose audience is developers who already have Claude Code, Codex or
pi installed, the Chrome Web Store may not be the right channel at all. Load
unpacked plus a good README reaches exactly that audience with no review, no
$5, no notarisation, and no listing to maintain. The store earns its keep when
you want people who are *not* already in that world — and by then the install
path has to be effortless anyway, which is Phase 1 either way.
