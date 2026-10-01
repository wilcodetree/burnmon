# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-27/28 (unattended overnight Claude Code session, Sonnet 5, spanned past
midnight) - **Owner:** Wilco de Tree
**Project:** BurnMon (WS2, BurnMon Dev, v0.4.0-alpha.3)
**Purpose:** WS2 alpha.3 (7 bundled items: To Do panel scroll, equal core bars, one 1s
System-zone cadence, minimized RAM, To Do due dates in local time, d9 at 1920x1080, w1
solo re-run) is committed on main, with a real gap a fresh review found and fixed before
commit, one real-window item confirmed via direct evidence rather than assumed clean.
**Read order:** this file, C:\ZND\50_projects\burnmon\SESSION_LOG.md's top entry
(2026-09-27, "WS2 alpha.3 patch, v0.4.0-alpha.3"), STATUS.md's "BurnMon Dev" section.
**Supersedes:** C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-09-27_ws2_alpha3_overnight_launched.md
(the mid-flight "launched, not finished" brief written from the hub chat at 23:40 while
this run was still in progress) and the "v0.4.0-alpha.3, later, no date" line it names in
C:\ZND\50_projects\burnmon\02_roadmap\roadmap.md item 9 and
C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-09-26_ws2_merged_workstream_closed.md.

## 1. Headline

BurnMon Dev is v0.4.0-alpha.3, committed directly on main (branch burnmon-dev was
already merged at f7f1c26 before this session started): the To Do panel scrolls instead
of cropping, all 20 core bars share one exact width (proved live), the whole System zone
(including the main chart and process-groups sparklines, after a review-found fix) now
moves on one true 1s beat, the process snapshot buffer is reused, To Do due dates convert
through Microsoft Graph's own time zone, and the d9 scrollbar check passes at all five
sizes. Cost guard held (Go process 0.21 percent avg CPU / 113.5 MB peak RAM active, 0.06
percent avg CPU / 113.5 MB peak RAM minimized, both settled and re-verified after the
review fix). Not pushed, tagged, merged, rebased or reset.

## 2. What changed on disk

- Committed in C:\ZND\50_projects\burnmon (branch main): one commit, WS2 alpha.3
  (v0.4.0-alpha.3), on top of f7f1c26. Exact SHA: see this session's own git log after
  the commit lands (this brief is written just before that commit, per the session's own
  order: code, verify, real-window checks, review, fix, then commit).
- Files touched (full paths, this session):
  C:\ZND\50_projects\burnmon\cmd\burnmon-dev\app.go,
  ...\cmd\burnmon-dev\main.go, ...\cmd\burnmon-dev\page.html,
  ...\internal\sysmon\process_windows.go, ...\internal\sysmon\process_windows_test.go,
  ...\internal\todo\todo.go, ...\internal\todo\todo_test.go (new),
  ...\tools\uicheck\check_d9.go, ...\tools\uicheck\check_d11.go,
  ...\tools\uicheck\check_d13.go (new), ...\winres\burnmon-dev.json, ...\README.md,
  ...\STATUS.md, ...\SESSION_LOG.md, plus the two spec docs this session read from
  (...\02_roadmap\2026-09-27_ws2_alpha3_system_cadence_todo_scroll.md,
  ...\02_roadmap\2026-09-27_ws2_alpha3_bundle_and_overnight_rules.md, both committed
  alongside the code) and the reference screenshot
  (...\04_assets\reference\2026-09-27_system_panel\system_panel_core_bars.png, committed
  per the session's own instructions).
- Left out of the commit, deliberately: C:\ZND\50_projects\burnmon\go.mod - a
  pre-existing uncommitted change from before this session (confirmed a line-ending
  difference only, no real content diff), not needed by this session's own work, left for
  Wilco per the session's own instructions.
- Untracked, not committed (evidence, not durable artifacts; their numbers are folded
  into STATUS.md/SESSION_LOG.md's own prose instead): six .log files under
  C:\ZND\50_projects\burnmon\04_assets\2026-09-27_alpha3_*.log (the d0-d13 sweep, the w-check
  sweep, the w1-solo attempt, and four measurement runs, two superseded by a later,
  settled re-run each). The three other pre-existing hub_agent_update_2026-09-25 briefs
  and the 2026-09-26 ws2_merged_workstream_closed brief already sitting untracked in
  04_assets before this session started are also left alone: out of this session's own
  scope, presumably awaiting a separate hub-update ingestion pass.

## 3. What did NOT happen (and why)

- Not pushed, tagged, merged, rebased or reset. House process for this workstream stops
  at the commit; PowerShell push command in section 8 below.
- The full stretch of desktop-idle real-window work this task's own rules anticipated
  did not happen uninterrupted. Twice during this session the desktop showed clear signs
  of a live human or agent actively using it in parallel (not this session's own doing):
  once a foreign foreground window ("PermitCompanyMismatchError") appeared at the same
  moment burnmon.exe/burnmon-dev.exe had been silently replaced by fresh, differently
  PID'd instances (interrupting a 5-minute minimized measurement at 20s); later, a w1
  solo re-run's own saved screenshot showed an unrelated, actively-used Cowork chat
  window in the foreground instead of burnmon.exe. Per this task's own overnight rules
  ("stop the real-window part on a foreign foreground window, never report a check as
  passed that did not run"), the session stopped each time rather than forcing a retry
  against what looked like someone else's active session, and later successfully re-ran
  the interrupted minimized measurement once the desktop looked clear again (it did, and
  completed clean); w1 was attempted once more after that and failed for the same
  confirmed-live-contention reason a second time, and was not forced further.
- w1 (solo, truly idle desktop) is not confirmed clean this session, see above. This
  matches the same environmental-contention class the 2026-09-26 alpha.2 session's own
  STATUS.md entry already documents for w1 (this exe's startup-timing check is sensitive
  to desktop contention it does not itself control); not a code regression, but not a
  clean pass either. A future session should re-run w1 alone when nobody else is known
  to be on the machine.
- No live Microsoft Graph read confirmed item 5's own timezone assumption. No cached
  Microsoft To Do sign-in token exists on this machine, and an unattended overnight
  session cannot complete an interactive device-code sign-in itself. The fix (converting
  dueDateTime.timeZone before comparing dates, instead of a bare UTC-string substring) is
  covered by unit tests only; a future session with a live, signed-in To Do panel should
  confirm Graph's real response shape matches what the code assumes.

## 4. Findings worth propagating

- [RESULT] 10-minute active measurement (burnmon.exe co-running), pre-review-fix build:
  Go process 0.21 percent avg / 0.37 percent peak CPU (alpha.2 baseline: 0.28/0.59), 111.5
  MB avg / 113.5 MB peak RAM (alpha.2: 195.4 MB peak); WebView2 tree 0.45 percent avg /
  1.26 percent peak CPU, 489.6 MB avg / 514.3 MB peak RAM, 6 processes peak (alpha.2:
  0.51/1.36, 671.1 MB peak). Well under the 2 percent/250MB cost guard despite the
  process walk now running three times as often (1s instead of 3s), confirming item 3's
  own prediction that the syscall-per-walk cost, not the walk frequency, dominates.
- [RESULT] 5-minute minimized measurement, on the review-fixed build, from an
  already-settled process: Go process 0.06 percent avg / 0.19 percent peak CPU, 111.5 MB
  avg / 113.5 MB peak RAM; WebView2 tree 0.00 percent avg / 0.04 percent peak CPU, 384.2
  MB avg / 387.0 MB peak RAM. No growth versus the active figures above; item 4's buffer
  reuse holds under the new 1s cadence. (A shorter spot-check taken immediately after a
  fresh launch briefly read RAM near 263 MB in both active and minimized phases before
  settling back to 109 MB once the one-time startup backfill finished; flagged as a
  measurement artifact, not a real number, in STATUS.md/SESSION_LOG.md, not reported as
  the headline figure.)
- [RESULT] Live proof of the shared 1s beat (check_d11.go, extended): over 30 seconds,
  both the system sample and the process-groups walk produced 30 fresh samples each,
  equal, not just within tolerance.
- [RESULT] Live proof of equal core bars (check_d13.go, new): all 20 .corebar elements
  measured 195px wide, min equals max.
- [RESULT] uicheck d0 through d13 (the full five-size sweep, including the new d13) and
  w0, w2 through w8 all passed clean against the rebuilt exes. go vet ./..., go test
  ./... -count=1 (every package), .\build.ps1, and node --check on both page.html and
  template.html's extracted script blocks all passed, both before and after the review
  fix.
- [RESULT] A fresh, independent Opus review (read-only, over the full diff) found one
  real gap before commit: the item 3 doc comment and an earlier README draft both
  claimed the main chart and process-groups sparklines/harness heatmap now moved on the
  same 1s beat as the rest of the System zone, but they were still reading from
  burnmon-dev.db, which only gains a new row every 10-second persistInterval (unchanged
  by item 3), so those two panels still moved in 10-second steps in practice,
  contradicting the patch's own claim. Fixed, not just reworded: two new in-memory ring
  buffers (cmd\burnmon-dev\app.go's sysHistBuf/groupsHistBuf) now back both fields
  directly at the true 1s resolution, appended by the same merged sampling tick that
  already updates a.latest/a.latestGroups; persistInterval and the store's own write
  volume are unchanged. The review also found a weak DST test (fixed: the original
  instant was 1.5 hours before the actual transition, so it never really exercised the
  post-transition offset) and one inaccurate code comment (fixed: check_d11.go's own
  comment overclaimed "real value changes" where the counter actually counts a new
  sample landing; the assertion itself was already correct, only the explanation was
  wrong). Everything else the review checked (the merged sampling goroutine's own
  locking, the process-snapshot buffer's aliasing safety across ticks, the CSS ch-unit
  sizing, the new checks' own comparison logic) came back clean. Full text of the review
  lives in this session's own transcript, not duplicated here.
- [STATE] internal\todo\todo.go's due-date fix is code-complete and unit-tested but not
  live-confirmed, see section 3.

## 5. Hub-level decision (if any)

Nothing. This is a project-internal UI/performance patch and its own review-fix pass,
not a cross-project, pricing, positioning, park/unpark or CIPHER-wall decision.

## 6. What the next hub read should update

C:\ZND\10_holding\01_projects\burnmon.md (the hub one-pager) should reflect that alpha.3
is committed on main, not "later, no date" (the exact stale line the mid-flight
"overnight_launched" brief already flagged in roadmap.md item 9 and the
ws2_merged_workstream_closed brief). No DEADLINES.md entry expected (no dated gate tied
to this patch).

## 7. Open flags for next session

- Push is Wilco's own manual step (command below); nothing here expires or blocks other
  work in the meantime.
- w1 (burnmon.exe's own startup-timing uicheck) should be re-run alone once nobody else
  is known to be using this machine; twice this session it captured direct evidence of
  another active session on the same desktop, not a code regression.
- The Microsoft To Do panel's own due-date fix (item 5) needs a live, signed-in session
  to confirm Graph's real dueDateTime.timeZone shape matches what localDueDate assumes
  (UTC by default, an unrecognized zone name conservatively falls back to UTC too).
- A pre-existing, unrelated stale comment on internal\sysmon\process_windows.go's
  decodeUnicodeString (mentions "the bounds check below", which does not exist) was
  flagged by this session's own review but left alone, out of scope for this patch; a
  future session touching that file could fix it.
- The three pre-existing untracked hub_agent_update_2026-09-25 briefs and the
  2026-09-26_ws2_merged_workstream_closed brief in 04_assets (present before this
  session started) still await a hub-update ingestion pass; not touched by this session.

## 8. Related files

- Specs: C:\ZND\50_projects\burnmon\02_roadmap\2026-09-27_ws2_alpha3_system_cadence_todo_scroll.md,
  ...\2026-09-27_ws2_alpha3_bundle_and_overnight_rules.md
- Superseded: C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-09-27_ws2_alpha3_overnight_launched.md
- Prior brief: C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-09-26_ws2_performance_patch.md
- Reference screenshot: C:\ZND\50_projects\burnmon\04_assets\reference\2026-09-27_system_panel\system_panel_core_bars.png
- Hub one-pager: C:\ZND\10_holding\01_projects\burnmon.md

## PowerShell commands for Wilco

Push main once you are happy with the commit. Run in PowerShell, at
C:\ZND\50_projects\burnmon:

    cd C:\ZND\50_projects\burnmon
    git status --short
    git log --oneline -5
    git push origin main

No tag suggested for an alpha; tag only if you want one, same commands as prior alpha
sessions (git tag v0.4.0-alpha.3 then git push origin v0.4.0-alpha.3).
