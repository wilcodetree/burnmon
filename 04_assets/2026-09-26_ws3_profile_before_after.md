# WS3: shared ingest performance, step 0 profile and before/after numbers

Date: 2026-09-26. Spec: `02_roadmap\2026-09-26_ws3_shared_ingest_performance.md`. Machine: Wilco's
laptop, 20 logical processors, Europe/Amsterdam. Method: `BURNMON_PPROF=1` (new,
`cmd\burnmon\profiling.go`/`profiling_windows.go`/`profiling_other.go`) plus PowerShell
`Get-Process` sampling every 15s for 10 minutes, CPU normalised to percent of the whole
20-core machine (`(cpuDelta / intervalSeconds / 20) * 100`), matching the WS2 phase 5
methodology exactly (same fixed normalisation bug that brief's own fix addressed).

## Step 0: before any change

`burnmon.exe` (0.3.1, unmodified) run alone with `BURNMON_PPROF=1`, sampled continuously:

- Heap profile (`heap-start.pprof`, `heap-after-60s.pprof`) and a 60s CPU profile
  (`cpu-60s.pprof`) written to `%LOCALAPPDATA%\burnmon\`.
- MemStats/handle/watch-count line every 10s in `burnmon-app.log`, e.g. 90s in:
  `HeapAlloc=870475880 Sys=992626936 NumGC=6117 GCCPUFraction=0.0141 handles=13541
  fsnotify_watches=13125 goroutines=10`.
- `Get-Process` after 75s: `WorkingSet64=957267968` (913 MB), `Handles=13541`,
  `CPU=126.75s` cumulative (startup backfill, not steady state).
- Directory count under the four native roots (`find -type d`): `.claude\projects` 360,
  `.codex\sessions` 25, but the Cowork root
  (`AppData\Local\Packages\Claude_*\LocalCache\Roaming\Claude\local-agent-mode-sessions`)
  alone holds **12,732 subdirectories** (11,756 `.jsonl` files, essentially one folder per
  Cowork/agent-mode session). `fsnotify_watches=13125` matches this almost exactly:
  97% of the process's open handles were one-fsnotify-watch-per-subdirectory, confirming
  hypothesis 1 outright, not a guess.
- `debug.SetMemoryLimit(400 << 20)` (`cmd\burnmon\app.go`): live heap sat at ~870 MB,
  more than double the 400 MB soft cap, `GCCPUFraction` 1.3-1.4% (elevated, not
  "continuous burn": the symptom tracked back to hypothesis 1's handle/bookkeeping bloat,
  not an independent GC-pressure bug, confirmed below).
- `stage AllEvents: 3.02s, 54855 rows` on one Collect pass (every 15 minutes, or on
  Refresh now/Settings save/History tab query): real, but periodic, not sustained.

## Hypotheses, what each turned out to be

**1. One watch per folder: CONFIRMED, fixed.** `internal\watch\watch.go`'s `addTree`
called `fsw.Add` once per subdirectory under every native root; against Wilco's real
Cowork history that is 13,125 open directory handles, the large majority of the
process's total. Checked, not assumed, whether the vendored fsnotify (v1.10.1) supports
recursive watching on Windows: its own doc comment says plainly "Recursive watching is
not currently enabled through fsnotify's public API; the recursive code path is gated and
only exercised by fsnotify's own tests." Tried directly anyway (a throwaway program,
`fsw.Add(root + "\...")`, then a folder-then-file race identical to the F7 test below):
it reported only one event, for the top-level folder, with the literal "..." segment
left in the reported path, and never delivered the nested file's own event at all.
Confirmed unusable, not merely undocumented, so `internal\watch\watch_windows.go` is a
small hand-rolled watcher instead: one recursive `ReadDirectoryChangesW` (IOCP,
overlapped I/O, `windows.FileNotifyInformation` buffer decode) per native root, added
once at construction and never touched again, since the recursion itself covers every
subdirectory created under it from then on, at any depth. `internal\watch\watch_other.go`
keeps the old per-directory fsnotify design for B1's browser-mode darwin/linux build
(untested on real hardware, out of WS3's scope). `TestWatcher_NewNestedDayFolderRace`
(the F7 regression test) stays green, and now passes because the race is structurally
impossible, not because it got lucky: a single recursive watch already covers a
brand-new nested folder from the moment its parent root was added, no per-folder `Add`
call ever needed. New `TestWatcher_WatchCountStaysOneAcrossManySubdirectories` proves
`WatchCount()` stays at 1 across 50 newly created, several-levels-deep subdirectories,
where the old design would have grown by 50.

**2. Memory cap causing GC burn: NOT an independent cause.** The live heap did sit above
400 MB (measured ~870 MB), but `GCCPUFraction` never exceeded ~6% even in that state, not
the "GC runs almost continuously" hypothesis wording implied; the actual before/after
comparison shows the ~870 MB heap was itself mostly hypothesis 1's own bookkeeping (13,125
fsnotify watch structs and their buffers): after hypothesis 1's fix, live heap sits at
3-6 MB, Sys ~140 MB, `GCCPUFraction` ~0.1-0.4%, both far under the 400 MB cap on their
own. `debug.SetMemoryLimit(400 << 20)` is kept unchanged: still well above the new live
heap (a ~65x margin), a cheap safety net for a genuine rebuild burst (Collect's own
event/session slices), not a lever that needed retuning once hypothesis 1 was fixed.

**3. Whole-table loads: real cost, not confirmed as the reported symptom's driver, not
fixed.** `store.AllEvents()` (54,855 rows) took 3.02s in the step 0 capture, feeding
`Collect` (every 15 minutes, or Refresh now/Settings save) and `bmHistory` (every History
tab filter change). That is a genuine, measurable cost, but it is periodic, not sustained:
it does not explain "climbs to 900MB+ within one to two minutes and stays there", which
step 0 already pinned on hypothesis 1. With hypothesis 1 alone fixed, both the solo and
dual-instance 10-minute measurements below already clear both Done-when targets with
comfortable margin. Per the spec's own instruction ("fixing only what the profile
confirms"), the SQL-aggregate/daily-summary-table rewrite this hypothesis proposed was
not built: the profile does not show it is needed to hit the targets, and building it
anyway would be exactly the kind of speculative rewrite AGENTS.md's Simplicity First rule
warns against. Flagging for a future pass, not silently dropping it: `bmHistory`'s
on-demand `AllEvents` still costs ~3s against Wilco's real store size on every History tab
filter change, which is a real UI-latency cost even though it is not a resource-climb bug.

**4. Double ingest: real, not confirmed as needing a fix.** `cmd\burnmon-dev\main.go` and
`cmd\burnmon\main.go` both call `store.DefaultPath()`/`store.Open()`, i.e. both exes read
and write the exact same `burnmon.db`, both independently watching, parsing and ingesting
the same source files (confirmed by reading both entry points, not assumed from the
spec's own wording). `store.Open`'s existing WAL mode plus `busy_timeout=5000` plus
idempotent upsert already makes two independent writers safe: no `SQLITE_BUSY` or lock
errors appeared in either app's log across the full 10-minute dual-instance run below.
The measured numbers confirm this concretely: `burnmon.exe`'s own footprint (peak RAM,
handles, average CPU) is statistically the same whether `burnmon-dev.exe` is running
alongside it or not (see the two tables below). A named-mutex single-ingest-owner
(reading is fine, only one process's watcher/pollers actually run) would add real
correctness-risk surface (handover logic, crash detection, a second code path to keep in
sync) for a benefit the profile does not show is needed once hypothesis 1 is fixed; not
built, per the same "fixing only what the profile confirms" instruction.

## Before/after: burnmon.exe alone, 10 minutes

| | Before (v0.3.1, unmodified) | After (this session) | Target |
|---|---|---|---|
| Avg CPU (whole machine) | not separately isolated; combined dual-instance figure was 1.89-2.22% (WS2 phase 5 brief) | ~0.22% | < 2% |
| Peak CPU (one 15s interval) | n/a (see handles/RAM below, the dominant symptom) | 5.07% (the startup backfill interval only; every steady-state interval read under 0.2%) | n/a |
| Peak RAM (working set) | ~957 MB (75s sample) | ~87 MB (86,958,080 bytes) | < 250 MB |
| Peak handles | 13,541 | 371 | n/a (was the headline symptom) |
| fsnotify/recursive watches | 13,125 | 4 (one per native root) | n/a |

Raw samples: `%TEMP%\burnmon_alone_10min.csv` (this session's scratch output, not
committed).

## Before/after: burnmon.exe (fixed) plus burnmon-dev.exe (unfixed, read-only), 10 minutes

`burnmon-dev.exe` here is the pre-built binary already on the `burnmon-dev` branch
(`.claude\worktrees\burnmon-dev\burnmon-dev.exe`, v0.4.0-alpha.1, WS2 phase 5's own
build), used read-only per the spec: it has not rebased onto this session's fix yet
(the plan's own stated order: WS3 lands on `main` first, WS2 rebases onto it afterward),
so its numbers below are expected to still show the pre-fix pattern.

| | `burnmon.exe` (fixed) | `burnmon-dev.exe` (unfixed, out of scope) | Target |
|---|---|---|---|
| Avg CPU (whole machine) | ~0.09% | ~4.3-4.6% (steady state) | < 2% |
| Peak RAM (working set) | ~85.5 MB | ~1,002 MB (1,050,972,160 bytes) | < 250 MB |
| Peak handles | 379 | ~13,944 | n/a |

`burnmon.exe`'s own numbers here are statistically the same as the solo run above
(hypothesis 4 confirmed not to need a fix). `burnmon-dev.exe`'s numbers reproduce the WS2
phase 5 brief's own already-documented shared-ingest climb exactly (its own
`internal\watch` import is the same package this session fixed on `main`; a future WS2
rebase onto this commit inherits the fix automatically, no separate change needed there).
Raw samples: `%TEMP%\burnmon_both_10min.csv` (this session's scratch output, not
committed).

## Live-ingest latency (unchanged, verified)

`TestWatcher_NativeRootSeesNewFile` (new file, ~10-30ms in test runs) and
`TestWatcher_NewNestedDayFolderRace` (new nested folder plus its first file, back to
back, no separate `Add` call in between, ~10ms) both still pass, now against the
recursive backend rather than fsnotify's per-directory one. A real-app check (fresh
`burnmon.exe`, a new `.jsonl` dropped into a brand-new folder under
`~\.claude\projects\ws3-live-ingest-check\`) shows `stage ingest` for the new 1-file,
152-byte write in the same log-timestamp second as the file was written; the first
attempt at this manual check grepped the log for the file's own name and found nothing,
which is a log-verbosity gap (the terse `stage ingest` line never includes the path,
unlike the `>10MB` verbose branch), not a real ingest miss, confirmed by the timestamp
match and by both automated tests above.

## Two fresh independent reviews, both real findings, both fixed

Per the spec's "fresh read-only Opus review before each commit", this landed in two
passes rather than one clean pass, because the first pass found real bugs.

**Pass 1** (over the full diff) found four things worth fixing, beyond confirming the
watch rewrite's Win32/IOCP mechanics and the profiling switch were sound:

1. `internal\forecast\forecast.go`'s `EnsureScored`/`actualTokensForWeek` did
   `sc.WeekStart.AddDate(0, 0, 7)` on a `WeekStart` that came back from
   `store.ForecastScore` still UTC-located (even though the instant it names is a local
   midnight). `AddDate` reconstructs via its receiver's own Location, so this added UTC
   calendar days (always exactly 24h) instead of local ones (23h/25h on a DST
   transition's own day): on the 2026-10-25 fall-back week the computed week-end landed
   one hour short of the real Monday 00:00 local boundary, silently dropping the whole
   of Sunday from the recorded actual, permanently (RecordForecastActual only fires
   once). Fixed at the source: `store.scanForecastScore` now converts `WeekStart` to
   `time.Local` right after parsing it back, so every downstream `AddDate`/`Format` call
   on it is correct without each call site needing to remember its own conversion. New
   test `TestActualTokensForWeek_DSTFallBackIncludesSunday`
   (`internal\forecast\forecast_test.go`), confirmed to actually fail without the fix.
2. `internal\report\template.html`'s `isoDate` (the History tab's default date-range
   builder) used `d.toISOString().slice(0,10)`, a UTC calendar day, sent to
   `history.Filter.From/To`, which are now local calendar days. For roughly two hours a
   day around local midnight (Europe/Amsterdam's own UTC+1/+2 offset) this shifted the
   default range's `to` one day early, silently excluding "today". Fixed: `isoDate` now
   builds the string from `getFullYear()/getMonth()/getDate()` (local), matching the Go
   side. `histPeriodStartDate`'s own `toISOString()` calls and the `getUTCDay` calls
   elsewhere in `template.html` were checked and are unaffected: pure integer calendar
   arithmetic from an ISO year/week number or a parsed date string, never touching a
   real wall-clock instant or the viewer's zone.
3. `internal\agg\agg.go`'s `Build` compared `cutoff` (now local-midnight, from
   `dataset.monthStart`) against `parseDay(s.End[:10])`/`parseDay(daystr)` (UTC-anchored:
   `s.End` is deliberately still a UTC-instant-labelled string, and `parseDay` anchors
   any bare date string to UTC midnight) via `.Before()`, an instant comparison mixing
   two different Locations. A session whose last real activity fell in the early local
   morning hours right at a retention cutoff had a UTC End-day one calendar day earlier
   than its true local day, so the old comparison silently dropped the whole session
   from the retained window even though its own (correctly local, post the fromstore.go
   fix) `Daily` map placed it inside it. Fixed: plain string comparison against
   `cutoff.Format("2006-01-02")` (safe here: `cutoff` is freshly computed with
   `time.Local` within the same call, never round-tripped through storage), matching
   `s.Daily`'s own local-day keys directly instead of re-parsing them through a UTC
   anchor. New test `TestBuild_CutoffComparesLocalDaysNotUTCInstant`
   (`internal\agg\agg_test.go`), confirmed to actually fail without the fix.
4. `internal\vendorstrip\vendorstrip.go`'s Copilot premium-request credits calculation
   (`creditsLeftForMonth`) had picked up the same `monthStart` this session switched to
   `time.Local` for the display totals, but GitHub's own billing-cycle reset is a
   third-party UTC boundary, not a display convention Wilco's local time zone has any
   bearing on. Fixed: a separate `utcMonthStart`, used only there; the display totals
   keep `monthStart` (local).

Two smaller hardening items from pass 1, not correctness bugs but worth doing anyway:
`store.DailyTokenTotals` split into an unbounded form and a new `DailyTokenTotalsUntil`,
with `actualTokensForWeek` switched to the bounded one (its own week's seven days, not
everything from that week to "now"), avoiding an O(weeks x rows-since-week-start)
rescan cost after a backlog of unscored weeks; and `internal\watch\watch_windows.go`
hardened in five ways pass 1 caught in the Win32/IOCP mechanics themselves (not the
local-time work): `Close` was calling `CancelIo`, which only cancels I/O issued by the
calling thread and was therefore a no-op against reads issued by the reader goroutine's
own re-arms, switched to `CancelIoEx`; a race where the reader goroutine could re-arm a
read on a handle `Close` was concurrently closing, fixed with a per-root mutex guarding
both; `onChange` used to run before the read was re-armed, widening a kernel
buffer-overflow's blast radius from "one folder" (the old per-directory design) to
"everything under a root", fixed by decoding into a path list first and re-arming
before dispatching; a failed first read in `AddRoot` used to leave a root registered
with an open handle; and `decode` gained explicit bounds checks before dereferencing a
notification entry or slicing its filename (defensive, the kernel is trusted, not an
observed live bug).

**Pass 2** (over pass 1's own fixes) confirmed four of the six clean and caught two real
problems in the other two, both fixed:

5. `TestActualTokensForWeek_DSTFallBackIncludesSunday`'s own skip guard checked
   `monday.Zone()`, always +2h since `monday` was constructed with the Amsterdam
   Location directly, rather than the actual machine's `time.Local` the code under test
   uses. The guard could never self-skip on a non-Amsterdam machine; the test itself
   was genuine (confirmed to fail without fix 1 above), only its portability guard was
   wrong. Fixed to check `monday.In(time.Local).Zone()`, matching every other DST
   test's own pattern.
6. `internal\watch\watch_windows.go`'s error branch (when `GetQueuedCompletionStatus`
   itself reports an error for a root's read) logged and gave up, never re-arming: a
   single transient error (a remote path hiccup, any recoverable I/O error) permanently
   killed that root's live watching for the rest of the process's life, silently
   falling back to the 15-minute rescan forever rather than just once. Fixed: retry
   once, the same re-arm the success path already does. This interacted with a second,
   related gap the same pass found: `Close`'s new shutdown sequence posted its wakeup
   packet without first confirming every root's just-cancelled read had actually
   finished completing through the port; closing a handle unblocks a pending overlapped
   read but does not itself guarantee the kernel is done writing into it. Fixed
   together: `recursiveRoot` gained a `reading` flag kept true only exactly while a read
   is genuinely outstanding, and `windowsRecursiveBackend` a `draining` WaitGroup that
   `Close` populates (one per root that was truly reading at cancel time) and waits on
   before posting the shutdown signal, with `run` calling it Done for each closed root's
   real final completion. Not a production risk (the real app never calls `Stop`; a
   watcher lives for the whole process and is torn down by process exit, not this
   code), but the test suite constructs and stops a `Watcher` constantly, so worth being
   correct about.

Two smaller items pass 2 flagged rather than blocked on, both pre-existing patterns
rather than something this session introduced, noted in `STATUS.md`'s Known gaps
instead of fixed: a `forecast_scores` row written before this session keeps a
UTC-Monday `week_start` (affects at most the one ISO week unscored at upgrade), and
`store.go`'s `at >= ?`/`at < ?` bounds compare RFC3339Nano strings lexically, which can
misplace a single event at an exact sub-second boundary.

Every fix from both passes re-verified: `go vet ./...` and `go test ./... -count=1`
clean, `internal/watch`/`internal/forecast`/`internal/agg` specifically repeated 10x
with no failures, `node --check` re-run on `template.html`'s two script blocks after
the `isoDate`/`histRangeDates` edits, `.\build.ps1` and `.\scripts\uicheck.ps1` both
re-run clean (`w1`'s own retry-flake and `w8`'s own behaviour unchanged from the first
pass).
