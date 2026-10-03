# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-10-03 (one Claude Code session on the laptop) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** tells the hub that Step 2 (Station CPU climb, uicheck d19 and d20, PLAN TABLE "E") is done as far as it can be, and that the CPU cause is still open.
**Read order:** this file, then `C:\ZND\50_projects\burnmon\04_assets\2026-10-03_station_cpu_measurements.md`, then the top entry of `C:\ZND\50_projects\burnmon\SESSION_LOG.md`.
**Supersedes:** nothing

## 1. Headline
Step 2 is on disk and uncommitted. d19 and d20 are explained and handled in the check, the PLAN TABLE "E" is fixed, and the Station CPU climb was not reproduced, so no cause is claimed and no Station code changed for it. No version moved.

## 2. What changed on disk
- **Committed:** nothing in this session. `HEAD` is still `e42ecc3`.
- **Uncommitted, all in `C:\ZND\50_projects\burnmon`:**
  - `cmd\burnmon-dev\station\station.js` (Test Chamber consoles from lx 1 and 5 to lx 0 and 6)
  - `tools\station_atlas\preview\check_themes.js` (new check: no room name under another room's sprite box)
  - `tools\uicheck\win32.go` (`capToScreen`, `parseOrigin`, `UICHECK_ORIGIN`, `windowCapped`), `tools\uicheck\win32_test.go` (new), `tools\uicheck\check_d19.go`, `tools\uicheck\check_d20.go` (skip a capped window)
  - `STATUS.md`, `SESSION_LOG.md`
  - `04_assets\2026-10-03_station_cpu_measurements.md`, `04_assets\2026-10-03_measure_dev_cpu.ps1`, `04_assets\2026-10-03_paintcost.ps1`, this brief
- **Outside the repo:** run CSV files and scratch scripts in the session scratch directory only. `%LOCALAPPDATA%\burnmon\burnmon-dev-window.json` was rewritten by the uicheck runs (100,100, 1936x1119, DISPLAY2).

## 3. What did NOT happen (and why)
- No commit, tag or push (Wilco's step).
- No fix for the CPU climb: it did not reproduce, and a guess would break the root-cause-first rule.
- No run at 200 percent scaling (the earlier A/B screen); the laptop panel read dpr 1 today.
- No bisect of the creep (history chart canvas against heatmaps against compositing).
- d23 was not made stable: it failed once with "0 agents on site" and passed three times after.
- `burnmon.exe` and `burnmon-cli.exe` were rebuilt by `build.ps1` but not otherwise touched.

## 4. Findings worth propagating
- [RESULT] Station open, 30 min, five-minute windows of Go plus WebView2 (percent of one core): 22.3, 17.0, 13.0, 13.9, 18.2, 31.7. Closed, same protocol: 15.4, 8.6, 8.3, 9.0, 11.4, 11.9. The recorded ramp (24, 45, 64) did not appear. Source: the Go and msedgewebview2 process CPU seconds sampled every 10 s by the script in `04_assets`.
- [RESULT] CPU follows the page's `requestAnimationFrame` rate (r 0.76 open, 0.70 closed); tokens flowing start `animateNumber` tweens at 40 to 60 calls a second.
- [RESULT] Quiet minutes (page rAF at most 10.5 a second): open 12.9, closed 8.4, so the Station costs about 4.5 points; creep 0.26 points a minute open and 0.22 closed, GPU process when closed; Go flat.
- [RESULT] `paintTick` (fake mode, synthetic rows): 3.3 ms at 5 minutes of rows, 8.5 ms at 60.
- [RESULT] A second session ran 3 fake agents for 30 minutes: Go 3.2 to 3.5 percent in every window, no climb.
- [RESULT] d19 and d20 pass with three monitors; on the laptop panel with `UICHECK_ORIGIN=-3100,-125` the window caps to 1500x900, d19 skips 4 of 5 cases, d20 skips. `go vet ./...`, `go test ./... -count=1` and `.\build.ps1` pass; d19, d20, d24 pass, d23 passes alone.
- [STATE] The creep is inferred to come from `renderHistoryChart` and the heatmaps as the rings fill; not tested.

## 5. Hub-level decision (if any)
Nothing.

## 6. What the next hub read should update
`C:\ZND\10_holding\01_projects\burnmon.md` and the portfolio line: the Station CPU climb is "not reproduced, cause open", not "open bug, climbs with uptime". `C:\ZND\50_projects\burnmon\STATUS.md` header and the BurnMon Dev section already say so.

Tracker rows moved: none.

## 7. Open flags for next session
- Decide whether to chase the CPU on a 200 percent screen or close it as not reproduced.
- Run d19 and d20 on a screen that holds 2560x1300 CSS px for a full pass (they report PARTIAL or SKIPPED on a small one).
- d23 flake (0 agents after d19 and d20).
- Step 3 (parked items) and Step 4 (plan-limits brief) not started.

## 8. Related files
`C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`, `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-10-03_station_ui_step1.md`.
