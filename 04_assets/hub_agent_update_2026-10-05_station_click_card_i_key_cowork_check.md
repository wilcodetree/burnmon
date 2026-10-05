# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-10-05 (Claude Code session on the laptop) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** Tells the hub that Station backlog items A and B are built and committed locally, and that item C (Cowork shows Waiting while busy) was disproved on the stored data.
**Read order:** this file, then `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md` (status block at the end), then `C:\ZND\50_projects\burnmon\SESSION_LOG.md` (top paragraph).
**Supersedes:** nothing. Follows `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-10-05_copilot_stage_and_context.md`.

## 1. Headline
Station items A (compact click card) and B (I key, 3D Station in the burn zone) are code-complete and committed locally as `5a93808`, not pushed. Item C was disproved on a copy of the store: the stored Cowork trails do not show the Waiting-while-busy pattern. No code changed for C.

## 2. What changed on disk
- **Committed** in `C:\ZND\50_projects\burnmon` (local `main`): `5a93808` station: compact click card, I key for the 3D view, Esc closes every Station mode. Made after the work, not by this session; checked live with `git log` and `git show --stat`.
- **Files touched:**
  - `C:\ZND\50_projects\burnmon\cmd\burnmon-dev\page.html`: compact card CSS for a popup opened from the Station, `placeStationCard`, the I key, Esc closes any Station mode, `__bdevStationView`.
  - `C:\ZND\50_projects\burnmon\cmd\burnmon-dev\station\station.js`: `mount(el, mode, view)`, `view()`, and the click callback now passes the click position.
  - `C:\ZND\50_projects\burnmon\tools\uicheck\check_d23.go`: I key steps, Esc order with a spy on `bdevExitFullscreen`, the same Esc order for O.
  - `C:\ZND\50_projects\burnmon\tools\uicheck\check_d25.go`: new check for the click card (size, wrapping, containment, placement) in P, O and I.
  - `C:\ZND\50_projects\burnmon\SESSION_LOG.md` and `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`: session paragraph and status block.
- **Untracked at the time of writing:** this brief.

## 3. What did NOT happen (and why)
- **Not pushed.** `origin/main` is at `e3c2506` (checked with `git ls-remote --heads origin`); local `main` is two commits ahead (`913c712`, `5a93808`). No tag. Pushing is Wilco's step.
- **No code change for C.** The hypothesis was disproved, so there was nothing to fix and no failing test to write.
- **The live Cowork case Wilco saw was not reproduced.** The store copy has no record of which session or time he meant.
- **No README mention** of the I key, per the roadmap decision (the Station stays undocumented).
- **`w1` was not made to pass.** It fails on the 1.5 s first paint of `burnmon.exe`, which this work did not touch.
- The copy of `burnmon.db` used for C was deleted after the analysis. No prompt text, project name, file path or client name was printed, written to a file or put into a test.

## 4. Findings worth propagating
- [RESULT] A: `d25` failed first on the old build (card 480 px wide, content 3229 px against 478 px clientWidth, card mid-window) and passes after the fix, on site and space Station themes and on the light page, in P, O and I. Source: `.\scripts\uicheck.ps1 d25`, run live, passed three times.
- [RESULT] B: `d23` extended for the I key failed first and passes. I opens the 3D view in the burn zone, O swaps to the plan view and back, T flips themes, and I closes it.
- [RESULT] Esc did not close O before this session (measured by the red `d23` run: the plan view stayed open after two Esc presses and the second Esc reached fullscreen). It now closes every Station mode, after a turn popup and before fullscreen. This goes beyond what the backlog asked for; the roadmap line "same order as O" assumed O already did it.
- [RESULT] C, measured on a copy of `burnmon.db` (aggregates only): Cowork has 3,407 turn rows against 48,294 tool calls; the key a call carries matches an event row for 95.1 percent of Cowork calls. A replay of `internal\stage\stage.go` ported to Python found: the "call is in the latest turn" input says busy in 4,002 of 4,002 Cowork moments with a turn row (Claude Code: 16,885 of 16,885); a gap over 180 s after a tool call occurs in 0.52 percent of Cowork cases against 0.79 percent for Claude Code; Cowork "no tool in the latest turn" verdicts have a median of 169 s to the next activity (Claude Code: 11 s, because its usage row lands before its own call row); transcript files are modified a median 0.2 s after their newest recorded row.
- [STATE] C, inferred and not proven: the replay assumes a row is visible when it is stamped and has no timestamp for when a tool result lands, so a live ingestion delay or a case outside the stored data is not ruled out. Cowork's AskUserQuestion use is about 10 percent of its Waiting hours against 1 percent for Claude Code; that is a real wait, not a fault.
- [RESULT] Verification: `go vet ./...` clean; `go test ./... -count=1` ok in every package (one earlier run exited 1 on a Windows "Access is denied" removing a temp test exe, the rerun exited 0); `.\build.ps1` ok; `.\scripts\uicheck.ps1` d3, d23, d24, d25 and w0, w2 to w8 pass. `w1` fails on first paint (known flake, repeated). d23 and d25 each failed once when no key press reached the page (window focus) and passed on rerun.

## 5. Hub-level decision (if any)
Nothing for `C:\ZND\10_holding\03_logs\decisions.md`. The Esc change is project-internal and reversible in one line.

## 6. What the next hub read should update
- `C:\ZND\50_projects\burnmon\STATUS.md`: Station backlog A and B done, C closed as not reproduced.
- `C:\ZND\10_holding\01_projects\burnmon.md`: one line for the hub view.
- `C:\ZND\10_holding\02_roadmap\roadmap.md` and the week plan: no change expected.

Tracker rows moved: none.

## 7. Open flags for next session
- Push of `913c712` and `5a93808` is waiting on Wilco.
- Ask Wilco for the time or a screenshot of the Cowork session that read Waiting while busy; without it C stays closed on the stored data only.
- Decide whether Esc closing O is wanted (it is now on).
- `w1` keeps failing on first paint; the repo notes already call it a known flake.
- d23 and d25 depend on window focus; rerun solo before suspecting code.

## 8. Related files
- `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`
- `C:\ZND\50_projects\burnmon\tools\uicheck\check_d25.go`
- `C:\ZND\50_projects\burnmon\internal\stage\stage.go`
