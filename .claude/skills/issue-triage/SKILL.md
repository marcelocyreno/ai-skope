---
name: issue-triage
description: Review this repo's untriaged open issues — read the screenshots, verify each claim against the code, rewrite the title and body into the house shape, and label them by type and area. Shows the queue and asks once before editing anything.
user-invocable: true
argument-hint: "[issue numbers] [--all] [--dry-run]"
---

Take this repo's open issues from *reported* to *ready to pick up*: a title that says what
should change, a body someone else could act on, and labels saying what kind of work it is
and which half it lands in. You rewrite and label. You do not fix, and you do not close.

The conventions below were not invented for this skill — they are read off the issues in
this repo that were written by hand (#5–#11). Match them.

## Steps

1. **Find the queue.** The absence of `triaged` is the backlog:
   ```bash
   gh issue list --state open --search "-label:triaged" --limit 100 \
     --json number,title,labels,createdAt
   ```
   Issue numbers in `$ARGUMENTS` narrow it to those. `--all` re-triages every open issue
   including ones already marked — for when the convention itself has moved.

2. **Make sure the vocabulary exists.** Idempotent, safe on every run:
   ```bash
   gh label create "triaged"         --color 0e8a16 --force --description "Title and body reviewed and rewritten; ready to pick up"
   gh label create "area: extension" --color 1d76db --force --description "The Chrome extension (extension/)"
   gh label create "area: server"    --color 5319e7 --force --description "The aiss server (server/)"
   gh label create "area: design"    --color c5def5 --force --description "The design kit and visual system (design/)"
   gh label create "area: docs"      --color 0075ca --force --description "README, docs/ and the store listing"
   gh label create "ux"              --color fbca04 --force --description "Interaction or interface quality, not a functional defect"
   ```

3. **Read each issue whole** — body *and* comments, since a correction often lands there:
   `gh issue view "$N" --json title,body,comments`

4. **Open every image.** Most issues here are a screenshot and nothing else: the screenshot
   *is* the report. GitHub's `user-attachments` URLs need the token:
   ```bash
   curl -sSL -H "Authorization: token $(gh auth token)" -o "$SCRATCH/i$N.png" "$URL"
   ```
   Then read that file with the Read tool and actually look at it. Never rewrite an issue
   whose evidence you have not seen — you will describe the wrong thing confidently.

5. **Verify the premise against the code before repeating it.** Grep for what the issue
   claims; open the files it implicates. #13 said the README "doesn't mention brew" — it
   did, eleven lines below the source build, and the real defect was the ordering. Rewrite
   around what is true, and say in the report where the original claim was off.

6. **Title: `Area: imperative summary`.** The area is the commit scope that would fix it —
   `Composer:`, `Model switcher:`, `Side panel:`, `README:`, `Server:`. The rest says what
   should change, not what is annoying. No bare nouns, no "needs improvement".

7. **Body: the house shape, screenshot first**, with alt text describing what it shows.
   Two skeletons, depending on what the issue is:

   | A defect (#5, #6, #7) | A change (#8, #9, #10, #11) |
   |---|---|
   | `## What the screenshot shows` | `## Today` |
   | `## Why` | `## Wanted` |
   | `## Acceptance` | `## Sketch` |
   |  | `## Acceptance criteria` |

   - `## What the screenshot shows` transcribes the evidence — the actual rows, the actual
     text — so the issue survives the image rotting.
   - `## Why` is the root cause, with the offending code quoted and cited as `file.ts:87`.
     If you could not find it, say so; never guess a cause.
   - `## Today` cites what already exists by `file:line`. Half of these turn out to be "the
     function is already there and has no caller".
   - `## Sketch` lists paths grouped per half (extension / server / design) when the work
     spans them — that grouping is the signal the PR will span them too.
   - `## Open questions` whenever a decision is the maintainer's. Name it, do not settle it.

8. **Labels: one type, at least one area, plus `triaged`.**
   - type — `bug`, `enhancement`, `documentation`, `question`
   - area — `area: extension`, `area: server`, `area: design`, `area: docs`; all of them
     when the issue spans halves
   - `ux` alongside a type and never instead of one, when the complaint is interface
     quality rather than a functional defect
   - `good first issue` only when it is self-contained, one half, and no design decision
     is still open

9. **Show the queue and ask once.** One row per issue: number, new title, labels, and what
   changed about it. Then the count you are about to edit. These are public issues, so the
   batch takes one explicit yes. On `--dry-run`, stop here and print the proposed bodies.

10. **Apply**, with the body on stdin so the shell expands nothing inside it:
    ```bash
    gh issue edit "$N" --title "..." --add-label "triaged,area: extension,ux" --body-file - <<'BODY'
    ...
    BODY
    ```
    The quoted `'BODY'` is load-bearing: these bodies carry backticks, `$` and `!` inside
    code samples. Use `--add-label`, never `--remove-label`, unless a label is wrong.

11. **Verify.** Re-read each edited issue: the `<img` survived and the labels landed. An
    edit that silently dropped the reporter's screenshot is worse than no edit at all.

12. **Report** one row per issue, plus anything left untriaged and why.

## Rules

- Never close, reopen, assign or set a milestone. This skill rewrites and labels, nothing
  else; `/git-issue-fix` is what fixes.
- Never drop the reporter's screenshot, and never replace their words with your guess at
  what they meant.
- Never invent an answer to a design question — it belongs under `## Open questions`.
- Never apply `wontfix`, `duplicate` or `invalid`. Those are verdicts for the maintainer to
  pass, not categories to sort into.
- Never add `## Resolution`: it is the close-time convention on #5–#11, written by whoever
  closes the issue. Never strip one that is already there either.
- Never touch a closed issue.
- The queue is a proposal until the user says yes. No edits before step 9.

Re-running is safe: `-label:triaged` excludes everything a previous run touched, so the
next run sees only what has arrived since.

`$ARGUMENTS`: issue numbers to limit the run to, `--all` to re-triage issues already marked
`triaged`, `--dry-run` to stop at the queue, plus free-text steering.
