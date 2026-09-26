# WS3: shared ingest performance (CPU and memory), BurnMon v0.3.2

Owner: Wilco. Executor: fresh Claude Code session, Sonnet 5 (Opus for the review agent).
Branch: `main` in `C:\ZND\projects\burnmon`. Runs AFTER WS2 phase 5 has finished, so its
10-minute measurement is the baseline. WS2 (`burnmon-dev`) rebases on this afterwards.
Never use em dashes anywhere.

Read first: `AGENTS.md`, `C:\ZND\AGENTS.md`, `STATUS.md`, top of `SESSION_LOG.md`, and the
WS2 phase 5 hub brief in `04_assets\` (its CPU, RAM and handle numbers are the baseline).

## Problem (observed, not yet profiled)

Both `burnmon.exe` and `burnmon-dev.exe` climb to about 900 MB to 1.3 GB and about 13k handles
within the first one to two minutes on Wilco's laptop. BurnMon Dev's own process-groups row
showed "BurnMon Dev 47% CPU, 1.3G" (unit unclear, may include WebView2). Target for each exe,
measured over 10 minutes with the other one running: under 2 percent average CPU of the whole
machine, under 250 MB RAM.

## Step 0: measure, before changing anything

- Add an opt-in profiling switch (env `BURNMON_PPROF=1`): write a heap profile and a 60 s CPU
  profile to the data folder, plus `runtime.MemStats` every 10 s to the log. Off by default.
- Count handles by type (Sysinternals `handle.exe -s -p <pid>` if present, else
  `GetProcessHandleCount` plus the Go side count of fsnotify watches).
- Report: top 10 heap owners, top 10 CPU functions, handle count by type, number of watched
  directories, live heap versus the `debug.SetMemoryLimit(400 MB)` cap.

## Hypotheses to test, in order (each is unverified until step 0 confirms it)

1. **One watch per folder.** `internal\watch\watch.go` `addTree` (line ~96) calls `fsw.Add`
   for every subdirectory under the Claude and Codex roots. On Windows each watch is an open
   directory handle plus its own change buffer. Replace it with one recursive watch per root
   (`ReadDirectoryChangesW` with subtree on). Check first whether the vendored fsnotify
   version supports recursive watches on Windows; if not, write a small Windows-only watcher
   in `internal\watch` behind the same `OnChange` interface. Keep the F7 nested-folder race
   test (`TestWatcher_NewNestedDayFolderRace`) green; with a subtree watch the race should
   disappear, prove it.
2. **Memory cap causing GC burn.** If the live heap sits above the 400 MB soft limit, the Go
   GC runs almost continuously and burns CPU. After fixing the heap, keep a limit only if it
   sits well above the new live heap; document the chosen value and why.
3. **Whole-table loads.** `store.AllEvents()` feeds `dataset.Cache.Collect` (startup and the
   15-minute rescan) and History (`bmHistory`). Replace per-row Go aggregation with SQL
   aggregates (`SUM ... GROUP BY day, agent, model`) or a small daily summary table kept up to
   date on upsert. The Sessions tab may keep per-session rows, but must not hold every event.
4. **Double ingest.** When both exes run, both watch, parse and poll the same sources. Add a
   single ingest owner: a named Windows mutex (for example `Local\burnmon-ingest`); the holder
   runs watcher and pollers, the other process only reads the store and takes over if the
   holder exits. Idempotent upsert stays as the safety net.

## Extra item 5: local time everywhere (Wilco, 2026-09-26)

Today "today" and every day bucket start at UTC midnight (`vendorstrip.dayStart`, and about
24 Go files call `.UTC()` or `time.UTC`). Wilco's decision: all times are local time.
- Storage stays as it is: timestamps stored in UTC, no migration of raw events.
- Every day boundary, day bucket and "today" uses local midnight (`time.Local`): vendor strip,
  History, heatmap, forecast, insight, live, dataset, CLI and export date ranges. DST days are
  23 or 25 hours; test both 2026 switch dates (29 March, 25 October, Europe/Amsterdam).
- Every time shown in the UI is local, 24-hour.
- Export: `--since` and `--until` are local dates; timestamps in the export carry their offset.
- If hypothesis 3 adds a daily summary table, key it on the local day.
- Keep a UTC call only where it is correct (parsing, storage, comparisons) and say why in a
  one-line comment. Report every `.UTC()` call kept or removed.

## Done when

- Step 0 report written in `04_assets\2026-09-26_ws3_profile_before_after.md`, with before
  and after numbers from the same method.
- `burnmon.exe` alone and `burnmon.exe` plus `burnmon-dev.exe` (from the burnmon-dev branch
  build, read-only use) each measured for 10 minutes: average and peak CPU, peak RAM, handles.
  Report against the targets, real numbers either way.
- A new file written by Claude Code or Codex still appears in BurnMon within 2 s (live ingest
  unchanged), with a test.
- `go vet ./...`, `go test ./... -count=1`, `.\build.ps1`, `node --check` on the template JS,
  `.\scripts\uicheck.ps1` (known pre-existing `w8`, `w1` failures: report, do not fix).
- Fresh read-only Opus review agent over the diff before each commit.
- Version v0.3.2 in `cmd\burnmon\app.go` and `cmd\burnmon-cli\main.go`; README, STATUS,
  SESSION_LOG updated. Commit on `main`, do not push or tag. Hand Wilco the PowerShell commands.
- One hub brief with the `hub-agent-update` skill in `C:\ZND\projects\burnmon\04_assets\`.

## Out of scope

BurnMon Dev's own sampler, frontend churn and minimized-window pause (that is the WS2
performance patch, `2026-09-26_ws2_performance_patch.md`, after WS2 rebases on this).
