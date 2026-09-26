# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-26 (hub chat 2026-09-24 to 2026-09-26) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** tell the hub that WS1 shipped, WS2 (BurnMon Dev) is built on a branch, and that a
performance workstream (WS3) plus release order are planned.
**Read order:** this file, `C:\ZND\10_holding\handovers\2026-09-26_burnmon_dev_hub_handover.md`,
`C:\ZND\projects\burnmon\02_roadmap\2026-09-26_session_prompts_phase5_ws3_perf.md`.
**Supersedes:** nothing.

## 1. Headline

WS1 is committed on `main` as v0.3.1; WS2 (a new app, BurnMon Dev, `burnmon-dev.exe`) is
code-complete through phase 4 plus three UI patches on branch `burnmon-dev`, partly pushed and
not rebased; phase 5, WS3 (shared ingest performance) and a WS2 performance patch are planned, not
started.

## 2. What changed on disk

- **Committed** in `C:\ZND\projects\burnmon` on `main`: `1291ef9` "v0.3.1: dev-only mode,
  Sessions harness fix, History stacked chart" (read from the reflog of `main`).
- **On a branch, not merged:** `burnmon-dev` in worktree
  `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev`: BurnMon Dev phases 0 to 4 and three
  UI patches; last commits `493a84d`, `516c24e` as reported by the Claude Code session (not
  re-read live by this hub session).
- **Untracked** (written by this hub session, not committed), all in
  `C:\ZND\projects\burnmon\02_roadmap\`: `2026-09-24_ws1_burnmon_cleanup.md`,
  `2026-09-24_ws2_burnmon_dev.md`, `2026-09-25_ws2_ui_review_patch.md`,
  `2026-09-25_ws2_burn_chart_no_scroll_patch.md`, `2026-09-25_ws2_ticker_headline_patch.md`,
  `2026-09-26_ws3_shared_ingest_performance.md`, `2026-09-26_ws2_performance_patch.md`,
  `2026-09-26_session_prompts_phase5_ws3_perf.md`. Plus
  `C:\ZND\10_holding\handovers\2026-09-26_burnmon_dev_hub_handover.md` and this file.

## 3. What did NOT happen (and why)

- `burnmon-dev` not rebased on v0.3.1, not tagged, not merged: phase 5 does that. Push state:
  hub pass 27 found the UI review patch already on `origin/burnmon-dev`; the no-scroll and
  ticker/headline patches are unpushed. After the rebase the push needs `--force-with-lease`.
- No 10-minute CPU and RAM measurement exists yet for either exe; phase 5 and WS3 produce it.
- WS3 and the WS2 performance patch not started; order agreed with Wilco.
- Microsoft To Do panel in BurnMon Dev: sign-in and list rendering not verified on Wilco's
  account; left for Wilco himself. No token or secret handled in this chat.
- This hub session ran no git writes and no builds.

## 4. Findings worth propagating

- [RESULT] v0.3.1 commit `1291ef9` exists on `main` (reflog read).
- [RESULT] BurnMon Dev startup, reported by the Claude Code session: window 18.67 s to 0.80 s,
  first full render 19.08 s to 1.03 s, by moving scan and backfill after window creation.
- [RESULT] Activity heatmap cost reduced from 3.34 s per minute to about 111 to 167 ms per
  minute by caching closed days (session report).
- [STATE] Both exes reach about 900 MB to 1.3 GB and about 13k handles in the first minutes;
  hub hypothesis (unverified): one fsnotify watch per folder in
  `C:\ZND\projects\burnmon\internal\watch\watch.go`, plus a 400 MB GC soft cap below the live
  heap. WS3 profiles first.
- [STATE] Reviews in the Claude Code sessions caught many real bugs; two items were reported
  done while only half done (headline rolling once a minute, uicheck at half CSS size). Both are
  step 1 and 2 of phase 5.
- [STATE] Plan limits (Claude 5-hour and weekly, Codex, Copilot) parked as a separate BurnMon
  item; idea source github.com/Javis603/token-monitor (MIT).

## 5. Hub-level decision

Nothing. All calls are project-internal to BurnMon.

## 6. What the next hub read should update

`C:\ZND\10_holding\01_projects\burnmon.md` (status: v0.3.1 shipped, BurnMon Dev on branch,
WS3 planned), `C:\ZND\10_holding\01_projects\portfolio.md` (BurnMon line),
`C:\ZND\projects\burnmon\DEADLINES.md` if a date for v0.4.0-alpha.1 is set, and
`C:\ZND\10_holding\SESSION_LOG.md`.

## 7. Open flags for next session

- The eight roadmap briefs are untracked on `main`; Wilco commits them.
- Phase 5 report pending; check it against its step list before handing out the WS3 prompt.
- Plan-limits panel parked, no brief yet.

## 8. Related files

`C:\ZND\projects\burnmon\04_assets\2026-09-24_burnmon_dev_design.md` (on branch `burnmon-dev`),
`C:\ZND\projects\burnmon\04_assets\reference\2026-09-25_ui_review\` (on the branch),
`C:\ZND\10_holding\01_projects\burnmon.md`.
