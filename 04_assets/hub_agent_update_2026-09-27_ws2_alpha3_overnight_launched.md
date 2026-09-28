# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-27 (hub chat, Cowork, Opus 5.5, written 23:40 while the run is in flight) - **Owner:** Wilco de Tree
**Project:** BurnMon (WS2, BurnMon Dev)
**Purpose:** tell the hub that v0.4.0-alpha.3 was specced, pulled forward from "later", and
launched as one unattended overnight Claude Code session that has not finished yet.
**Read order:** this file,
`C:\ZND\projects\burnmon\02_roadmap\2026-09-27_ws2_alpha3_system_cadence_todo_scroll.md`,
`C:\ZND\projects\burnmon\02_roadmap\2026-09-27_ws2_alpha3_bundle_and_overnight_rules.md`, then the
run's own report at the end of
`C:\ZND\projects\burnmon\04_assets\2026-09-27_alpha3_overnight_run.log` and its own hub brief
once they exist.
**Supersedes:** the "v0.4.0-alpha.3, later, no date" line in
`C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-26_ws2_merged_workstream_closed.md`
and in `C:\ZND\projects\burnmon\02_roadmap\roadmap.md` item 9.

## 1. Headline

v0.4.0-alpha.3 is in flight: an unattended overnight Claude Code session (Sonnet 5, headless) on
`main` in `C:\ZND\projects\burnmon`, started 2026-09-27 21:53, not finished, nothing committed yet.

## 2. What changed on disk

- **Committed:** nothing yet. `main` is still `f7f1c26` (Wilco's `git log` at about 23:35).
- **Written by this hub chat, untracked** in `C:\ZND\projects\burnmon\02_roadmap\`:
  `2026-09-27_ws2_alpha3_system_cadence_todo_scroll.md` (Wilco's three requests: To Do panel
  scrolls vertically, all 20 core bars equal length with fixed three-character slots, one 1 s
  cadence for the whole System zone like perfadvisor) and
  `2026-09-27_ws2_alpha3_bundle_and_overnight_rules.md` (parked items bundled: minimized RAM,
  To Do due dates in local time, d9 at 1920x1080, w1 solo re-run; plus the unattended rules).
- **Copied by Wilco, untracked:**
  `C:\ZND\projects\burnmon\04_assets\reference\2026-09-27_system_panel\system_panel_core_bars.png`
  (System panel only; Wilco's To Do screenshot was deliberately kept out of the repo).
- **Uncommitted work by the running session, seen at about 23:40 (not reviewed):**
  `C:\ZND\projects\burnmon\cmd\burnmon-dev\page.html` `.todobody` now has `overflow-y:auto`;
  the `processWalkInterval` constant is gone from `cmd\burnmon-dev`; new files
  `C:\ZND\projects\burnmon\04_assets\2026-09-27_alpha3_d_checks.log`,
  `...\2026-09-27_alpha3_w_checks.log`, `...\2026-09-27_alpha3_measure_active.log`,
  `...\2026-09-27_measure.ps1`.
- **Pre-existing, not from this work:** `C:\ZND\projects\burnmon\go.mod` modified before the run
  (`git diff` showed only a line-ending warning); the session was told to leave it out of its
  commits unless its own work needs it.

## 3. What did NOT happen (and why)

- The run has not finished: no report, no commits, no measurements this hub chat has read, no
  review findings. Nothing about alpha.3 is a result yet.
- Nothing pushed or tagged; the run is forbidden to push, tag, merge, rebase or reset.
- The first launch attempt (21:5x) failed at once with an API 400 ("API key is not scoped to a
  workspace"): headless `claude -p` used the machine-wide `ANTHROPIC_API_KEY` variable instead of
  Wilco's Claude login. The second launch removed `ANTHROPIC_API_KEY` for that one PowerShell
  window only, passed a smoke test, then started. The variables themselves were not changed and
  no value was written anywhere.
- The WebView2 memory target (parked item 2) is out of scope for alpha.3.
- Hub `C:\ZND\10_holding\SESSION_LOG.md` not edited by this chat (hub rule: hub changes only; and
  the Cowork shell is down).

## 4. Findings worth propagating

- [STATE] alpha.3 scope: seven items, part 1 items 1 to 3 plus part 2 items 4 to 7, per the two
  specs above. Pulled forward by Wilco on 2026-09-27; it was "later, no date" the day before.
- [STATE] Wilco overrode the decision "no scrollbars except the turn ticker" for one more panel:
  the Microsoft To Do body may scroll vertically. d9 is to allow exactly that exception.
- [STATE] Root cause of the unreadable To Do panel, read from `page.html` (not yet verified in
  the running build): `.todobody` was `overflow:hidden` and `.todorow` could shrink, so rows
  were squeezed into the panel height until they overlapped.
- [STATE] Root cause of unequal core bars, read from `page.html`: the value text beside each
  `.corebar` has its own width, so the flex bar gets a different length per core.
- [STATE] Process groups lagged the rest of the System zone because the process walk ran on its
  own 3 s interval (`processWalkInterval`, `cmd\burnmon-dev\app.go`).
- [RESULT] On this laptop `ANTHROPIC_API_KEY` and `ANTHROPIC_WORKSPACE_ID` are set as environment
  variables (names read from `Get-ChildItem Env:`, values not read). Headless Claude Code picks
  up the key and fails with a workspace error; interactive sessions use the Claude login. Any
  future unattended launch must remove the key for its own process first.
- [RESULT] The run was alive at 21:55 (transcript
  `C:\Users\WilcoDeTree\.claude\projects\C--ZND-projects-burnmon\c1be4a62-1495-4943-9978-1cd41558a703.jsonl`,
  1.7 MB) and was writing check and measurement logs by about 23:40.

## 5. Hub-level decision

Nothing. Project-internal to BurnMon.

## 6. What the next hub read should update

`C:\ZND\10_holding\01_projects\burnmon.md` (alpha.3 in flight, not "later"),
`C:\ZND\10_holding\01_projects\portfolio.md` (BurnMon line) only after the run's own brief lands.
`C:\ZND\projects\burnmon\02_roadmap\roadmap.md` item 9 wording ("later, no date") is stale; the
run's own brief or Wilco should update it. No DEADLINES change. No Mission Deck This Week item
known to be affected.

## 7. Open flags for next session

- Read the run's report at the end of
  `C:\ZND\projects\burnmon\04_assets\2026-09-27_alpha3_overnight_run.log` and check every item
  against the two specs; the run's claims are not results until checked.
- Which real-window checks did not run (screen lock risk overnight).
- Whether the run committed the untracked specs, the reference screenshot and the four logs, and
  whether `go.mod` stayed out.
- The machine-wide `ANTHROPIC_API_KEY`: Wilco's call whether it should stay set.

## 8. Related files

- `C:\ZND\projects\burnmon\02_roadmap\2026-09-26_parked_after_alpha2.md`
- `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-26_ws2_performance_patch.md`
- `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-26_ws2_merged_workstream_closed.md`
- `C:\ZND\10_holding\04_assets\hub_agent_update_2026-09-27_nightly_mirror_ssh_fix.md`
