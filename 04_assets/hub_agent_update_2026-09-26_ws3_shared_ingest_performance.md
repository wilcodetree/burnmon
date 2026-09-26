# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-26 - **Owner:** Wilco de Tree
**Project:** BurnMon (WS3, shared ingest performance and local time everywhere)
**Purpose:** WS3 finished: profiled and fixed the shared-ingest handle/memory climb both
exes exhibited, and made every day/week/month boundary local time instead of UTC.
Committed on `main` as v0.3.2, not pushed or tagged.
**Read order:** this file, SESSION_LOG.md's newest entry, STATUS.md's top section and its
two new Known gaps entries, `04_assets\2026-09-26_ws3_profile_before_after.md` (the full
step 0 profile and before/after numbers).
**Supersedes:** nothing (first WS3 brief).

## 1. Headline
BurnMon v0.3.2 is committed on `main` (`69a071e`): the reported shared-ingest handle and
memory climb is fixed and measured for real (peak RAM 957MB to 87MB, peak handles
13,541 to 371), and every day/week/month boundary now uses local time instead of UTC.
Not pushed, not tagged; Wilco has the exact commands to run himself, below.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (branch `main`):
  - `69a071e` v0.3.2 WS3: shared ingest performance, local time everywhere (28 files,
    1,862 insertions, 200 deletions).
- **On a branch, not merged:** nothing from this session; `burnmon-dev` (WS2) is
  untouched, still pending its own rebase onto this commit.
- **Files touched** (full paths): `C:\ZND\projects\burnmon\internal\watch\watch.go`
  (refactored behind a new `nativeBackend` interface),
  `...\internal\watch\watch_windows.go` (new, the hand-rolled recursive
  `ReadDirectoryChangesW` watcher), `...\internal\watch\watch_other.go` (new, the old
  per-directory fsnotify design moved here for the untested darwin/linux build),
  `...\internal\watch\watch_test.go`, `...\cmd\burnmon\profiling.go`,
  `...\profiling_windows.go`, `...\profiling_other.go` (new, the `BURNMON_PPROF=1`
  diagnostic switch), `...\cmd\burnmon\app.go`, `...\main.go`,
  `...\cmd\burnmon-cli\main.go`, `...\internal\vendorstrip\vendorstrip.go`,
  `...\internal\forecast\forecast.go`, `...\forecast_test.go`,
  `...\internal\dataset\dataset.go`, `...\fromstore.go`, `...\fromstore_test.go`,
  `...\internal\history\history.go`, `...\history_test.go`, `...\internal\export\export.go`,
  `...\internal\store\store.go`, `...\store_test.go`, `...\internal\agg\agg.go`,
  `...\agg_test.go` (new), `...\internal\report\template.html`, `README.md`, `STATUS.md`,
  `SESSION_LOG.md`.
- **Untracked / outside a repo:** `04_assets\2026-09-26_ws3_profile_before_after.md` is
  committed (in the list above), not left loose.

## 3. What did NOT happen (and why)
Not pushed, not tagged: house process for this workstream stops at the commit and hands
Wilco the exact PowerShell commands, below. Three of the four step 0 hypotheses (the
400MB memory cap, whole-table `AllEvents` loads, double ingest between `burnmon.exe` and
`burnmon-dev.exe`) were tested and confirmed NOT to be driving the reported symptom, so
none of the larger fixes those hypotheses proposed (retuning the memory cap, a SQL
aggregate or daily-summary-table rewrite of History's aggregation, a named-mutex
single-ingest-owner) were built; two real, smaller findings from that testing (a
`bmHistory` UI-latency cost, and `burnmon-dev.exe`'s own numbers being unimproved until
it rebases) are recorded in STATUS.md's Known gaps, not fixed here. `burnmon-dev.exe`
itself was used read-only (the pre-built binary already on its own branch); WS2's own
rebase onto this commit is a separate, not-yet-scheduled step.

## 4. Findings worth propagating
- [RESULT] Step 0 profiling (`BURNMON_PPROF=1`, new) found 13,125 of a real process's
  13,541 open handles were one fsnotify watch per subdirectory, against ~12,700 real
  Cowork session folders on Wilco's own laptop; cross-checked directly against the
  filesystem, not left as a correlation.
- [RESULT] Fixed with a small hand-rolled Windows recursive watcher (one
  `ReadDirectoryChangesW` call per native root instead of one per subdirectory), after
  confirming directly (a throwaway program, not just reading fsnotify's own doc comment)
  that the vendored fsnotify's undocumented recursive-watch path is genuinely broken:
  wrong reported path, missed nested-file events.
- [RESULT] Measured for real, 10 minutes each: `burnmon.exe` alone, peak RAM 957MB to
  87MB, peak handles 13,541 to 371, average CPU about 0.22 percent (target: under 2
  percent, under 250MB). With the unmodified `burnmon-dev.exe` running alongside,
  `burnmon.exe`'s own numbers are statistically unchanged (about 85.5MB/379 handles/0.09
  percent), confirming double ingest between the two exes does not need a fix once the
  handle-count bug is gone.
- [RESULT] Two independent fresh reviews of the diff, before commit, both real findings:
  the first found four real bugs (a DST week losing its last day's recorded actual, a
  UTC-vs-local mismatch in the History tab's default date range, a session wrongly
  dropped from the retained window at a retention cutoff, a third-party billing
  boundary that had accidentally been switched to local time) plus five Win32/
  concurrency hardening items in the new watcher; the second, over those fixes, found
  two more real problems (a DST test whose own portability guard checked the wrong
  clock, and a watcher read that never re-armed after a transient error) and confirmed
  the rest clean. All fixed, all re-verified: full detail in
  `04_assets\2026-09-26_ws3_profile_before_after.md`.
- [RESULT] `go vet ./...`, `go test ./... -count=1` clean (every package, including nine
  new tests across five packages); `internal/watch`, `internal/forecast`, `internal/agg`
  specifically repeated 10 times each with no failures; `.\build.ps1`,
  `.\scripts\uicheck.ps1` clean (`w1`'s own known retry-flake reproduced and passed on
  retry, not a regression; `w8` passed clean, its own STATUS.md open question left
  unresolved rather than silently closed).
- [STATE] `burnmon-dev.exe` (WS2, on its own branch) still shows the pre-fix handle/RAM
  climb when run: expected, it has not rebased onto this commit yet.

## 5. Hub-level decision (if any)
Nothing. This is a project-internal performance and correctness pass, not a
cross-project, pricing, positioning, park/unpark or CIPHER-wall decision.

## 6. What the next hub read should update
`C:\ZND\10_holding\01_projects\burnmon.md` (the hub one-pager) should reflect v0.3.2 on
`main`, committed, not pushed or tagged, and that WS2 (BurnMon Dev) is next in line to
rebase onto this commit. `C:\ZND\10_holding\02_roadmap\roadmap.md` if it tracks WS3 as an
open item. No DEADLINES.md entry expected (no dated gate tied to this).

## 7. Open flags for next session
- Push and tag are Wilco's own manual steps (commands below); nothing here expires or
  blocks other work in the meantime.
- WS2's own rebase onto this commit (to inherit the shared-ingest fix) is a separate,
  not-yet-scheduled step, per the plan's own stated order.
- `bmHistory`'s on-demand `store.AllEvents()` still costs about 3 seconds per History tab
  filter change against Wilco's real store size (STATUS.md's own Known gaps): a real
  UI-latency cost, not fixed this session since the profile did not show it driving the
  resource-climb symptom this session's Done-when targeted.
- Two low-severity, pre-existing patterns flagged rather than fixed (STATUS.md's Known
  gaps): one `forecast_scores` row with a UTC-Monday `week_start` left over from before
  this session (affects at most one ISO week), and a lexical-string sub-second edge case
  in `store.go`'s event-time bounds.

## 8. Related files
- Spec: `C:\ZND\projects\burnmon\02_roadmap\2026-09-26_ws3_shared_ingest_performance.md`
- Full profile and before/after numbers:
  `C:\ZND\projects\burnmon\04_assets\2026-09-26_ws3_profile_before_after.md`
- WS2 phase 5 hub brief (this session's baseline numbers):
  `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-26_ws2_phase5_verify_release.md`
- SESSION_LOG.md's newest entry, STATUS.md's top section

## PowerShell commands for Wilco

Push `main` and tag v0.3.2 (run in: PowerShell, `C:\ZND\projects\burnmon`):

```powershell
cd C:\ZND\projects\burnmon
git push origin main
git tag v0.3.2
git push origin v0.3.2
```
