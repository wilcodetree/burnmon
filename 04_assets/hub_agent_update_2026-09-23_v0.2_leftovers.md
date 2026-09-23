# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 (v0.2 leftovers session) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** Report the close of three mechanical leftovers from the v0.2 ship, no new features, no version bump.
**Read order:** this file, `C:\ZND\projects\burnmon\STATUS.md`, `C:\ZND\projects\burnmon\SESSION_LOG.md`
**Supersedes:** nothing

## 1. Headline
Three v0.2 leftovers closed: the stale "Overview tab" copy in About is gone, the v0.1.2
patch spec's supposedly pending diff turned out to already be committed (nothing to do),
and the only untracked file in the tree was this session's own prompt file. Tests and
build both green. Committed locally, not pushed.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (local commits, not pushed): About copy fix,
  STATUS/SESSION_LOG updates, and tracking this brief plus the session prompt file.
- **Files touched** (full paths):
  - `C:\ZND\projects\burnmon\internal\report\template.html` (deleted the two sentences
    naming the removed Overview tab, lines 349 and 420 before the edit)
  - `C:\ZND\projects\burnmon\STATUS.md` (Known gaps: dropped the Overview-tab clause,
    kept the still-true "About only covers Claude's seat model" gap)
  - `C:\ZND\projects\burnmon\SESSION_LOG.md` (new entry prepended)
  - `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.2_leftovers_prompt.md` (now tracked)
  - `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-23_v0.2_leftovers.md` (this file)
- **On a branch, not merged:** none; all work is on `main`, committed locally.
- **Untracked / outside a repo:** none remaining; `git status --porcelain -uall` showed
  exactly one untracked file at session start (the session prompt) and none by the end.

## 3. What did NOT happen (and why)
- Not pushed to `origin/main`. Push command handed to Wilco, per the session prompt's own
  instruction.
- No commit made for the v0.1.2 patch spec item: `git diff` and
  `git status --porcelain -uall` against `02_roadmap\2026-09-22_v0.1.2_patch_spec.md` both
  showed nothing pending, and `git log` shows the file last touched in commit `876c37e`,
  already on `main`. The session prompt's premise (a modified working tree) did not hold;
  nothing to fix, no diff to show Wilco.
- No other untracked `04_assets` files were committed, because there were none: all 26
  files already under `04_assets\`, hub briefs included, were already tracked
  (`git ls-files 04_assets | wc -l` matched the directory's file count).
- No version bump, no tag: out of scope for this session by instruction.
- Did not touch the dead `#overview` markup or any other About text beyond the two
  sentences named in the prompt.

## 4. Findings worth propagating
- [RESULT] History does not show the same figure the deleted About sentences claimed.
  `internal\history\history.go:95-113` (`eventCostUSD`) computes plain API list price,
  gated on vendor coverage; the subscription-basis "personal cut of the company bill"
  figure the About text described has no live page to point at until the subscription/API
  cost-basis split ships (STATUS's own v0.3 scope). Sentences deleted outright rather than
  redirected to History, per the session prompt's own instruction for that case.
- [RESULT] `node --check` on the script extracted from `template.html` passed,
  `go test ./... -count=1` green across every package, `.\build.ps1` green (both exes
  built).
- [STATE] Local commits exist on `main` but are not pushed.

## 5. Hub-level decision (if any)
Nothing. Purely mechanical, project-internal.

## 6. What the next hub read should update
Nothing new beyond what BurnMon's own `STATUS.md` already carries; no roadmap, deadline,
or portfolio figure changed this session. `10_holding\01_projects\burnmon.md` needs no
edit unless the hub wants to note the leftovers are closed.

## 7. Open flags for next session
- Push to `origin/main` still pending, Wilco's call.
- v0.3 scope (per-vendor cost, dev/business switch, full client map, macOS/Linux builds)
  unchanged, still the next real work.

## 8. Related files
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.2_leftovers_prompt.md` (this session's prompt)
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1.2_patch_spec.md` (checked, already committed)
- `C:\ZND\projects\burnmon\STATUS.md`, `C:\ZND\projects\burnmon\SESSION_LOG.md`
