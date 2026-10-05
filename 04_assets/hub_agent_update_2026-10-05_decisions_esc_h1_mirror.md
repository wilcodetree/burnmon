# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-10-05 (Cowork planning session, no code written by this session) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** Records four small calls Wilco made on 2026-10-05 and one open question, so the hub does not rediscover them from the chat.
**Read order:** this file, then `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md` (section "Decisions 2026-10-05"), then `C:\ZND\50_projects\burnmon\02_roadmap\2026-09-22_burnmon_plan.md` lines 23 and 33.
**Supersedes:** nothing

## 1. Headline
Decided: Esc closes every Station mode (O included); H1 is measured through Wilco's own use plus public GitHub signals; `mirror` is a private backup of all repos on the cipher box. The H1 window dates and thresholds are still OPEN. Decisions are recorded in a file that is not committed yet.

## 2. What changed on disk
- **Committed earlier, same planning thread:** `2908574` (DEADLINES.md: the 2026-10-24 Valona pilot line removed), `e0bd629` (Valona pilot struck from the plan, H1 measure OPEN).
- **Files touched, NOT committed (checked against HEAD with git show):** `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`. The section "Decisions 2026-10-05" and the item F text (hover card) exist only in the working copy; HEAD has neither.
- **Not touched:** `C:\ZND\50_projects\burnmon\02_roadmap\2026-09-22_burnmon_plan.md`. Its H1 row (line 33) still says the measure is set at the 2026-12-19 scoring.

## 3. What did NOT happen (and why)
- Nothing in this brief is committed, tagged or pushed. Commits are Wilco's step.
- The H1 row in the plan file was not rewritten, because the window and thresholds are not decided.
- `C:\ZND\10_holding\03_logs\decisions.md` was not edited: this session cannot reach the hub folder.
- The state of the `mirror` remote was not checked after 2026-10-05. Last seen `mirror/main` at `9c8d0f4`, well behind `origin/main`.

## 4. Findings worth propagating
- [STATE] Esc closes every Station mode, O included. Shipped in `5a93808`. Wilco confirmed keeping it.
- [STATE] H1 measure without Valona: Wilco's own daily use plus public ZND GitHub signals (stars, release downloads, issues) counted over a fixed window before the 2026-12-19 decision. No direct developer outreach.
- [STATE] `mirror` is a backup of all repos on the cipher box (Wilco's words). Behind `origin` is expected.
- [STATE] The plan's line 33 mentions "Talon" (decision 2026-09-29) rescoring H1. This session does not know what Talon is. The two statements may overlap; the hub should reconcile them, not this brief.

## 5. Hub-level decision (if any)
  **Decision:** H1 for BurnMon is scored on Wilco's own use plus public GitHub signals, not on a developer pilot.
  **Context:** No Valona pilot exists. BurnMon is a ZND product, handed out through the public ZND GitHub.
  **Alternatives considered:** own use only (says nothing about other developers); direct outreach to 3 to 5 outside developers (needs a message sent as ZeroNonsense.dev, Wilco's step).
  **Consequences:** The 2026-12-19 scoring needs a window start date and a yes threshold. OPEN: window start and thresholds (waits on Wilco).
  **Links:** `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`, `C:\ZND\50_projects\burnmon\02_roadmap\2026-09-22_burnmon_plan.md`

## 6. What the next hub read should update
- `C:\ZND\10_holding\03_logs\decisions.md`: the block in section 5, after Wilco confirms it.
- `C:\ZND\10_holding\01_projects\burnmon.md`: H1 measure line.
- `C:\ZND\50_projects\burnmon\DEADLINES.md`: no change; the Valona line is already struck.
Tracker rows moved: none.
This brief names no Mission Deck This Week item.

## 7. Open flags for next session
- OPEN: H1 window start and the thresholds that count as a yes (waits on Wilco).
- Commit `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md` (decisions section and item F).
- Reconcile "Talon" in plan line 33 with the H1 decision above.
- Step 3 (parked items and known gaps) and step 4 (plan-limits panel, brief first) are not started.

## 8. Related files
- `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-10-05_station_hover_card.md`
- `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-10-05_station_click_card_i_key_cowork_check.md`
- `C:\ZND\50_projects\burnmon\02_roadmap\2026-09-26_parked_after_alpha2.md`
