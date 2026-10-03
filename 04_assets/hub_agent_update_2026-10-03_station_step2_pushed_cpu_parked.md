# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-10-03 (same Claude Code session as the Step 2 brief, written after the push) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** tells the hub that Step 2 is pushed to `origin/main` and that Wilco parked the Station CPU item as "not reproduced, reopen only on a new bug".
**Read order:** this file, then `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-10-03_station_ui_step2_bugs.md`, then `C:\ZND\50_projects\burnmon\04_assets\2026-10-03_station_cpu_measurements.md`.
**Supersedes:** the "uncommitted" and "Wilco's step" lines of `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-10-03_station_ui_step2_bugs.md`; its findings stand.

## 1. Headline
Step 2 is committed as `1b86eec` and pushed: `git ls-remote origin refs/heads/main` and `git rev-parse HEAD` both return `1b86eec6d8b1df63825649783a9ed6812e2488da` (checked 2026-10-03 20:14). No tag was created, no version moved. Wilco decided the Station CPU climb is parked as not reproduced; he will use `burnmon-dev` for a few days and open a new bug if he sees CPU or memory burning.

## 2. What changed on disk
- **Committed and pushed** in `C:\ZND\50_projects\burnmon`: `1b86eec` "Station UI pass step 2: PLAN TABLE label clear of Test Chamber consoles, uicheck skips a capped window, CPU climb measured not reproduced", 12 files, 457 insertions, 15 deletions. Wilco ran the commit and the push; `origin/main` moved from `e42ecc3` to `1b86eec`.
- **Files in that commit:** `C:\ZND\50_projects\burnmon\cmd\burnmon-dev\station\station.js`, `C:\ZND\50_projects\burnmon\tools\station_atlas\preview\check_themes.js`, `C:\ZND\50_projects\burnmon\tools\uicheck\win32.go`, `win32_test.go`, `check_d19.go`, `check_d20.go`, `C:\ZND\50_projects\burnmon\STATUS.md`, `C:\ZND\50_projects\burnmon\SESSION_LOG.md`, and under `C:\ZND\50_projects\burnmon\04_assets\`: the CPU measurements file, `2026-10-03_measure_dev_cpu.ps1`, `2026-10-03_paintcost.ps1`, the Step 2 brief.
- **Untracked:** this brief.

## 3. What did NOT happen (and why)
- No tag and no version bump: `burnmon-dev.exe` still reports v0.4.0-alpha.10.
- The CPU climb was not fixed, only measured. No Station code changed for it.
- STATUS.md and SESSION_LOG.md still say "uncommitted" in the Step 2 entries; they were not edited after the commit.
- No run at 200 percent scaling, no bisect of the creep, d23 flake not explained.

## 4. Findings worth propagating
- [RESULT] Push verified live: `origin/main` equals local `HEAD`, `1b86eec6d8b1df63825649783a9ed6812e2488da`.
- [RESULT] Station CPU, 30 minute runs, five-minute windows (percent of one core, Go plus WebView2): open 22, 17, 13, 14, 18, 32; closed 15, 9, 8, 9, 11, 12. The recorded ramp did not appear. A second session's 3-fake-agent run: Go 3.2 to 3.5 percent per window. Source: `C:\ZND\50_projects\burnmon\04_assets\2026-10-03_station_cpu_measurements.md`.
- [STATE] Decision by Wilco, 2026-10-03: CPU item parked, "not reproduced, not fixed". Trigger to reopen: a new bug from his own daily use of `burnmon-dev`. Date of that review: none set.
- [STATE] A second Claude session started a 60 minute synthetic run at about 19:45 on port 9334; its result does not exist yet in this project's files.

## 5. Hub-level decision (if any)
Nothing. The parking is a BurnMon-internal call.

## 6. What the next hub read should update
`C:\ZND\10_holding\01_projects\burnmon.md` and the portfolio line: Station CPU climb "not reproduced, parked; reopen on a new bug", Step 2 pushed as `1b86eec`. `C:\ZND\50_projects\burnmon\STATUS.md` Step 2 entry still says uncommitted and should say pushed.

Tracker rows moved: none.

## 7. Open flags for next session
- Step 3 (parked items) and Step 4 (plan-limits brief) of `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md` not started.
- d19 and d20 need a screen that holds 2560x1300 CSS px for a full pass.
- d23 failed once of four runs ("0 agents on site"), cause open.
- Wilco's `burnmon-dev.exe` restart: the rebuilt exe contains the Test Chamber console move.

## 8. Related files
`C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`, `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-10-03_station_ui_step1.md`, `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-10-03_station_ui_step2_bugs.md`.
