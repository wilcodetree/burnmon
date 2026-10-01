# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-25 - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** report that the WS2 burn-chart-no-scroll patch (sections 1-5) is code-complete and committed on burnmon-dev, verified against the real running window, not yet released.
**Read order:** this file, then C:\ZND\50_projects\burnmon\02_roadmap\2026-09-25_ws2_burn_chart_no_scroll_patch.md, then SESSION_LOG.md top two entries in the worktree.
**Supersedes:** nothing (first brief on this specific patch)

## 1. Headline
The no-scroll patch (burn chart bars only, one shared colour palette, no
scrollbar anywhere, panel-drop on small windows, one shared 2s render tick)
is code-complete and committed on branch burnmon-dev, verified with go vet,
go test, a real build, and a real-window uicheck battery (d0-d11, all
green). Not merged to main, not pushed, not tagged, version deliberately
unchanged at 0.4.0-alpha.1.

## 2. What changed on disk
- **Committed** in the burnmon repo (worktree
  C:\ZND\50_projects\burnmon\.claude\worktrees\burnmon-dev, branch
  burnmon-dev): `51a1f72` "v0.4 burn chart no-scroll patch, sections 1-5" -
  cmd\burnmon-dev\page.html, cmd\burnmon-dev\app.go, cmd\burnmon-dev\main.go,
  tools\uicheck\check_d8.go/check_d9.go/check_d10.go/check_d11.go, and the
  two reference screenshots
  04_assets\reference\2026-09-25_ui_review\9_burn_chart_lines.png and
  10_scrollbars.png.
- **Committed**, same branch: `d0c18d8` "SESSION_LOG: v0.4 burn chart
  no-scroll patch (sections 1-5) entry" - SESSION_LOG.md only.
- **Files touched** (full paths, all inside the worktree above):
  C:\ZND\50_projects\burnmon\.claude\worktrees\burnmon-dev\cmd\burnmon-dev\page.html,
  \cmd\burnmon-dev\app.go, \cmd\burnmon-dev\main.go,
  \tools\uicheck\check_d8.go, \check_d9.go, \check_d10.go, \check_d11.go,
  \SESSION_LOG.md.
- **Untracked, left alone on purpose** (pre-existing, not from this
  session): C:\ZND\50_projects\burnmon\.claude\worktrees\burnmon-dev\04_assets\reference\2026-09-25_ui_review\5_topbar_target.jpg,
  6_ticker_target.png, 7_perfadvisor_system.png, 8_current.jpg - four
  reference images from an earlier, separate session that were never
  committed there either; out of this patch scope, not touched.

## 3. What did NOT happen (and why)
- Not merged to main, not pushed to any remote, not tagged - per house
  process and explicit instruction for this session.
- Version stays 0.4.0-alpha.1 everywhere it lives (cmd\burnmon-dev\app.go,
  winres\burnmon-dev.json) - the version bump is scoped to phase 5 (a
  separate, later session), not this patch, on explicit instruction.
- Phase 5 itself (rebase on main, 10-minute own-CPU/RAM measurement, final
  review, version bump, release commands, hub brief for the release) has
  not run. This brief is not that one.
- No fresh burn-in / soak test beyond the uicheck battery and a roughly
  90-second CPU sample before/after the refresh-tick change (see section
  4); no multi-hour real-usage observation yet.
- The known, separate, pre-existing shared live-watch/store memory and
  handle-growth issue (burnmon.exe and burnmon-dev.exe both climbing to
  roughly 900MB-1GB) was not touched; it is explicitly out of this
  branch scope per the original WS2 brief and gets its own session on
  main.

## 4. Findings worth propagating
- [RESULT] All five required uicheck cases pass against the real running
  window: d0 (header), d1 (export), d8/d4/d2/d5/d6 (the five viewports:
  1024x768, 1280x860, 1152x2048, 1920x1080, 2560x1440), d9 (no visible
  scrollbar anywhere, checked element by element, not just at the page
  level, at all five sizes), d10 (no marker/line elements in the burn
  chart), d11 (30 seconds of real paint-timestamp logging: 15 complete
  render ticks, every panel paint lands within the same tick, cadence
  holds around the configured 2000ms).
- [RESULT] go vet ./..., go test ./... -count=1 (every package) and
  .\build.ps1 all green after every edit pass, including after two
  independent Opus code reviews fixes.
- [RESULT] Startup timing unaffected by the new background-refresh
  architecture: window creation measured at 702-752ms and first full
  render at 939-973ms across several real runs this session, matching the
  fast-launch baseline SESSION_LOG already established (this had to be
  re-verified because a review-caught bug briefly reintroduced the old
  slow-startup regression before it was fixed; see section 7).
- [RESULT] CPU comparison, same real store, roughly 90 second
  steady-state samples after a 75-second warmup, this laptop (20 logical
  cores): before the refresh-tick patch, average 1.86% of all cores
  (about 37% of one core); after, average 1.89% of all cores (about 38%
  of one core). No measurable regression or improvement either way;
  working set held around 930-932MB in both builds (the known
  pre-existing memory issue noted above, unrelated to this patch).
- [STATE] Which panels are visible at the app own 1280x860 default
  launch size, on this session own display: header, burn chart (bars,
  legend, session cards) and the system CPU box always show; vendor
  strip, turn ticker, the activity/harness-heatmap row, process groups
  and memory/disk/network get dropped or clipped. Confirmed by direct DOM
  inspection this is a genuine space constraint (both droppable panels
  are correctly dropped by the fixed order; the "always stay" panels
  alone still do not fit) rather than a logic bug, and specific to this
  session own display, which reports roughly 2x DPI scaling for a
  nominal 1280x860 window (real CSS pixels come out around 627x395). This
  has not been checked against Wilco own real hardware; flagged as
  pending, not asserted as representative.

## 5. Hub-level decision (if any)
Nothing. This is a project-internal implementation patch, not a
cross-project, pricing, positioning, park/unpark or CIPHER-wall call.

## 6. What the next hub read should update
- C:\ZND\10_holding\01_projects\burnmon.md (the project one-pager): note
  that the no-scroll patch (sections 1-5) landed on burnmon-dev, phase 5
  (release) still pending.
- C:\ZND\50_projects\burnmon\02_roadmap\2026-09-25_ws2_burn_chart_no_scroll_patch.md
  itself has no status field to update; SESSION_LOG.md in the worktree is
  the record of record for this patch own history.
- No This Week / Mission Deck item name was given for this session, so
  section 6 own done/in_progress/blocked line does not apply here; if
  BurnMon WS2 is tracked as a named deck item, the hub reader should match
  it against this brief rather than guessing an item ID here.

## 7. Open flags for next session
- Phase 5 has not run: rebase burnmon-dev on main, a 10-minute own-CPU/RAM
  measurement (longer and more formal than this session 90-second
  sample), a final review, the version bump (to whatever phase 5 decides,
  not touched here), release commands handed to Wilco, and a hub brief for
  the release itself (separate from this one).
- Whether the panel-drop behaviour at small/DPI-scaled windows (section 4)
  needs a follow-up depends on what Wilco actually sees on his own two
  screens; this session own display is very likely not representative
  (roughly 2x DPI scaling measured indirectly via CSS-pixel-to-physical-
  pixel ratios across five different requested sizes).
- Two independent Opus code reviews ran mid-session (sections 1-4, then
  section 5) and both caught real bugs, all fixed and re-verified before
  commit; details are in SESSION_LOG.md own entry, not repeated here to
  avoid duplicating that record.

## 8. Related files
- C:\ZND\50_projects\burnmon\02_roadmap\2026-09-25_ws2_burn_chart_no_scroll_patch.md
  (the spec this patch follows)
- C:\ZND\50_projects\burnmon\.claude\worktrees\burnmon-dev\SESSION_LOG.md (top
  entry, full narrative of what changed and what the two reviews caught)
- C:\ZND\50_projects\burnmon\04_assets\2026-09-24_burnmon_dev_design.md
  (the underlying BurnMon Dev design doc this patch sections build on)
- C:\ZND\50_projects\burnmon\04_assets\reference\2026-09-25_ui_review\ (in
  the worktree at ...\.claude\worktrees\burnmon-dev\04_assets\reference\
  2026-09-25_ui_review\ - the two committed screenshots plus four
  pre-existing, still-untracked ones)
