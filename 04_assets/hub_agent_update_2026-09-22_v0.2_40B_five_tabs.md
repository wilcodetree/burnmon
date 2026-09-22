# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-22 (v0.2 tick 40B) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** Report v0.2's tab-shell restructure (P1, P4, P5) shipped, tagged and pushed.
**Read order:** this file, `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry), `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` section 2.2.
**Supersedes:** nothing.

## 1. Headline
BurnMon v0.2's five-tab shell (Now, History, Sessions, Tools, About) is shipped, merged to
main and tagged `v0.2.0-alpha.1`, pushed to GitHub. Now opens first, History is a one-line
placeholder for 41A, How it works moved into About, the Dutch language toggle is gone.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (GitHub `wilcodetree/burnmon`): `3d6bc88` "feat:
  five tabs, Now default, English only (P1, P4, P5)".
- **Tagged:** `v0.2.0-alpha.1` on `3d6bc88`, annotated, pushed.
- **Pushed:** `main` and the tag are both on `origin` (`e469457..3d6bc88`).
- **Files touched** (full paths): `C:\ZND\projects\burnmon\internal\report\template.html`
  (tab bar, sections, i18n table, deep-link read), `C:\ZND\projects\burnmon\SESSION_LOG.md`
  (new entry prepended, tick 40B).
- **Not touched, left as found:** `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1.2_patch_spec.md`
  showed modified in `git status` at session start; this session never opened it, so it was
  deliberately excluded from the commit and is still sitting locally modified. Flagged to
  Wilco in-session; he has not yet said what to do with it.

## 3. What did NOT happen (and why)
- **History (P2), Sessions tools totals wiring, vendor strip (P3), owner-column UI**: not
  built. Out of scope for this tick; History is a literal placeholder pending 41A.
- **Overview/Months/Weeks/Days render code**: not moved or deleted, only taken off the tab
  bar. Their `<section>` markup and JS render functions (`renderOverview`,
  `drawOverviewCharts`, `renderPeriods`) are still in `template.html`, hidden and
  unreferenced from `TAB_IDS`, kept intentionally for 41A to reuse per the spec's own
  instruction not to move that code yet.
- **Visual click-through of History and About in the running app**: attempted and abandoned.
  `Get-Process` and `FindWindow` from this session's PowerShell tool could not see or address
  the `burnmon.exe` window that `tasklist` confirmed was running, so simulated mouse input
  never reached it (a sandbox session/desktop isolation issue, not an app bug). Only the Now
  tab was confirmed visually, via a real screen capture (`CopyFromScreen`, which does reach
  the interactive desktop) showing Now active on cold start against this laptop's live store.
  History and About content were verified by reading the rendered HTML/JS directly instead.
- **The `2026-09-22_v0.1.2_patch_spec.md` modification**: not committed, not investigated,
  left exactly as `git status` found it.

## 4. Findings worth propagating
- **[RESULT]** `v0.2.0-alpha.1` is tagged and pushed to `origin/main` at `3d6bc88`. Verified
  by the push output the user pasted back (`main -> main`, `[new tag] v0.2.0-alpha.1`).
- **[RESULT]** `go test ./... -count=1` and `.\build.ps1` both green on this change, and
  `node --check` passed on the extracted inline script, all re-run in-session, not carried
  over from a prior tick.
- **[STATE]** A real pre-existing bug was found and fixed as a side effect of this merge: two
  elements shared `id="how"` in `template.html` (the old standalone section and the
  "Refreshing" card inside About), so the "How do I refresh this?" button's `scrollIntoView`
  always landed on the wrong card. Fixed by renaming the card's id to `refresh_info`. Not
  independently re-verified by click (see section 3); low risk, one-line fix, covered by
  reading the resulting DOM.
- **[STATE]** Two "Overview tab" references survive inside the About copy that used to live
  on the standalone How it works tab, now pointing at a tab that is off the nav bar (Overview
  itself still exists, hidden, pending 41A). Left as-is rather than guessed at; see section 7.

## 5. Hub-level decision (if any)
Nothing. This is a project-internal build tick, no cross-project time, park/unpark,
consultancy, positioning or CIPHER-wall call in it.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` (hub one-pager): v0.2 build order now shows week
  40 session B done (P1, P4, P5 shipped, tagged `v0.2.0-alpha.1`).
- `C:\ZND\10_holding\02_roadmap\roadmap.md`: if it tracks BurnMon's v0.2 progress by tick,
  advance it past 40B.
- No `DEADLINES.md` change: the only hard date in `C:\ZND\projects\burnmon\DEADLINES.md` for
  v0.2 is the 2026-11-14 release itself, unaffected by a single tick landing on schedule.

## 7. Open flags for next session
- Decide what to do with `02_roadmap\2026-09-22_v0.1.2_patch_spec.md`'s uncommitted local
  modification: fold into a future commit, or was it meant to be discarded.
- The two "Overview tab" copy references inside About's moved How-it-works content: rewrite
  once P2 (History, 41A) decides Overview's actual fate, not before.
- Visual click-through verification of the History placeholder and the About merge is still
  outstanding; either fix the PowerShell-tool-to-interactive-desktop input path, or verify by
  some other route (a headless render harness, or Wilco eyeballing it once) before trusting
  further GUI-only changes to screenshot verification alone.

## 8. Related files
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` (section 2.2, P1/P4/P5).
- `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry, "2026-09-22, v0.2 40B").
- `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.2_40A_tool_calls.md` and
  `..._39B_migrations_owner.md` (prior, still-unpropagated briefs this week).
