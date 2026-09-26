# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-26 - **Owner:** Wilco de Tree
**Project:** BurnMon (WS2, BurnMon Dev performance patch)
**Purpose:** WS2's own performance patch (items 1-6) finished on branch burnmon-dev, after
rebasing onto main's v0.3.2 (WS3): cheaper process sampling, pause-when-hidden, no
per-tick DOM rebuilds, an honest self row, local time in every remaining UTC spot in
cmd/burnmon-dev, and measured headline-change evidence. A fresh Opus review found five
real issues before commit, all fixed. Committed on burnmon-dev, not pushed, not tagged,
not merged.
**Read order:** this file, SESSION_LOG.md newest entry (repo C:\ZND\projects\burnmon,
worktree C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev), STATUS.md BurnMon Dev
section and its two new Known gaps entries.
**Supersedes:** nothing (first WS2-performance-patch brief; the WS2 phase 5 brief and the
WS3 brief it builds on stay valid for their own earlier state).

## 1. Headline
BurnMon Dev is v0.4.0-alpha.2 on branch burnmon-dev, committed (c236162), rebased onto
main's v0.3.2: post-patch, 10-minute measurement shows the Go process at 0.28 percent
average CPU (down from 0.89 percent pre-patch) and 195.4 MB peak RAM (under the 250 MB
target, down from 252.5 MB). Not merged, not pushed, not tagged; Wilco has the exact
commands below.

## 2. What changed on disk
- Committed in C:\ZND\projects\burnmon (branch burnmon-dev, worktree
  C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev):
  - c236162 v0.4 WS2 performance patch, v0.4.0-alpha.2 (21 files, 1613 insertions,
    297 deletions - items 1-6 plus the five review fixes, one commit).
  - The branch was also rebased onto main's 69a071e (v0.3.2) in this same session:
    22 commits replayed, one conflict (SESSION_LOG.md, both branches prepend entries,
    four separate conflict points), resolved keeping every entry in newest-on-top order
    by real commit timestamp; no other file conflicted.
- On a branch, not merged: burnmon-dev carries this commit plus every earlier WS2
  phase (0 through 5) and prior UI patches. main is unaffected.
- Files touched (full paths, this session): C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev\cmd\burnmon-dev\app.go,
  ...\main.go, ...\headline.go, ...\headline_test.go, ...\export_run.go,
  ...\export_run_test.go (new), ...\page.html,
  ...\internal\sysmon\process.go (trimmed to shared code only),
  ...\internal\sysmon\process_windows.go (new), ...\internal\sysmon\process_other.go
  (new), ...\internal\sysmon\process_windows_test.go (new),
  ...\internal\sysmon\harness.go, ...\internal\sysmon\harness_test.go,
  ...\internal\advisor\advisor.go, ...\internal\advisor\advisor_test.go,
  ...\internal\devexport\devexport.go, ...\internal\devexport\devexport_test.go,
  ...\winres\burnmon-dev.json, ...\README.md, ...\STATUS.md, ...\SESSION_LOG.md.
- Untracked / outside a repo: a stray, garbled-named WebView2 profile cache folder
  reappeared in the worktree root during this session's own repeated test launches (the
  same harmless pattern the WS3 session already found and removed once); removed again.
  Nothing else left loose.

## 3. What did NOT happen (and why)
Not merged, not pushed, not tagged: house process for this workstream stops at the commit
and hands Wilco the exact PowerShell commands below (force-with-lease, since
origin/burnmon-dev still holds the pre-rebase commits). WebView2's own memory usage
target was not set to "Low" while minimized: ICoreWebView2Controller4 is not vendored in
go-webview2, and hand-deriving its COM vtable layout without the official WebView2 SDK
header risks a wrong method-slot offset, a real crash risk in a tool Wilco runs daily, for
a soft (RAM-only) gain; flagged in STATUS.md rather than attempted blind. The Microsoft To
Do panel's own due-date timezone handling (named under item 5's own "To Do" in the spec)
was not touched: correctness there depends on Microsoft Graph's own undocumented response
timezone behaviour, unverifiable without a live, signed-in To Do panel this sandbox does
not have; flagged in STATUS.md rather than guessed at. d9 (the no-scrollbar sweep)
failed at one specific viewport (1920x1080) on a pre-existing CSS characteristic
(.heatmonth with overflow:visible, committed 2026-09-24, two days before this session)
combined with a gap in check_d9.go's own detection (it flags any non-hidden overflow,
not just an actual scroll/auto one); confirmed unrelated to this session two ways (today
is not a Monday, the one day-of-week the heatmap's own DST fix could have changed
anything; the CSS predates this session) and left as-is, since fixing check_d9.go's own
detection logic is outside WS2's scope. w1 (burnmon.exe's own startup-timing check, a
fixed 1500ms WebView2 warm-up budget its own doc comment calls independent of app code)
passed once earlier in the session but could not get a clean run against the final build
after five attempts; the last attempt's own screenshot captured an entirely different,
unrelated foreground window (confirmed live via GetForegroundWindow), which is real
desktop contention on a machine in active interactive use during the run, not a code
regression - w1 exercises only cmd\burnmon's own startup path, which nothing in this
session's diff touches. w0/w2-w8 all passed clean on the final build.

## 4. Findings worth propagating
- [RESULT] Post-rebase, pre-anything-else 10-minute baseline (Go process, burnmon.exe
  co-running): 0.89 percent avg / 1.95 percent peak CPU, 252.5 MB peak RAM, 763 peak
  handles - confirms the WS3 rebase already removed most of the earlier session's own
  shared-ingest climb.
- [RESULT] Post-patch (items 1-6 plus review fixes) 10-minute measurement, active,
  burnmon.exe co-running: Go process 0.28 percent avg / 0.59 percent peak CPU, 195.4 MB
  peak RAM, 781 peak handles; WebView2 tree 0.51 percent avg / 1.36 percent peak CPU,
  671.1 MB peak RAM, 6 processes peak. burnmon.exe itself (context, unaffected): 0.05
  percent avg / 0.17 percent peak CPU, 92.3 MB peak RAM, 421 peak handles.
- [RESULT] Minimized for 5 minutes: Go process 0.15 percent avg / 0.80 percent peak CPU,
  297.2 MB peak RAM; WebView2 tree 0.03 percent avg / 0.11 percent peak CPU, 499.0 MB
  peak RAM - CPU on both drops sharply from the active run (WebView2 CPU down about 94
  percent relative), confirming the minimize-pause actually holds even without the
  WebView2 memory-target call.
- [RESULT] Headline evidence (item 6, watched for real during this session's own active
  Claude Code session, its own transcript live-ingested): 9 real value changes out of 30
  render ticks in a measured 29.0-second window.
- [RESULT] A fresh, independent Opus review over the full diff before commit found five
  real issues, all fixed with a failing test (or a node repro for the JS-only one)
  confirmed first: (1) the activity heatmap's own week count silently dropped a whole
  week, including today's own column, on any Monday whose 182-day lookback crossed a DST
  transition (a raw ms/604800000 division, now day-counted first); (2) the process-groups
  crop's "-1 row" headroom guess hid one row that actually fit whenever the real thead was
  shorter than a data row, now measured against the thead's real height; (3)
  internal/advisor's evalSelfOverhead still only read HarnessSelf after item 4's split,
  silently missing WebView2's own larger share of this app's overhead, now sums both; (4)
  internal/devexport's dailyRows bucketed by UTC calendar day even though
  --since/--until now select a local window, now buckets by local day (plus the same fix
  for summary.md's top-turns table); (5) the process-registry cache (item 1) had no guard
  against a pid being reused by a different process within one 3s walk, now guarded by
  the snapshot's own CreateTime. The hand-derived Windows struct layout (item 1's
  systemProcessInfoT), the harness-split ordering, the DOM-reuse reconciliation and the
  hidden atomic.Bool concurrency were all independently checked and found correct.
- [RESULT] Full verify pass after the review's own fixes: go vet ./..., go test
  ./... -count=1 (every package, including all new/updated tests), .\build.ps1,
  node --check on both page JS files, uicheck d0-d12 re-run in full (all pass except the
  pre-existing, unrelated d9/1920x1080 finding above), w0/w2-w8 all pass (see section 3
  for w1's own environmental-contention finding).
- [STATE] Every .UTC() call in cmd\burnmon-dev is now gone (headline.go's own named fix,
  plus two more the spec did not name directly: the activity heatmap's day boundary and
  the vendor strip's cache-breakdown day boundary, both in the same UTC/local class of
  bug). internal/devexport's own .UTC() calls on the exported bundle's instant fields
  (GeneratedAt/Since/Until/event At) are kept, deliberately, matching WS3's own convention
  that a portable, machine-readable export stays UTC while display converts to local at
  render time, except dailyRows' own day-bucket key and summary.md's top-turns display
  time, both fixed to local by the review pass (finding 4 above).

## 5. Hub-level decision (if any)
Nothing. This is a project-internal performance patch and its own review-fix pass, not a
cross-project, pricing, positioning, park/unpark or CIPHER-wall decision.

## 6. What the next hub read should update
C:\ZND\10_holding\01_projects\burnmon.md (the hub one-pager) should reflect that WS2's own
performance patch is code-complete on branch burnmon-dev at v0.4.0-alpha.2, pending
Wilco's own push/tag/merge decision. C:\ZND\10_holding\02_roadmap\roadmap.md if it tracks
this patch as an open item. No DEADLINES.md entry expected (no dated gate tied to this).

## 7. Open flags for next session
- Push, tag and merge are Wilco's own manual steps (commands below); nothing here expires
  or blocks other work in the meantime.
- WebView2's own memory usage target ("Low" while minimized) is not built; a future
  session with access to the real WebView2 SDK headers (to confirm
  ICoreWebView2Controller4's exact COM vtable layout) could revisit it.
- The Microsoft To Do panel's own due-date timezone handling needs a live, signed-in
  session to confirm whether Microsoft Graph's dueDateTime needs local-time correction;
  not exercised live this session.
- w1 (burnmon.exe's own startup-timing uicheck) should be re-run alone once the desktop
  is idle, to get one final confirmation independent of this session's own foreground
  contention; w0/w2-w8 already confirmed green on the final build.
- The stray, garbled-named WebView2 profile cache folder keeps recurring in the worktree
  root during test runs (twice now, across two different sessions); worth a look if it
  keeps happening, though harmless so far (just an EBWebView cache folder, removed both
  times).

## 8. Related files
- Spec: C:\ZND\projects\burnmon\02_roadmap\2026-09-26_ws2_performance_patch.md
- Prior briefs: C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-26_ws2_phase5_verify_release.md,
  ...\hub_agent_update_2026-09-26_ws3_shared_ingest_performance.md
- Design: C:\ZND\projects\burnmon\04_assets\2026-09-24_burnmon_dev_design.md
- Hub one-pager: C:\ZND\10_holding\01_projects\burnmon.md

## PowerShell commands for Wilco

Push burnmon-dev with force-with-lease (the branch was pushed before this session's
rebase, so a plain push would be rejected). Run in PowerShell, at C:\ZND\projects\burnmon:

    cd C:\ZND\projects\burnmon
    git fetch origin
    git push --force-with-lease origin burnmon-dev

Tag v0.4.0-alpha.2 once you are happy with the push (optional, this is an alpha). Run in
PowerShell, at C:\ZND\projects\burnmon:

    cd C:\ZND\projects\burnmon
    git tag v0.4.0-alpha.2 burnmon-dev
    git push origin v0.4.0-alpha.2
