# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-26 - **Owner:** Wilco de Tree
**Project:** BurnMon (WS2, BurnMon Dev)
**Purpose:** WS2's phase 5 (verify and release) finished on branch burnmon-dev: fixes, a
full verify pass, two 10-minute perf measurements, a fresh independent review and its one
fix, docs. Code-complete on the branch, not merged, not pushed, not tagged.
**Read order:** this file, SESSION_LOG.md's newest entry (repo C:\ZND\projects\burnmon,
worktree C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev), STATUS.md's "BurnMon Dev"
section, README.md's "BurnMon Dev" section.
**Supersedes:** nothing (first phase 5 brief).

## 1. Headline
BurnMon Dev (burnmon-dev.exe, v0.4.0-alpha.1) is code-complete and verified on branch
burnmon-dev: five real bugs fixed (headline staleness, uicheck DPI sizing, a process-CPU
unit confusion, and a headline dip a fresh review caught), sampling cadences split, and
refresh_ms's own default flipped from 2000 to 1000 based on a measured comparison. Not
merged, not pushed, not tagged; Wilco has the exact release commands to run himself.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (branch `burnmon-dev`, worktree
  `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev`):
  - `2e0f91c` v0.4 phase 5 fix 1-2: headline per-tick, uicheck DPI-aware sizing
  - `be5dce5` v0.4 header tweak: center the headline total between title and stats
  - `935b2a0` 04_assets: commit the UI review patch's reference screenshots
  - `f1ca1de` v0.4 phase 5 fix 5b: split sampling cadences, fix process-groups CPU unit
  - `0bbaedd` v0.4 phase 5 fix: headline monotonic floor after fresh Opus review
  - `0089baf` SESSION_LOG: v0.4 phase 5 verify and release entry
  - Branch also rebased onto `main`'s `v0.3.1` (`1291ef9`, WS1's cleanup) during this
    session; one conflict in SESSION_LOG.md, resolved keeping both entries.
- **On a branch, not merged:** `burnmon-dev` carries all of the above plus every earlier
  WS2 phase (0 through 4) and three UI patches from prior sessions. `main` is unaffected.
- **Files touched** (full paths, this session only): `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev\cmd\burnmon-dev\headline.go`
  (new), `...\cmd\burnmon-dev\headline_test.go` (new), `...\cmd\burnmon-dev\app.go`,
  `...\cmd\burnmon-dev\main.go`, `...\cmd\burnmon-dev\page.html`,
  `...\tools\uicheck\win32.go`, `...\tools\uicheck\check_d11.go`, `...\README.md`,
  `...\STATUS.md`, `...\SESSION_LOG.md`, and the four reference screenshots under
  `...\04_assets\reference\2026-09-25_ui_review\`.
- **Untracked / outside a repo:** none left; the four reference screenshots were
  untracked at session start and are now committed.

## 3. What did NOT happen (and why)
Not merged to main, not pushed to origin, not tagged v0.4.0-alpha.1: house process for
this workstream stops here and hands Wilco the exact PowerShell commands to run himself
(force-with-lease push, since origin/burnmon-dev still holds pre-rebase commits). The
known shared-ingest memory/handle climb (both exes toward roughly 900 MB and 13k handles
in their first minute) was reproduced again but deliberately not fixed: it lives in code
shared with burnmon.exe/main, out of this branch's scope (WS3). Microsoft To Do sign-in
was not exercised live this session (needs Wilco's own Microsoft account); the panel
stays off by default until he opts in and signs in himself, steps below. The w-suite's
`w1` check failed once under concurrent build/test load and passed clean on an immediate
retry; this is the same known, pre-existing first-paint-budget flake earlier BurnMon
sessions already documented (not a regression from this session's work).

## 4. Findings worth propagating
- [RESULT] Headline total now updates most render ticks instead of once a minute
  (cmd\burnmon-dev\headline.go), verified by five unit tests plus a real-world case this
  session: the sandbox's clock crossed UTC midnight mid-session and the header correctly
  dropped from 534,790,202 to 12,700,392 tokens (the new day's own true total, not a bug).
- [RESULT] tools\uicheck now sizes test windows in real CSS pixels (GetDpiForWindow/
  AdjustWindowRectExForDpi/MonitorFromWindow), replacing a prior silent bug where a
  requested "1280x860" landed at roughly half that in CSS pixels on a 200%-scaled
  display. Verified: d4 (1280x860 target) now reports an exact 1280x860 CSS match on this
  sandbox's screen; larger targets that do not fit are capped and logged instead of
  silently shrinking.
- [RESULT] 10-minute measurement, burnmon-dev.exe with burnmon.exe co-running, refresh_ms
  2000 (the then-current default): 2.22 percent average whole-machine CPU, 19.14 percent
  peak, 937 MB peak RAM, 13,940 peak handles.
- [RESULT] Same 10-minute measurement at refresh_ms 1000, after splitting sampling into
  three independent cadences (paint/cheap system sample on refresh_ms, the process walk
  fixed at 3s, persistence fixed at 10s wall clock): 1.89 percent average whole-machine
  CPU, 16.96 percent peak, 937.5 MB peak RAM, 13,774 peak handles. Under the 2 percent bar
  and not worse than the 2000 run, so refresh_ms's own coded default flips to 1000.
- [RESULT] Peak RAM/handles in both runs match the design doc's own already-documented,
  out-of-scope shared-ingest climb; unaffected by anything changed this session.
- [RESULT] Process-groups panel's CPU percent was gopsutil's own unnormalized
  per-process convention (100 percent = one full logical core, so a multi-threaded
  process could read past 100 percent), inconsistent with every other "%" on the page
  (whole-machine normalized). Fixed by dividing by the live core count before display;
  confirmed by screenshot ("BurnMon Dev" process row now reads a sane 3 percent instead of
  the raw, ambiguous 47 percent Wilco flagged).
- [RESULT] A fresh, independent read-only review over the full main..burnmon-dev diff
  (74 files, about 10.5k insertions) found no security issues and confirmed the sampling
  refactor's locking as correct; one real gap (a headline dip possible if vendor_strip's
  cache stalled past its 30-minute event window) was fixed with a per-day monotonic floor,
  covered by two new tests.
- [STATE] Microsoft To Do panel: built, off by default, sign-in not yet exercised live.
  Pending Wilco's own device-code sign-in once he opts it on.

## 5. Hub-level decision (if any)
Nothing. This is a project-internal verify/release pass, not a cross-project, pricing,
positioning, park/unpark or CIPHER-wall decision.

## 6. What the next hub read should update
`C:\ZND\10_holding\01_projects\burnmon.md` (the hub one-pager) should reflect that WS2
(BurnMon Dev) is code-complete on branch burnmon-dev at v0.4.0-alpha.1, pending Wilco's
own push/tag/merge decision. `C:\ZND\10_holding\02_roadmap\roadmap.md` if it tracks WS2 as
an open item. No DEADLINES.md entry expected (no dated gate tied to this).

## 7. Open flags for next session
- Push, tag and merge are Wilco's own manual steps (commands below); nothing here expires
  or blocks other work in the meantime.
- WS3 (the shared live-watch/store memory and handle climb, both burnmon.exe and
  burnmon-dev.exe) remains a separate, not-yet-scheduled workstream.
- Microsoft To Do: sign in once, live, to confirm the device-code flow end to end on this
  machine (steps below).
- The odd stray WebView2 profile folder this session found and removed from the worktree
  root (a garbled directory name, contents just an EBWebView cache folder) suggests some
  other process on this machine may have launched a WebView2 host with a corrupted
  user-data-dir argument at some point; not this branch's own code, not investigated
  further, flagging in case it recurs.

## 8. Related files
- Plan: `C:\ZND\projects\burnmon\02_roadmap\2026-09-24_ws2_burnmon_dev.md`
- Patches: `C:\ZND\projects\burnmon\02_roadmap\2026-09-25_ws2_ui_review_patch.md`,
  `..._burn_chart_no_scroll_patch.md`, `..._ticker_headline_patch.md`
- Design: `C:\ZND\projects\burnmon\04_assets\2026-09-24_burnmon_dev_design.md`
- Hub one-pager: `C:\ZND\10_holding\01_projects\burnmon.md`
