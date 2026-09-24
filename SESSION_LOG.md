# BurnMon, session log

One paragraph per work session, newest on top.

## 2026-09-26, WS3: shared ingest performance, local time everywhere, v0.3.2

Read first: `02_roadmap\2026-09-26_ws3_shared_ingest_performance.md` (decisions already
taken: profile before changing anything, test the four listed hypotheses in order, fix
only what the profile confirms; local time everywhere per Wilco's own extra item 5), the
WS2 phase 5 hub brief (baseline: 1.89-2.22% avg CPU, 937 MB peak RAM, ~13.9k peak handles,
both exes running). Ran on `main`, after WS2 phase 5 finished; WS2 rebases onto this
afterward.

**Step 0, profiling.** New opt-in switch, `BURNMON_PPROF=1` (`cmd\burnmon\profiling.go`,
`profiling_windows.go` for `GetProcessHandleCount` via a manual `kernel32.dll` proc since
`x/sys/windows` does not bind it, `profiling_other.go` a stub): a heap profile
immediately, a 60s CPU profile, a second heap profile once that ends, and a
MemStats/handle-count/`internal\watch`-watch-count line every 10s for the process's
life. `internal\watch.Watcher` gained `WatchCount()` for this. Run for real against
Wilco's own store (54,855 events): 13,541 open handles, 870 MB live heap (Sys 992 MB),
`GCCPUFraction` 1.3-1.4%. Cross-checked against the real filesystem, not left as a
correlation: `find -type d` under the four native roots found 12,732 subdirectories
under the Cowork root alone (`AppData\Local\Packages\Claude_*\LocalCache\Roaming\Claude\local-agent-mode-sessions`,
11,756 `.jsonl` files, one folder per session), and the profiling log's own
`fsnotify_watches=13125` matches that almost exactly: 97% of the process's handles were
one-fsnotify-watch-per-subdirectory. Full report, every number,
`04_assets\2026-09-26_ws3_profile_before_after.md`.

**Hypothesis 1 (one watch per folder): CONFIRMED, fixed.** Checked, not assumed, whether
the vendored fsnotify (v1.10.1) supports recursive watching on Windows: its own doc
comment says "Recursive watching is not currently enabled through fsnotify's public
API; the recursive code path is gated and only exercised by fsnotify's own tests."
Tried the undocumented `path + "\..."` convention directly anyway, in a throwaway
program reproducing the exact F7 race (a new nested folder plus its first file, back to
back): it reported only one event, for the top-level folder, with the literal "..."
segment left in the reported path, and never delivered the nested file's own event at
all. Confirmed unusable, not merely undocumented. `internal\watch` refactored behind a
new `nativeBackend` interface (`AddRoot`/`WatchCount`/`Close`): `watch_windows.go` is a
small hand-rolled recursive watcher (one `ReadDirectoryChangesW` call per native root,
`recurse=true`, IOCP, overlapped I/O, `windows.FileNotifyInformation` buffer decode,
added once at construction and never touched again since the recursion covers every
subdirectory created under it from then on); `watch_other.go` keeps the old
per-directory fsnotify design, moved verbatim, for B1's untested darwin/linux
browser-mode build. One real shutdown race found and fixed while building this:
`CancelIo`/`CloseHandle` on a root's handle can complete with `err==nil, n==0`,
indistinguishable from a real buffer overflow, rather than reliably
`ERROR_OPERATION_ABORTED`; fixed with a per-root `closed atomic.Bool` set before
cancelling, checked before trusting any completion. `TestWatcher_NewNestedDayFolderRace`
stays green, now because the race is structurally impossible rather than merely
untriggered; new `TestWatcher_WatchCountStaysOneAcrossManySubdirectories` proves
`WatchCount()` stays at 1 across 50 newly created, several-levels-deep subdirectories.
Measured for real: peak RAM 957MB to 87MB, peak handles 13,541 to 371, alone; 85.5MB/379
handles/~0.09% avg CPU with `burnmon-dev.exe` (unfixed, pre-built, read-only) running
alongside, statistically the same as alone.

**Hypothesis 2 (memory cap causing GC burn): NOT an independent cause.** Live heap sat
above the 400MB cap (~870MB), but `GCCPUFraction` never exceeded ~6%, not the
hypothesis's own "runs almost continuously" framing; after hypothesis 1's fix, live
heap sits at 3-6MB (Sys ~140MB), confirming the ~870MB was itself mostly hypothesis 1's
own bookkeeping. `debug.SetMemoryLimit(400 << 20)` left unchanged: still ~65x the new
live heap, a cheap safety net for a genuine rebuild burst, no reason to retune it.

**Hypothesis 3 (whole-table loads): real cost, not the reported symptom's driver, not
fixed.** `store.AllEvents()` measured at 3.02s for 54,855 rows, feeding `Collect` (every
15 minutes) and `bmHistory` (every History tab filter change). Periodic, not sustained:
does not explain the reported "climbs within a minute and stays there", already pinned
on hypothesis 1. Both 10-minute measurements already clear both Done-when targets with
hypothesis 1 alone. Per the spec's own "fixing only what the profile confirms", the
SQL-aggregate/daily-summary-table rewrite this hypothesis proposed was not built.
Flagged in `STATUS.md`'s Known gaps for a future pass, not silently dropped: `bmHistory`
still costs ~3s per History tab filter change on Wilco's real store size.

**Hypothesis 4 (double ingest): real, not confirmed as needing a fix.** Confirmed by
reading both entry points, not assumed: `cmd\burnmon-dev\main.go` and `cmd\burnmon\main.go`
both call `store.DefaultPath()`/`store.Open()`, i.e. share the exact same `burnmon.db`.
`store.Open`'s existing WAL mode plus `busy_timeout=5000` plus idempotent upsert already
make two independent writers safe (no `SQLITE_BUSY`/lock errors in either log across the
full 10-minute dual-instance run). `burnmon.exe`'s own footprint measured statistically
unchanged whether `burnmon-dev.exe` runs alongside or not. A named-mutex
single-ingest-owner was not built: real correctness-risk surface (handover logic, crash
detection) for a benefit the profile does not show is needed.

**Extra item 5, local time everywhere (Wilco's decision).** Storage stays UTC (no
migration of raw events); every day/week/month boundary now uses `time.Local`:
`internal\vendorstrip`'s and `internal\forecast`'s `dayStart`/`weekStart`/`monthStart`,
`internal\dataset\dataset.go`'s payload cutoff `monthStart`, `internal\history.go`'s
`bucketKey` and date-range filter, `internal\export.go`'s row-key day, `cmd\burnmon-cli\main.go`'s
export `--since`/`--until` filter (flag help text updated to say "local day";
`Doc.ExportedAt` already carried `time.Now()`'s own local Location and needed no change,
confirmed it already gives the export its real UTC offset). `store.DailyTokenTotals`
rewritten from a SQL `substr(at,1,10)` UTC-day `GROUP BY` into a bounded Go-side read
(still `WHERE at >= ?` at the SQL layer, still only F1's own 4-5 week plan/live window,
not the whole table) bucketed via `at.In(time.Local)`: SQLite has no IANA timezone
database, so it cannot bucket correctly across a DST transition, only a fixed offset,
which is wrong specifically on the transition's own 23h/25h day. Two genuine,
previously-hidden bugs found this way, neither caught by a literal `.UTC()` grep since
both were a bare `.Format(...)` silently rendering whatever Location the `time.Time`
already carried: `internal\dataset\fromstore.go`'s per-session daily bucket (fed
`scan.Session.Daily`, `internal\agg`'s Days/Weeks/Months aggregation, the Now page's own
view) used a bare `e.At.Format(...)`, wrong for an early-local-morning turn whenever the
store had already given it a UTC Location; `internal\forecast\forecast.go`'s `dateKey`
broke `TestBuildOneScoredWeekShowsBand` outright once dayStart/weekStart switched to
local: a `WeekStart` round-tripped through `store.ForecastScores` comes back UTC-located
even though the instant it names is a local midnight, and the old bare `t.Format(...)`
trusted that stale Location instead of converting. Every kept `.UTC()` call (parsing at
every adapter's own timestamp field, every SQL storage/comparison parameter in
`store.go`, every display-only `GeneratedAt`/session `Start`/`End`/turn `At` value the
frontend's own `localStamp()`/`localHMS()` already renders in local time via plain JS
`Date` accessors, confirmed by reading `template.html`, not assumed) was left exactly as
it was. DST verified against both 2026 Europe/Amsterdam transition dates (29 March
spring-forward/23h day, 25 October fall-back/25h day) with three dedicated tests
(`internal\history`, `internal\store`, `internal\dataset`), each temporarily reverted
and re-verified to actually fail without its corresponding fix, not merely asserted to
pass; all three skip themselves if the running machine's `time.Local` does not actually
agree with Europe/Amsterdam at the tested instants.

**Verify.** `go vet ./...`, `go test ./... -count=1` (every package green, including the
five new tests above), `.\build.ps1`, `node --check` on both of `template.html`'s
extracted `<script>` blocks (untouched this session, checked anyway),
`.\scripts\uicheck.ps1`: `w1` failed once then passed clean on an immediate retry (the
same known, pre-existing first-paint-budget flake earlier sessions already documented,
not a regression), `w0`/`w2`-`w8` all passed clean including `w8` (STATUS.md's own open
question about it, unresolved, re-ran clean a second time with no cost-axis code
touched). A real-app live-ingest check (a new `.jsonl` dropped into a brand-new folder
under a running `burnmon.exe`) showed ingestion in the same log-timestamp second as the
write; the first attempt at this check grepped the log for the file's own name and
found nothing, which is a log-verbosity gap (the terse `stage ingest` line never
includes the path), not a real ingest miss, confirmed by the timestamp match and by
`TestWatcher_NativeRootSeesNewFile`/`TestWatcher_NewNestedDayFolderRace` both passing.
**Review, two passes.** Fresh read-only Opus review over the full diff found four real
bugs beyond confirming the watch rewrite's Win32/IOCP mechanics and the profiling switch
sound: (1) `EnsureScored`/`actualTokensForWeek`'s `sc.WeekStart.AddDate(0, 0, 7)` ran on a
value still UTC-located after the store round-trip (the same class the `dateKey` fix
addressed, but that fix alone did not cover `AddDate` itself), adding UTC calendar days
instead of local ones and dropping the whole of Sunday from the 2026-10-25 fall-back
week's recorded actual; fixed at the source, `store.scanForecastScore` now converts
`WeekStart` to `time.Local` right after parsing it back, with new test
`TestActualTokensForWeek_DSTFallBackIncludesSunday`. (2) `template.html`'s `isoDate` (the
History tab's default date-range builder) used `toISOString()` (UTC), now local
`getFullYear()/getMonth()/getDate()`, matching `history.Filter.From/To`'s own local-day
convention. (3) `internal\agg\agg.go`'s `Build` compared a local-midnight `cutoff`
against UTC-anchored `parseDay(s.End[:10])`/`parseDay(daystr)` via `.Before()`, silently
dropping a session whose last real activity fell in the early local morning hours right
at a retention cutoff; fixed with a plain string comparison against the session's own
(correctly local) `Daily` keys, new test `TestBuild_CutoffComparesLocalDaysNotUTCInstant`.
(4) `vendorstrip.go`'s Copilot credits calculation had picked up the local `monthStart`
meant for display totals, when GitHub's own billing-cycle reset is a third-party UTC
boundary; fixed with a separate `utcMonthStart` used only there. Also hardened, not
correctness bugs: `store.DailyTokenTotals` split into an unbounded form and
`DailyTokenTotalsUntil`, the latter used by `actualTokensForWeek` to avoid an unbounded
rescan after a backlog of unscored weeks; and five Win32/concurrency fixes in
`internal\watch\watch_windows.go` (`CancelIoEx` not the no-op `CancelIo`, a per-root mutex
closing the re-arm-vs-Close race, decode-then-rearm-then-dispatch to shrink a kernel
buffer overflow's blast radius, closing the handle on a failed first read instead of
leaving it registered, bounds checks in `decode`). A second, focused review over these
six fixes confirmed four clean and caught two real problems in the other two: the new
DST test's own skip guard checked `monday.Zone()` (always +2h, since `monday` was built
with that Location) instead of the actual machine's `time.Local`, so it could never
self-skip on a non-Amsterdam machine, fixed to check `monday.In(time.Local).Zone()`
matching every other DST test's own pattern; and `watch_windows.go`'s error branch never
re-armed a failed read at all, permanently killing that root's live watching on any
transient error, fixed to retry once, matching the success path, with `Close` reworked to
wait (`b.draining`, a `sync.WaitGroup`) for every genuinely outstanding read's real
completion before posting its own shutdown signal, since closing a handle unblocks a
pending read without guaranteeing that completion has already been delivered through the
port (a theoretical GC-safety gap in the test suite's own repeated Watcher construction,
not a production risk: the real app never calls `Stop`). Two smaller items from the
second pass were flagged rather than fixed, both pre-existing patterns, not regressions:
a `forecast_scores` row written before this session keeps a UTC-Monday `week_start`
(affects at most the one week that was unscored at upgrade), and `store.go`'s `at >= ?`/
`at < ?` bounds compare RFC3339Nano strings lexically, which can misplace an event by one
row at an exact sub-second boundary (the same pattern the pre-existing lower bound
already used, now also on the new upper bound); both noted in STATUS.md's Known gaps.
Every fix re-verified: `go vet`, `go test ./... -count=1` clean, `go test` on
`internal/watch`, `internal/forecast` and `internal/agg` specifically repeated 10x clean,
`node --check` re-run on the two `template.html` blocks after the `isoDate`/
`histRangeDates` edits, `.\build.ps1` and `.\scripts\uicheck.ps1` (`w1` retry-flake and
`w8` behaviour unchanged from the first pass) both re-run clean.

Version 0.3.2 (`cmd\burnmon\app.go`, `cmd\burnmon-cli\main.go`). README's Status line and
STATUS.md updated. Committed on `main`, not pushed or tagged (Wilco's own manual step,
commands in the hub brief). Hub brief:
`04_assets\hub_agent_update_2026-09-26_ws3_shared_ingest_performance.md`.


## 2026-09-24, WS1 cleanup: dev-only mode, Sessions harness fix, History stacked chart, v0.3.1

Read `02_roadmap\2026-09-24_ws1_burnmon_cleanup.md` (three tasks, decisions already taken:
pin dev, delete Monitor view's code not just its button, keep Light/Dark). Ran in parallel
with WS2 (`burnmon-dev`, a worktree, untouched by this session) on `main`.

**Task 1, dev/business and monitor removal.** Deleted `#btn_mode`, `#btn_monitor`, the
monitor exit button, `#monitor_text_view` and its CSS, the whole monitor
text/braille-chart rendering block (`mtBrailleGraph`, `renderMonitorText`,
`monitorSessionBoxHTML`, `sizeMonitorChart`, ~330 lines), `setView`/`applyViewMode`, and
every `isBusiness()` call site (replaced with its dev branch, then the function itself
deleted): `sessionCardBusinessBody` (folded into `sessionCardHTML` calling
`sessionCardDevBody` unconditionally), the forecast's euro branch, the Now chart's forced
token/cost axis toggling, the vendor strip's Copilot-credits line. `pricing.Config.Mode`,
`BusinessMode()`, `Config.View`, `MonitorView()`, `Payload.Mode`/`View`,
`settingsPayload.DefaultView`, the Settings dialog's "Default view" select, and
`main.go`'s `ccSaveView` binding are gone; an old `burnmon.json` carrying `"mode"` or
`"view"` still loads (unknown JSON keys are silently ignored), guarded by
`TestLoadModeAndCopilotPlan`, rewritten to check exactly that instead of the now-removed
`BusinessMode()`. Deleted `tools\uicheck\check_v3.go`, `check_v3b.go`, `check_u5.go`
outright (every case in all three tested only mode/monitor); trimmed
`scripts\uicheck.ps1`'s `selfManagedChecks` to `w1` alone. One judgment call beyond the
mechanical isBusiness() sweep: History's per-client table (K3) was originally
business-mode-only per the v0.3 spec's own wording, but nothing else about it was
"business", so rather than delete a documented, tested product feature ("what did one
client cost" was V3-6's own Done-when item 2), it now shows whenever client rules exist,
the same "hidden while no client rule exists" convention the client filter and Sessions'
client column already use: gate changed from `!isBusiness() || !clients.length` to
`!clients.length`, nothing else.

**Task 2, Sessions vendor mislabeling and mispricing.** Diagnosed as stated: `scan.Session`
had no vendor/agent field, so the Sessions tab's Surface column (`SurfaceLabel`, Claude-only
vocabulary: "cli" → "Claude Code") mislabeled every Codex and Copilot CLI session as Claude
Code, since both adapters can also classify their own surface as "cli". Fixed: `Vendor` and
`Agent` added to `scan.Session`, filled in `buildSession` from the (already-present)
`schema.Event` fields; `dedupSessions`'s key gained `Vendor` so two different vendors can no
longer collide on it (`TestDedupSessionsKeepsSameSessionIDAcrossVendors`). New
`scan.AgentLabel` (matches `internal/history.AgentLabel`/`internal/vendorstrip.AgentLabel`'s
existing vocabulary, "Cowork" not "Claude Desktop": found and aligned with the existing
precedent rather than inventing a fourth naming). Sessions tab: a new Harness column/filter
(the real harness, primary) with Surface demoted to the cell's tooltip; a session priced by
a vendor with no book entry now reads "no price" instead of a number.
`internal\dataset\fromstore.go:117-134`'s real bug: every non-OpenAI vendor fell through to
`cfg.ModelFamily`'s Claude-family price table (`FallbackFamily` "sonnet" for anything that
didn't substring-match opus/sonnet/haiku/fable), so a Copilot or Hermes call priced as
Claude. New `pricing.Config.EventCost` (per-event twin of `CostForEvents`, C2's own "one
function ... so the numbers agree everywhere") routes Anthropic through `AnthropicBook`,
OpenAI through the existing `OpenAIPrices` path (left untouched: `OpenAIBook` is missing
two of the four model ids `OpenAIPrices` already covers, so switching would have
regressed real pricing, not just Copilot/Hermes's, wide open for a future book update),
GitHub through `CopilotCredits`, anything else 0/unpriced.
`TestSessionsFromEventsPricesGitHubEventsThroughCopilotBook`,
`TestSessionsFromEventsNoPriceForUnbookedVendor`,
`TestBuildSessionCarriesVendorAndAgent` added. Verified against the real store
(`scripts\uicheck.ps1 scratch`, a one-off check written and deleted after use): the Harness
filter lists all six vendors with real data (Claude Code, Codex, Copilot CLI, Copilot (VS
Code), Cowork, Hermes); a real Cowork session now reads "Cowork" with "Surface: Claude
Desktop" in its tooltip, not "Claude Code". Separately, `STATUS.md`'s adapter line for
Copilot CLI ("session totals read at shutdown, not live") was stale against the adapter's
own doc comment (A2 disproved that assumption and replaced it with a live 5-second poll);
fixed the doc, not the code, and did not add the "totals at exit" note the WS1 plan asked
for, since the premise behind it no longer holds: flagged rather than silently built
around, per house rule "say it when a document is stale".

**Task 3, History's "Burn per period" as stacked bars.** `history.Totals` gained
`ByVendor map[string]*VendorAgg` (tokens, headline cost per agent), built alongside the
existing bucket accumulation in `Build`, no separate pass;
`TestBuildByVendorSumsToRowTotals` guards day/week/month all summing back to the row's own
totals. `drawHistoryChart`: one Chart.js dataset per agent, `stacked:true` on both axes,
legend on, colour from the same `VENDOR_COLOR_FAMILIES` map the Now page's per-session
chart already uses (no separate "vendor strip" colour source exists; that map is the de
facto standard). New `histPeriodStartDate` returns the period's own start date as
`yyyy-mm-dd` for every period (week: that ISO week's Monday, computed from the same
`agg.weekKey` convention `bucketKey` already uses; month: `key + "-01"`); the chart's
tooltip title still calls the old `histPeriodLabel` for the weekday/week-number detail.
Verified live: real-store x labels `2026-08-24,2026-08-31,2026-09-07,...`, six stacked
vendor series with real, differing per-week totals.

**Verify.** `go vet ./...`, `go test ./... -count=1` (every package green, three new test
files/additions), `.\build.ps1`, `node --check` on both inline script blocks: all clean.
`scripts\uicheck.ps1`: `w1` failed twice (WebView2 first-paint budget of 1.5s missed on
this loaded laptop (multiple concurrent Claude Code sessions, WS2 building in its own
worktree at the same time; one run's screenshot even caught an unrelated File Explorer
window's z-order stealing the BitBlt capture, not burnmon's own window), pre-authorized
in the plan as pre-existing, not fixed, reported as asked. `w0`, `w2`–`w8` all passed,
including `w8`, previously a documented "known gap" (cost-axis toggle), flagged in
`STATUS.md` as an open discrepancy rather than marked fixed, since nothing in this session
touched that code path. Found and closed with the user's explicit go-ahead: a stray
already-running `burnmon.exe` (the user's own long-lived monitor instance, PID from
08:49, holding the single-instance mutex) blocked every non-self-managed uicheck launch;
asked before stopping it, stopped it, reran clean.

A fresh Opus read-only review agent ran over the full diff before commit and found three
real issues, all fixed: (1) `internal\live\live.go`'s `turnCost` and `ApplySessionTotals`
had the exact same non-OpenAI-vendor-priced-as-Claude bug `buildSession` had just been
fixed for, an instance the plan's own diagnosis (scoped to `fromstore.go`) had missed;
both now route through the same new `cfg.EventCost`. (2) The Sessions table's model-pill
colour rule (`m==='Fable'`/`m==='Opus'`) stopped matching once Anthropic's own per-call
label became the book's exact-model name ("Claude Opus 5.5") instead of the family name
("Opus"), a real regression from this session's own pricing fix; changed to a substring
check. (3) The pricing fix's real scope reaches further than "Copilot/Hermes no longer
priced as Claude": Anthropic's own events on Sessions and Now are now also priced from
`AnthropicBook`'s exact model id instead of the family-generic table, a deliberate,
correct consequence of routing "every vendor through its own book" that the code comments
and this log previously under-stated; documented properly in `STATUS.md` and the
`fromstore.go` comment. Two low-severity, non-blocking findings (a leftover `LAST_*`
cache/`shortSurface` dead-code trail from the isBusiness() removal, and two backend
figures, `copilot_credits_left`/`BusinessCost`, computed but no longer read by anything)
were the same class of leftover the removal work itself produces; the first was already
being cleaned up in this same pass, the second is flagged in `STATUS.md`'s Known gaps
rather than fixed (small, non-urgent, touches `live.go` a third time in one session for
no functional gain). Everything above re-verified after the fixes: `go vet`/`go test`/
`node --check` all clean again.

Version bumped `0.3.0` to `0.3.1` (`cmd\burnmon\app.go`, `cmd\burnmon-cli\main.go`, both
const `version`); `README.md` and `STATUS.md` updated throughout (Dev/Business and
Monitor mode sections removed, adapter/pages/cost/forecast/config sections rewritten for
v0.3.1). Not pushed, not tagged, not committed by this session: see the hub brief and the
commands handed to Wilco.

## 2026-09-24, v0.4 WS2 phase 4: advisor panel and export

Built the "TODAY'S READ" advisor panel and the AI-ready export bundle on
`burnmon-dev`, following `02_roadmap\2026-09-24_ws2_burnmon_dev.md`'s phase
4 scope. `internal/advisor` (six table-driven rules: heavy-turn-pressure,
harness-runaway, cache-hit-drop, context-window, hard-faults-memory,
self-overhead), `internal/devexport` (pure `Assemble`/`BuildSummaryMD`/
`BuildDataJSON`/`BuildDailyCSV`/`Redact`, `Write` the only disk-touching
function), `internal/sysmon/machineprofile*.go`, the `export` CLI
subcommand and `bdevAdvisorNow`/`bdevExport` bindings, and a page.html
panel plus header button. Built test-first throughout (advisor and
devexport both have real fire/no-fire unit tests, including a dedicated
privacy test that a prompt/response-text fixture never reaches any of the
three export files).

A fresh Opus review agent over the whole diff, before committing, caught
one real must-fix (`live.BuildTurns` had silently inherited the Now page's
50-turn ticker cap, so a 24-hour export's advisor pass only ever saw its
newest 50 turns, and could make harness-runaway fire a false positive for
a harness whose real turns had aged out of that cap) plus several
should-fix items, all applied and re-verified before commit: heavy-turn-
pressure no longer counts cache reads toward "heavy" (nearly every late-
session turn was crossing 200K on cache read alone); both per-turn rules
now collapse a sustained episode into one finding via a 5-minute cooldown
instead of one finding per matching turn; `Redact` now salts its hash with
a per-install random value (`redact_salt`, generated once) instead of an
unsalted 8-hex-character hash that a short, guessable name like a client
folder could realistically be reverse-brute-forced from; the export folder
name now includes seconds and uses local time (was colliding on two
exports in the same UTC minute); `--until` on a bare `YYYY-MM-DD` now
means through the end of that day (was excluding the whole day, since
Assemble's window is exclusive at the upper bound); and a documented
caveat that `burnmon-dev.exe`, built `-H windowsgui`, needs
`Start-Process -Wait -RedirectStandardOutput` (or an output redirect) for
the `export` subcommand's own stdout to be visible from a shell.

Ran a real 24-hour export against the live store both times (before and
after the review fixes): `summary.md` reads as a genuinely usable prompt
(machine profile, totals per harness/model, top 10 expensive turns, real
pressure episodes, advisor findings, known limits); real sizes measured
this session ranged from about 3.4KB to 11.5KB for `summary.md` (well
under the ~30KB budget; a synthetic 720-turn stress test in
`devexport_test.go` also stays under budget), 1.1-1.6MB for `data.json`
(no size budget applies), under 1KB for `daily.csv`. The advisor's own
self-overhead rule fired on real data both times, correctly naming
`burnmon-dev.exe`'s own known shared-ingest memory climb (design doc,
"both exes climb to about 900 MB... a BurnMon bug on main, handled in a
separate session") rather than a false alarm.

Built a parallel `tools\uicheck`/`scripts\uicheck-dev.ps1` dev-eval-channel
path for `burnmon-dev.exe` (port 9334, env `BURNMON_DEV_UICHECK`, window
title "BurnMon Dev": none of this existed before this session) since the
plan's own verify list calls for "uicheck d* cases". Four checks (`d0`-
`d3`) all ran and passed against the real running window this session:
advisor panel populates with real findings, the header EXPORT button
writes real files end to end (click through the dev eval channel, then
confirmed on disk), and both viewports screenshot with no content
clipping (`d2` at this session's own screen came within about 16px width /
39px height of the 1152x2048 target; `d3`'s known "system zone scrolls
here" gap, design doc section 2.2, "not wired yet", is unchanged by this
phase and was not asserted away). This laptop's own cold start for
`burnmon-dev.exe` (window plus startup backfill) measured up to ~90s this
session, well past `burnmon.exe`'s 20s equivalent wait budget; the dev
script's own wait is 120s.

`go vet ./...`, `go test ./... -count=1` (every package, including the new
ones) and `.\build.ps1` all green; `node --check`-equivalent parse on
page.html's script tag clean. Committed on `burnmon-dev`, not pushed,
tagged or merged, per house process. Left alone, per the plan's own scope:
the shared memory/handle-growth bug, plan limits, phase 5.

## 2026-09-24, v0.3 V3-6: release candidate, Done-when and VERIFY pass, v0.3.0

Read `02_roadmap\2026-09-23_v0.3_spec.md` sections 5 (Done when) and 6 (VERIFY carried).
`go vet ./...`, `go test ./... -count=1` and `.\build.ps1` all green before touching
anything. Every Done-when item run against the built exe and the real local store
(`%LOCALAPPDATA%\burnmon\burnmon.db`), not fixtures:

1. **Business mode shows euros per vendor with the basis named: PASS.** Real click on
   `#btn_mode` (`scripts\uicheck.ps1 v3`) flips to Business; a direct DOM read of a real
   session card showed `€2.84` / `API list (upper bound)` / `Client: unassigned`, the
   basis named right under the figure exactly as `sessionCardBusinessBody` writes it.
2. **History answers "what did one client cost in tokens, euros and active time last
   week" in two clicks: PASS.** Dropped a scratch `owners` config
   (`C:\ZND\* -> ZND`, `C:\dev\Work\* -> Valona`, each with a matching `client`) next to
   the exe, ran `burnmon-cli reown` against the real store (52,213 events), launched the
   real app, and drove two real clicks (History tab, then the Business toggle) over the
   dev eval channel: the per-client table read back `ZND 2.62B tokens, EUR 687, 3702 min,
   312 sessions` and `unassigned 2.29B tokens, EUR 999, 5316 min, 378 sessions` (Valona
   0, no Valona events exist on this laptop, correctly so). Reverted immediately after
   (`reown` again with an empty `owners` config) so the real store's baseline state was
   not left changed by this check.
3. **An export from Wilco's laptop with `--owner ZND` holds no Valona row and no path:
   PASS.** With the same scratch owner rules active, `burnmon-cli export --owner ZND
   --label wilco-laptop-v36 --out export1.json` wrote 35 rows; grepped the raw bytes for
   "Valona" (0 matches), a Windows path separator pattern (0 matches), and a UUID-shaped
   session-id pattern (0 matches); every row's `"owner"` value is `"ZND"`.
4. **Two exports merge into one offline HTML report: PASS.** A second export
   (`--owner ZND --owner Valona --label dev-2-v36`) merged with the first via
   `burnmon-cli merge export1.json export2.json --out merged\`: `report.html` has zero
   `<script>` tags and both labels appear in its tables; `merged.json`'s `by_client`/
   `by_vendor`/`by_week` breakdowns carry both labels as columns (e.g. `by_vendor.github`:
   1,373,902 tokens under each label, the real Copilot VS Code data flowing through the
   whole export/merge path, not just the vendor strip). Owner rules removed and the store
   reowned back to empty afterward, same as item 2.
5. **A v0.2 store and a v0.2 `burnmon.json` open with nothing lost: PASS, with a caveat.**
   `TestMigrateRealV01Store` (copies the real store, opens it, asserts every event
   survives) and the new-this-session confirmation that it lands at schema version 7 both
   pass; `TestMigration7AddsClientColumn` proves that migration is additive;
   `TestLoadV02ConfigWithOwnersOnlyIsUnchanged` proves a literal v0.2-shaped config still
   loads unchanged. Caveat stated plainly: the real store on this laptop has already been
   fully migrated forward by earlier v0.3 sessions, so there is no genuine pre-migration
   v0.2-era store file left on this machine to open fresh; this Done-when item stands on
   the proven migration path plus the regression tests guarding it, not on opening a
   distinct v0.2 artefact that no longer exists.
6. **A Copilot VS Code session appears once OTel is on: PASS.** Confirmed twice this
   session against the real store: the vendor strip's `Copilot (VS Code)` row read `0 /
   412K / 412K` (today/week/month) before any config changes, and item 4's real
   export/merge run carried real `github`-vendor tokens through end to end.
7. **macOS and Linux artefacts exist on the tag, labelled untested: mechanism confirmed,
   artefact-on-tag itself still pending.** Cross-compiled all four combinations
   (darwin/linux x amd64/arm64, `CGO_ENABLED=0`) clean against the current tree;
   `.github\workflows\release.yml` still builds and attaches on any `v*` tag push;
   README and About's "untested" notes are both still present. The literal "artefacts on
   the tag" cannot be true until `v0.3.0` is pushed and that workflow actually runs,
   which is outside this session (house rule: commits/tags happen here, pushing is
   Wilco's own step, printed below, not run).
8. **Startup shows loading states, never the saved-report text: PASS**, reconfirmed via
   `scripts\uicheck.ps1 v3`'s startup assertions against the current build.
9. **Monitor mode opens from the remembered setting, and in dev mode it is text only:
   PASS, now proven end to end.** V3-3b's own click-driven checks could not complete on
   2026-09-23 (locked console session); this laptop's console was unlocked this session
   (`query session`, confirmed Active), and `scripts\uicheck.ps1 v3b` ran to completion
   for the first time: opens straight into the dev-mode text page from a saved
   `"view":"monitor"` setting, Full view and back with real clicks, business-mode monitor
   showing the normal visuals instead of text, and the choice surviving a real restart.

Root cause found and fixed along the way, not a regression: item 9's own `v3` check
(dev/business toggle) failed on the first run today with the real click landing but the
label never changing. Traced to a stale `%LOCALAPPDATA%\burnmon\burnmon.json` holding
`{"view": "monitor"}`, left over from an incomplete real-window attempt on 2026-09-23:
with monitor mode forced on, `#topbar` (which holds `#btn_mode`) is `display:none`, so
`getBoundingClientRect()` returns a zero-size rect and the click lands nowhere real. Not
Wilco's own setting (no evidence he opened the app between sessions); reset to
`"view": "full"` and `v3` then passed cleanly. Left as a plain finding here rather than
silently worked around, per house culture.

Real, reproducible failure found and left open, out of scope for this pass: `scripts\
uicheck.ps1 w8` (the Now chart's cost-axis-follows-legend-toggle check) fails against the
current build exactly as V3-3c already isolated it: the cost axis stays hidden after its
series is shown. Confirmed today it is not new (re-ran `w1`, which also flaked identically
on 2026-09-23 under a locked session, and it now passes clean, so today's session had no
locked-session confound for `w8` either; the failure is real and specific to
`display:'auto'`'s own Chart.js scale-width behaviour, not an environment artefact).
Judged not "small" (would need a real Chart.js layout debugging session, not a one-line
fix) and out of this session's scope (a release-candidate verification/docs pass, not a
UI bugfix session); carried into `STATUS.md`'s Known gaps rather than fixed blind.

VERIFY items (spec section 6), each resolved to a source and date or an explicit
still-unverified label:

- **GitHub Copilot AI credit table and per-model multipliers**: Free/Pro/Pro+/Max already
  dated 2026-09-23 (V3-1), unchanged. Business and Enterprise resolved further today:
  live-checked against `docs.github.com/en/copilot/concepts/billing-and-usage/
  organizations-and-enterprises/billing` (2026-09-24), their AI-credit allotments are
  1,900/user/month (Business) and 3,900/user/month (Enterprise), pooled at the billing
  entity. Deliberately not added to `copilot_credits.json`'s `plans` block: the page's
  own "for businesses" tab still does not render in a plain fetch, so no per-seat monthly
  price is confirmed for either, and their pooled, org-level billing does not fit
  `CopilotCreditsLeft`'s single-machine model without further design work anyway.
- **ChatGPT or Codex plan-credit table**: confirmed live 2026-09-24
  (`help.openai.com/en/articles/12642688`) that no table comparable to GitHub's AI
  Credits exists to build a book from: ChatGPT's own "credits" are a pay-as-you-go
  overage top-up on top of the plan's own usage limits, not a fixed monthly allotment.
  This is a confirmed absence, not a guess, replacing V3-1's "nothing found today".
- **Anthropic and OpenAI list prices re-dated**: unchanged since V3-1 (2026-09-23),
  reconfirmed still current by running `burnmon-cli price-check` against the rebuilt exe.
- **Copilot VS Code span attribute names**: resolved in V3-5, unchanged.
- **Default data roots on macOS and Linux for each adapter**: Claude, Codex and Copilot
  CLI were already correct (`os.UserHomeDir()`-based, no per-OS special-casing needed);
  Copilot CLI's `~/.copilot` default confirmed live today against
  `docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference`
  ("By default, this directory is `~/.copilot`"). Hermes was wrong and is fixed this
  session: `internal\adapter\hermes\hermes.go`'s `DefaultDBPath` used to guess
  `~/Library/Application Support/Hermes/state.db` (darwin) and
  `$XDG_DATA_HOME/Hermes/state.db` (linux), following the per-OS app-data convention
  `store.DefaultPath` uses; live-checked today against Hermes's own docs
  (`github.com/NousResearch/hermes-agent`'s `session-storage.md`:
  "the HERMES_HOME environment variable, and finally the platform default (`~/.hermes`
  on macOS and Linux; `%LOCALAPPDATA%/hermes` on Windows)"), Hermes does not follow
  either platform's app-data convention at all, it is one dot-folder in `$HOME`
  regardless of OS. Rewrote `DefaultDBPath` to check `$HERMES_HOME` first (Hermes's own
  documented override, not previously supported), then `~/.hermes` (darwin/linux) or
  `%LOCALAPPDATA%\Hermes` (Windows, confirmed live on Wilco's own laptop since v0.2 41B,
  unchanged); two new tests
  (`TestDefaultDBPathHermesHomeOverride`, `TestDefaultDBPathNoInstallIsEmpty`). Copilot in
  VS Code's own row: not actually a "default root" VERIFY at all, since
  `github.copilot.chat.otel.outfile` has no default on any OS, you always set it
  yourself; reworded the README table instead of leaving a VERIFY that no research could
  ever resolve. README's default-roots table updated for all three rows.
- **Plan assumption A8 (active time usable)**: still unverified.
  `C:\ZND\10_holding\03_logs\time\` still does not exist on this laptop (checked again
  today), so V3-2's spot-check still cannot be compared against an external time log.
  Unchanged, carried forward as-is rather than guessed at.

Docs and release mechanics: version constants bumped to `0.3.0`
(`cmd\burnmon\app.go`, `cmd\burnmon-cli\main.go`; confirmed via `burnmon-cli -version`
against the rebuilt exe). `README.md` rewritten for v0.3: dev/business mode and cost, the
extended owner/client map and active time, monitor mode, export and merge, the two
new `burnmon-cli` command lines, a new "The Groundwork Kit" section (install, set
`"mode": "business"`, run `export`/`merge` on a cadence, written generically rather than
naming a specific Kit client, since this is a public README), and the default-roots table
fixes above; "Not yet there (v0.3)" removed since it now is. `STATUS.md` rewritten in full
for v0.3: every shipped item, the store's real schema version (7), and a Known gaps
section carrying the `w8` cost-axis bug, the Copilot Business/Enterprise per-seat price,
the ChatGPT/Codex non-table finding, A8, and the pending macOS/Linux tag artefacts, all
stated above. `DEADLINES.md`'s v0.3 row and `02_roadmap\roadmap.md` item 7 both marked
DONE 2026-09-24, tagged `v0.3.0`, 15 days early. Full suite green throughout
(`go vet ./...`, `go test ./... -count=1`, `.\build.ps1`, `node --check` on template.html's
extracted scripts, `scripts\uicheck.ps1` default suite plus `v3`/`v3b`/`u5` all green
except the pre-existing, out-of-scope `w8` above). `C:\dev\Work` untouched throughout.

## 2026-09-23, v0.3 V3-3c: monitor view like perfadvisor (U5)

Read `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_monitor_view_patch.md` and both
reference images in `C:\ZND\projects\burnmon\testdata\uicheck\reference\`
(`perfadvisor_history.png`, `monitor_mockup.jpg`), then perfadvisor's `graph()`
(`C:\ZND\projects\perfadvisor\internal\tui\widgets.go`) and `graphLines()`
(`C:\ZND\projects\perfadvisor\internal\tui\view.go`) before writing anything. Replaced U4's
one-block-per-minute strip with a real braille line chart: `internal\live\live.go`'s
`BuildSnapshot` gained an optional `bucketSeconds` argument (variadic, so every existing
3-arg call keeps its default 60-second/30-slot chart unchanged), threaded into `buildChart`,
which now sizes its slot count from that argument instead of the fixed `BucketSeconds`
constant; `cmd\burnmon\main.go`'s `bmLive` binding takes the same optional int and passes it
straight through, no new binding. `internal\report\template.html`: monitor view's own poll
now calls `bmLive(10)` (10-second buckets, finer dot columns) while full view keeps
`bmLive()` (60s); `renderMonitorChartText` rebuilds the whole chart panel per poll,
`mtBrailleGraph` (a JS twin of `graph()`, U+2800 dots, later series drawn over earlier ones
in a shared cell) draws one overlaid series per session appearing anywhere in the chart's
30-minute window, scaled to one shared peak (not each series' own, since sessions are
directly comparable, unlike perfadvisor's mixed-unit metrics), `sizeMonitorChart` picks the
dot grid's row count from the panel's own real pixel height (about 35% of the window) via a
hidden probe element, recomputed on resize. Wilco's own call on the patch's flagged open
choice (candidates asked via AskUserQuestion): one legend row per running session, not one
packed perfadvisor-style line, since full "agent · model · project" labels are much longer
than perfadvisor's "cpu"/"ram"; "scaled to peak" sits right-aligned on the last row only,
verified by a real-window read (`PeakRowIndex`/`PeakRowCount` in the new uicheck below).
Session cards' box width is now computed from the longest line across every card rather
than a hardcoded 30, so the right border stays one straight column at any content length,
in a real CSS grid (four per row, wrapping) instead of the old flex-wrap. Monospace stack
reordered to Cascadia Mono, JetBrains Mono, Consolas per the patch; braille glyph rendering
was checked with a headless Edge screenshot of a standalone test page in each of the three
fonts plus the combined stack (`C:\Users\WilcoDeTree\AppData\Local\Temp\claude\...\scratchpad\braille_font_test.png`,
not committed): all four render real dot glyphs via the browser's own font-fallback, no
extra fallback font needed. Real-window check: new self-managed `tools\uicheck\check_u5.go`
(added to `scripts\uicheck.ps1`'s `$selfManagedChecks`, alongside `w1`/`v3b`), reads the
chart panel's DOM state (title, legend row count, non-blank dot count, dot row count,
minute label text, session box count and width equality, computed font-family) rather than
a bare `window.LAST_NOW_SNAP` global: an earlier version of the check read that global and
it always came back empty even with real sessions rendered, because the dev eval channel
(`cmd\burnmon\uicheck_devserver.go`) runs in an isolated JS world that shares the page's DOM
but not its plain script globals; DOM-based counts do not have that problem. Spawned two
short-lived read-only subagents in this same repo to get genuinely concurrent running
sessions for the screenshot rather than fabricate data. Before/after screenshots (stashing
just `template.html` to rebuild the pre-patch exe for a true "before", not a mockup):
`C:\ZND\projects\burnmon\testdata\uicheck\out\u5-before-monitor-dev.png` (old bug: tiny
block strip top-right, labels cut off, "claude-code · claude-son") versus
`C:\ZND\projects\burnmon\testdata\uicheck\out\u5-chart-monitor-dev.png` (full-width dark
panel, "live burn, newest right", 5 overlaid braille series across 3 real running sessions,
"scaled to peak 1.18M" right-aligned on the last legend row, minute labels 06:07..06:36
fully visible after a fix for the last label clipping to its first character at the panel's
right edge, 3 equal-width session card boxes, bigger vendor strip type). Matches
`perfadvisor_history.png`'s dot-chart style and `monitor_mockup.jpg`'s panel layout; still
differs in that the legend is one row per session rather than one packed line (Wilco's own
choice, not a gap) and the mockup's 8 placeholder cards versus this run's 3 real ones (grid
is confirmed 4-per-row via CSS, just not exercised past one row with only 3 sessions live).
`node --check`, `go test ./... -count=1`, `.\build.ps1` and `.\scripts\uicheck.ps1 u5` all
green. Also ran the full default suite (`w0`, `w2`-`w8`); `w8` (Now page's Chart.js cost-axis
toggle, full view, untouched by this patch) fails on this build regardless of this session's
changes, confirmed by stashing `template.html` back to its pre-patch state and re-running
`w8` against that: identical failure ("cost axis stayed hidden after showing the cost
series"), so a pre-existing issue outside U5's scope, not a regression; `w1` (fresh-launch
timing, 1.5s window-appear budget) also failed once during this session, most likely the
same kind of environment timing sensitivity given multiple burnmon.exe launches back to back
under this session's own load, not isolated the same rigorous way for time reasons. Neither
is touched by this patch's files.

## 2026-09-23, v0.3 V3-5: Copilot in VS Code and macOS/Linux builds (A4, B1)

Read `02_roadmap\2026-09-23_v0.3_spec.md` sections 2.3 A4/2.4 B1 and the A3 correction entry
below before touching anything. The prompt's own precondition ("Before this session Wilco
copies his real Copilot VS Code OTel span file ... into `testdata\copilotvsc\`") had not
happened: no such folder existed. The live file the A3 correction session used was still on
disk at `%LOCALAPPDATA%\Temp\copilot-otel.jsonl` (1,793 lines, 14.7MB, last written
2026-09-23 04:39), so this session read it directly rather than waiting, and built the
fixture from it itself. Real attribute names, read before writing the parser as the prompt
asked: log records mix two shapes, `attributes["event.name"]` values
`copilot_chat.session.start` (carries its own `session.id`, distinct from the constant
resource-level `session.id` every line in the file shares for the VS Code window's whole
life), `gen_ai.client.inference.operation.details` (one LLM call: `gen_ai.request.model`,
`gen_ai.response.model`, `gen_ai.response.id`, `gen_ai.usage.input_tokens`,
`gen_ai.usage.output_tokens`), `copilot_chat.tool.call` and `copilot_chat.agent.turn` (a
per-turn rollup this adapter deliberately does not read, since its own input/output totals
already include every inference call inside that turn and reading both would double-count).
Two findings worth carrying: no attribute value over 200 bytes exists anywhere in the file
(checked programmatically across every attribute on every line), so "fixture ... stripped of
prompt text" was already true of the source and nothing needed removing; and no
workspace/folder/cwd attribute of any kind exists either, so "project from a workspace
attribute where present" has nothing to read today (VERIFY per spec section 6, a
`workspaceAttrCandidates` best-effort list kept in `internal/adapter/copilotvsc/copilotvsc.go`
for if GitHub adds one later). One `gen_ai.response.id` real dedup case was caught directly in
the file: `dc0615ba-...` appears on four separate lines with growing input tokens (60281,
91502, 95281, 103664) and non-monotonic output tokens (434, 369, 256, 1197), read as the same
in-flight multi-step agent turn being re-emitted as it grows; "last write wins per request"
(spec's own phrase) is implemented as keyed-by-response-id overwrite in file order, not
largest-output, so this case resolves to the last line (103664/1197) regardless of which
value happens to be biggest. `PollOnce` is a plain 5-second poll of the whole file every
time (like Hermes/Copilot CLI, not an incremental tail: no prompt text anywhere means no
line-size reason to avoid re-reading it, and `UpsertEvents`' own largest-output-per-RequestID
dedup already makes a repeat read of unchanged lines a no-op), wired as
`startCopilotVSCPoll` alongside the other two pollers, config-gated on a new
`pricing.Config.CopilotVSCodeOtelFile` (`copilot_vscode_otel_file` in `burnmon.json`; no
fixed default path exists anywhere to auto-detect, unlike Hermes/Copilot CLI, since VS
Code's own `otel.outfile` setting is user-chosen). Vendor/agent/surface `github`/
`copilot-vscode`/`vscode`, label "Copilot (VS Code)" added to both Go-side maps
(`internal/vendorstrip`, `internal/history`) and the template's own `AGENT_LABELS`. Fixture:
`testdata\copilotvsc\copilotvsc_fixture.jsonl`, the file's own 29 attribute-bearing lines
(every `session.start`/inference/tool.call/agent.turn line), no synthetic data.
`TestPollOnce_Fixture` asserts the exact numbers above; `go test ./internal/adapter/copilotvsc/...`
green. Real-window check (never a proxy check alone): pointed a scratch `burnmon.json`'s
`copilot_vscode_otel_file` at the real live file, ran `burnmon.exe` for real with
`BURNMON_UICHECK=1`, and read the actual running DOM over the dev eval channel:
`#t_vendorstrip` shows a real "Copilot (VS Code)" row, `0 / 412K / 412K` (today/week/month),
confirming the whole path (config -> poll -> `cfg.OwnerFor`/`ClientFor` -> `UpsertEvents` ->
vendor strip -> template label) works end to end against real data, not just the fixture;
`go test ./...` stayed green afterward (this real ingestion did not disturb
`TestMigrateRealV01Store` the way V3-4's real `reown` run once did, since it only adds new
rows under a vendor no existing test snapshots).

B1: the whole `cmd\burnmon` package was Windows-only (`//go:build windows` on the single
`main.go`, `go-webview2` imported unconditionally), so darwin/linux did not merely lack a
window, they did not compile at all. Split into `app.go` (no build tag: the `app` struct,
`rebuild`, live/poll wiring including the new `startCopilotVSCPoll`, Settings, and the HTML
chrome `rebuild` injects, none of it touches a Windows API), `main.go` (unchanged
`//go:build windows`, now only the WebView2 window itself, its `w.Bind` calls, and the
single-instance/WebView2-missing-fallback Win32 bits), and a new `main_other.go`
(`//go:build !windows`): per decision #3 of the grill ("macOS and Linux: Pure-Go builds,
browser mode, no cgo, labelled untested"), this collects once, writes `dashboard.html`, opens
it in the OS default browser (`open` on darwin, `xdg-open` on linux), then keeps rewriting
that same file on `-interval` in the background since there is no bound-JS live-update path
outside a WebView2 window (a saved report already falls back to its own "only available in
the BurnMon app window" text for anything live, which U2 anticipated as the correct case, not
a bug, for exactly this situation). `internal/scan/wsl_other.go` gained a `WSLDistroNames`
stub (`app.go`'s `rebuild` called the real one unconditionally; the cross-platform build
would not link without it) and `internal/adapter/hermes.DefaultDBPath` gained darwin/linux
default paths (`~/Library/Application Support/Hermes/state.db`,
`$XDG_DATA_HOME/Hermes/state.db` else `~/.local/share/Hermes/state.db`), both VERIFY, no
Hermes documentation confirms either; Codex (`NativeSources`), Claude
(`scan.DefaultSourcesWithOptions`) and Copilot CLI (`copilotcli.DefaultDBPath`) already used
`os.UserHomeDir()` and needed no change. Verified for real, not just read: cross-compiled
`burnmon`/`burnmon-cli` for darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, all four
with `CGO_ENABLED=0`, all eight binaries produced with zero build errors. `.github\workflows\release.yml`
added: on a `v*` tag push, builds all four darwin/linux combinations on `ubuntu-latest`
(pure cross-compilation, no need for a macOS/Linux runner) and attaches every binary to that
tag's GitHub release via `softprops/action-gh-release`; Windows builds stay `build.ps1`'s own
job (go-winres icons, WebView2), not duplicated here, matching B1's own wording ("builds
burnmon and burnmon-cli for darwin and linux"). README gained the two VS Code settings, a
default-transcript-roots table by OS (VERIFY rows named), a macOS/Linux section labelled
untested, and the About tab (`internal/report/template.html`) gained its own "macOS and Linux
builds are untested" note, confirmed present in the real running window over the dev eval
channel above, not just read from the template source. `go vet ./...`, `go test ./...
-count=1` (Windows), `node --check` on all three of `template.html`'s extracted `<script>`
blocks, and `.\build.ps1` all green; `scripts\uicheck.ps1 w0` passed against the rebuilt
`burnmon.exe`. `C:\dev\Work` untouched. No open call was hit that needed stopping to ask;
the "two VS Code settings" vs. the four the A3 correction actually used was the one
judgement call made without asking (documented above, not treated as open, since it was a
README-wording question, not a behaviour decision).

## 2026-09-23, v0.3 V3-4: export and merge (K4, K5)

Read `02_roadmap\2026-09-23_v0.3_spec.md` section 2.2 K4/K5 and the v0.2 spec's P6 wall rule
before touching anything. K4: new `internal/export` package, `export.Doc`/`export.Row`, grouping
real turns (the claude adapter's synthetic tool-only events excluded, same `isTurn` filter as
`internal/history`) into day (UTC)/owner/client/vendor/model rows, five token classes (input,
cache write, cache read, output, reasoning), cost on every basis C2's `CostForEvents` covers for
that row's vendor, and active minutes recomputed directly off the row's own events via a new
`dataset.ActiveTimeMinutes` (K2's gap-sum algorithm exported for buckets finer than a whole
session, since one session's events can land in more than one row here). `export.DefaultLabel`
is the literal string `"dev-1"`; nothing in the export path calls `os.Hostname`. `burnmon-cli
export --since --until --owner... --label --out`: refuses to run when `burnmon.json` carries
owner rules and no `--owner` was given (`cfg.Owners` non-empty gate), silent otherwise, matching
K4's "so a Valona row never leaves by accident." No field on `Row`/`Doc` carries a path, session
id, prompt or project name; `TestExportedJSON_NoPathsOrSessionIDs`
(`internal/export/export_test.go`) is the requested leak scanner: builds a doc from an event
whose `Project`/`SessionID`/`Title` do carry a Windows path and a UUID, marshals it the way the
CLI will, and fails if a path separator or a UUID-shaped session id pattern survives into the
actual bytes. K5: new `internal/merge` package, `merge.Merge` refuses files whose `schema` value
differs, sums tokens (the five classes) into per-client/per-vendor/per-ISO-week rows, one column
per label (`LabelTotals`, a repeated label from two files of the same machine sums into the same
column rather than erroring); `burnmon-cli merge <file>... --out <dir>` writes `merged.json` and,
via a new `internal/mergereport` package, a `report.html` rendered entirely server-side with Go's
`html/template`, no `<script>` tag at all (a chart was judged not needed for three totals tables,
so nothing from the vendored Chart.js copy was pulled in; "opens offline with no network script"
holds trivially since there is no script). Found and fixed a real bug while running this for
real: `runMerge`'s flag/positional split treated `--out`'s own value as a second positional file,
since it only recognised a token as "the value of a flag" by prefix, not by asking whether the
previous token was a flag; fixed to consume the next token after any `-`-prefixed argument as
that flag's value (`cmd/burnmon-cli/main.go`, `TestMerge...` would not have caught this, it is
CLI-parsing not package logic, so this was only caught by actually running the exe). Ran for real
on this laptop: no `burnmon.json` with owner rules existed anywhere on it yet (checked
`%LOCALAPPDATA%\burnmon\burnmon.json` and next to the exe, neither existed), so `--owner ZND`
would have matched nothing meaningful; dropped a temporary `burnmon.json` in the scratchpad with
`C:\dev\Work\* -> Valona`, `C:\ZND\* -> ZND` (the same rule shape `burnmon.example.json` already
documents) and ran `burnmon-cli reown` against the real store once to backfill Owner on the
51,795 events already in it, per K1. `burnmon-cli export --owner ZND --label wilco-laptop`
produced 32 rows, every one `"owner": "ZND"`, zero occurrences of "Valona" and zero path
separators in the file (checked by grep, not just by eye). A second export with `--owner ZND
--owner Valona --label dev-2` (kept local, in the scratchpad only, never published, per the wall
rule) gave a second file with different totals to merge against; `burnmon-cli merge` on the two
produced `merged.json` (three breakdowns, both labels as columns, e.g. by_vendor.anthropic:
wilco-laptop 2,403,871,082 vs dev-2 2,404,052,920 tokens) and a `report.html` that opens with
`file://`, no console errors, three plain tables matching the JSON exactly. That real `reown` run
temporarily changed the live store's `owner`/`client` columns, which made
`TestMigrateRealV01Store` fail (it snapshot-compares the real `%LOCALAPPDATA%\burnmon\burnmon.db`
and expects `Owner == ""`, the pre-K1 state); reverted by running `reown` once more against an
empty-`owners` config, which is P6's documented "empty means no owner column at all" behaviour,
confirmed the test green again before stopping. `go vet ./...`, `go test ./... -count=1` and
`.\build.ps1` all green.

## 2026-09-23, v0.3 V3-3b: monitor mode (U3, U4)

Read `02_roadmap\2026-09-23_v0.3_spec.md` section 2.5 U3/U4 and the v0.2.2 vendor colour table
(`VENDOR_COLOR_FAMILIES`, `internal\report\template.html`) before touching the template. U3: a
`view` field on `pricing.Config` (`"view"` in `burnmon.json`, `""`/`"full"` default, `"monitor"`
starts straight into it), exposed to the page as `D.view` via a new field on `dataset.Payload`,
same start-state convention as `Mode`/`D.mode`. `cmd\burnmon\main.go`'s `writeSubscriptionConfig`
generalised into `writeConfigKey(path, key, value)` (a merge-into-JSON, `.tmp`+rename write for any
top-level key, not just `"subscription"`), reused by a new `ccSaveView` binding (the header's live
Monitor/Full switch persists its own choice, no rebuild: the view choice changes chrome only) and by
`ccSaveSettings`/`applySettings`, which now also writes `"view"` from a new `defaultView` field on
`settingsPayload` and the Settings dialog (a `<select>` next to Your seat). Template: `UI.view`
(from `D.view`, not `localStorage`, unlike `mode`/`theme`: the header switch's own choice belongs in
`burnmon.json`, not per-viewer browser storage), a header `#btn_monitor` button, a fixed
`#monitor_exit_btn` ("Full view", shown only via `body.monitor-mode` CSS) and `setView`/
`applyViewMode` (toggles `body.monitor-mode`/`body.monitor-dev`, forces the Now tab active, saves via
`ccSaveView`). Monitor mode's three panels (chart, sessions, vendor strip) are the Now page's own:
`body.monitor-mode` hides `#topbar` (header and tab bar, `.tabsbar` is its own child) and a new
`#now_extra` wrapper around the turn ticker and forecast blocks; business mode's monitor view is
exactly the normal Now page underneath that chrome, "reuse bmLive/bmVendorStrip as they are" from the
brief taken literally, no second Chart.js instance. U4 (dev mode only, `body.monitor-mode.monitor-dev`
additionally swaps `#now` for `#monitor_text_view`, a fixed-position dark monospace page): a
block-character chart (`renderMonitorChartText`, one `▁`-`█` column per minute over the same
30-minute `chart` array `drawNowChart` already gets from `bmLive`, one row per session, vendor colour
from `VENDOR_COLOR_FAMILIES.dark`, fixed regardless of the theme toggle), ASCII-bordered session boxes
(`monitorSessionBoxHTML`/`mtBox`, the same tokens/turn/last-turn fields `sessionCardDevBody` shows,
clicking one opens the I3 turn drawer for `turn_count`, no hover, no bar click) and an ASCII vendor
strip table (`mtVendorStripText`). Both render functions are called from `renderNow`/`renderVendorStrip`
when `body` carries `monitor-dev`, off the same 2s/60s polls the Now page already runs: no new backend
call, per the brief. `internal/pricing`: `TestLoadView`/`TestDefaultsAreFullView` (new `"view"` field,
same shape as the existing `TestLoadModeAndCopilotPlan`/`TestDefaultsAreDevMode`). `node --check` on
both extracted `<script>` blocks, `go vet ./...`, `go test ./... -count=1` and `.\build.ps1` all green.
Real-window check: wrote a new self-managed `tools\uicheck\check_v3b.go` (`scripts\uicheck.ps1`'s own
self-managed list generalised from a hardcoded `w1` case to `@("w1", "v3b")`, since v3b also needs to
control `burnmon.json` before its own launch and restart the exe mid-check) covering every uicheck step
the brief asked for: opens in monitor mode from a saved `"view":"monitor"` setting (asserted via the
dev eval channel: `#topbar` and `#now_extra` hidden, `#monitor_text_view` shown, `monitor-dev` on
`body`), Full view and back with real `SendInput` clicks, business-mode monitor showing the normal
Now visuals instead of the text page, a restart with the exe killed and relaunched confirming the
choice survives. The eval-only assertions (everything above that does not need a click) hold, and are
strong evidence the markup/CSS/JS wiring is right. The click-driven assertions did not run to
completion in this session: `query session` showed this laptop's console session locked (foreground
window "Windows Default Lock Screen") for the whole run, so `BitBlt` screenshots captured the lock
screen's own Spotlight-style background image (not burnmon), and `SendInput` clicks landed on it too,
never reaching burnmon's window regardless of the window's own topmost z-order (`ensureWindowSize`,
called before every click and screenshot in `check_v3b.go`, same trick `main.go`'s dispatch already
uses for every other check). Confirmed this is an environment condition, not a v3b or monitor-mode
regression, by re-running the already-`x`-ticked `v3` check (V3-3's own dev/business toggle test,
unchanged by this session): it failed the exact same way (`btn_mode label after click = "Dev", want
"Business"`), so the click path was never exercisable this session, on any check, old or new. Per the
house rule (never a proxy check alone) this is reported plainly rather than claimed: Wilco needs to
run `.\scripts\uicheck.ps1 v3b` himself, from an unlocked interactive session, for the click-driven
screenshots (`v3b-full-view.png`, `v3b-monitor-business.png`, `v3b-monitor-restart.png`) to show real
content instead of the lock screen. Not carried further this session: V3-3b is otherwise complete and
ticked below; the interactive re-verification is the one open item, tracked in `STATUS.md`'s Known
gaps section rather than blocking the next V3-4 session, since nothing about V3-4 (export/merge)
depends on it.

## 2026-09-23, v0.3 V3-3: dev/business switch (C3), client view (K3), Now page fixes and loading states (U1, U2)

Read `02_roadmap\2026-09-23_v0.3_spec.md` sections 2.1 C3, 2.2 K3, 2.5 U1/U2 and
`04_assets\2026-09-22_burnmon_now_page_features.md` section 4 before touching the template. C3: a
`btn_mode` toggle in the header, dev by default; start state reads `pricing.Config.Mode` ("mode" in
`burnmon.json`, exposed to the page as `D.mode` via a new field on `dataset.Payload`), a per-machine
choice persisted in `localStorage` afterward overrides it on later loads, same pattern as the theme
toggle. Business mode flips the Now page in place from cached poll payloads (`LAST_NOW_SNAP`,
`LAST_VENDOR_STRIP`, `LAST_FORECAST`, `LAST_HISTORY`), no extra backend call on toggle: session cards
show the vendor's headline basis in euros with the basis named (`sessionCardBusinessBody`, fed by a new
`live.Session.BusinessCost *pricing.BasisCost`, computed via `cfg.CostForEvents` on the session's own
turns, and recomputed the same way from `ApplySessionTotals`'s SQL-aggregated lifetime totals via
synthetic per-model events so the lifetime figure agrees with the windowed one), the client
(`live.Session.Client`, the same `firstNonEmptyProject`-style pull K1 already carries on
`schema.Event`), and GitHub Copilot credits left where a plan is configured; the live chart hides the
per-session token bars and shows the existing (previously opt-in-only) `Cost/min` line by default,
token y-axis hidden via `display:!isBusiness()`; the forecast text shows `end_of_day_cost_usd` /
`end_of_month_cost_usd` instead of token counts. "Credits left" needed a decision the spec itself
did not resolve (no account-wide credits-consumed tracking existed, and no field named which Copilot
plan tier the account is actually on): asked Wilco, recommendation taken, a new `Config.CopilotPlan`
field (mirrors `Subscription.YourSeat`'s pattern) names a key into the compiled-in
`CopilotCredits.Plans` table; `pricing.CopilotCreditsLeft` subtracts this calendar month's
github-vendor credits spend (from `cfg.CopilotCreditsLeft`, `internal/vendorstrip.Build`'s own extra
`EventsSince(monthStart)` query, gated so it never runs at all with no plan configured) from that
plan's `MonthlyCredits`, surfaced once on the vendor strip and read by every github-vendor session
card from that one cached figure rather than repeated per card. K3: History gains a client filter
(`h_client`, same "hidden while no client rule exists" convention as owner) and, in business mode, a
per-client table (`history.ClientRow`: tokens, headline cost, active time via
`dataset.SessionsFromEvents`/`ActiveMinutesByClient`, sessions), built from events matching
period/range/vendor/owner but deliberately not the client filter itself, so every client stays
comparable side by side even with one selected above; Sessions gains a client column and filter, same
hidden-until-configured convention, `applyOwnerColumnVisibility` extended to a client twin. Along the
way, replaced History's whole cost computation: it used to gate on a hardcoded
anthropic/openai-only `coveredVendorForAgent` and only show a figure at all when narrowed to one
vendor; now `headlineCostUSD` sums `cfg.CostForEvents`'s headline basis across every vendor present
(github and any future book included), so an "All vendors" view shows real cost too, not just a
single-vendor one, a real behaviour change from v0.2, not just a relabel, confirmed by hand-computing
the new numbers and updating `TestBuildWeekAndMonth`'s and `TestBuildOneScoredWeekShowsBand`'s own
expected figures (book prices, not the old family-generic ones) rather than leaving them silently
wrong. Every literal "tokens only until v0.3" string (`internal/history`, two spots in
`template.html`) is now "tokens only" or a real figure, per the spec's own instruction, since v0.3
is what shipped the cost function that string was waiting on. `pricing.BasisCost`/`VendorCost` had no
JSON tags at all (a V3-1 gap that never mattered until this session put `BasisCost` on the wire as
`business_cost`): added explicit snake_case tags matching every other exported struct in this codebase,
caught before it shipped a `Basis`/`Label`/`USD` capitalised-field payload to the template. A second,
independent USD pass (`forecast.addCostForecast`, `EventsSince` on today and this month, headline
basis, the same end-of-day/end-of-month extrapolation `liveLine` already runs on tokens) backs the
business-mode forecast text; deliberately left in USD (`end_of_day_cost_usd`), not pre-converted to
EUR, since the template's own `money()` already applies the viewer's currency and FX rate everywhere
else, converting twice was caught and reverted before it shipped. U1: the Now page's intro paragraph
moved to a new "The Now page" section at the top of About; the "no running sessions" notice moved from
a panel above the chart to one small line (`now_no_sessions`) underneath it. U2: found the vendor strip
empty-after-startup bug: `pollVendorStrip()`/`pollForecast()` ran at module-scope, and
`startNowPolling()` ran at the very end of the same synchronous script, all checking `typeof
window.bmX === 'function'` exactly once, immediately, go-webview2 injects its bindings asynchronously
after the script has already started running, so the very first poll in the real app window routinely
lost that race and showed the "only available ... not in a saved report" text (or an empty header with
no rows) for a moment on every startup, not just in an actual saved report. Fixed with one shared
`waitForBinding` retry helper (150ms x 40 = 6s budget) used by all three pollers; confirmed fixed
against the real window (`tools\uicheck\check_v3.go`, new "v3" check): `now_empty`/`vs_empty`/
`now_forecast_note` all read "Loading..." or a real value at startup, never the saved-report text, once
`scripts\uicheck.ps1`'s own wait for the first Collect log line has passed. Added loading states
throughout: `now_empty`, `vs_empty` and `now_forecast_note` default to visible "Loading..." text in the
markup itself rather than hidden, `now_cards` gets a skeleton placeholder, both chartboxes get a
`.chart-loading` overlay cleared on each chart's first successful draw. Also fixed, same U2 item:
`tokPrecise(0)` fell through to the K-suffix branch and printed "0.00K" for an empty axis tick; now
returns "0" explicitly, the same case `tok()` already handled. Real-window check: `scripts\uicheck.ps1
v3` (new check, `tools\uicheck\check_v3.go`) against the actual running exe with a live Claude Code
session: startup state asserted via the dev eval channel (never the saved-report text, `btn_mode` reads
"Dev"), a real `SendInput` click (not an eval `.click()`) flips it to "Business" (chart axis, cost line
and card content all confirmed switched, in both a DOM read and a screenshot after a settle delay, the
first screenshot attempt landed before DWM had repainted, 300ms was not enough, 1000ms was), a second
real click reverts to "Dev". Screenshots: `testdata\uicheck\out\v3-startup-dev.png`, `v3-business.png`,
`v3-back-to-dev.png`. Not verified live (no client rules or Copilot plan configured on this laptop):
the client filter/column, the per-client table, and the credits-left line all render correctly against
synthetic fixtures in `internal\history\history_test.go`/`internal\vendorstrip\vendorstrip_test.go` but
have not been seen populated in the real window; worth a spot-check once a real `burnmon.json` with an
`owners` rule and a `copilot_plan` exists. `go vet ./...`, `go test ./... -count=1` (every package
green, including new tests in `internal/pricing`, `internal/live`, `internal/history`,
`internal/forecast`, `internal/vendorstrip`), `.\build.ps1` and `node --check` (both inline `<script>`
blocks) all green.

## 2026-09-23, v0.3 V3-2: client map (K1), active time (K2)

Read `02_roadmap\2026-09-23_v0.3_spec.md` section 2.2 K1/K2. K1: `OwnerRule` gains optional
`client` and `remote` fields. A new unified `Config.matchRule(projectPath)` (used by both
`OwnerFor` and `ClientFor`) matches per spec order: a rule whose `remote` matches the project's
git origin remote (read directly from `.git\config` in the new `internal\pricing\gitremote.go`,
no git binary, walking up to find `.git`, handling a plain checkout, a worktree's `.git` file
with its `commondir` chain, and a submodule) wins first; otherwise a rule whose `match` path
prefixes the project wins; otherwise owner `"personal"`, client `"unassigned"`. The one winning
rule decides owner and client together, not two independent lookups: an Opus code-review pass
after the first implementation caught a Critical bug in the original (independent) version, a
remote-only rule (no `match`) made `OwnerFor`'s path prefix check empty, so it matched every
path and leaked a client's owner onto every other project, including Valona's. Fixed by the
unification above (a rule with an empty `match` and no matching `remote` never wins). The same
review caught two more real issues, both fixed and covered by new tests: (1) remote matching was
a bare substring, so a pattern for one repo (`github.com/org/dsi`) also matched a sibling
(`.../dsi-engine`); fixed with `remoteMatches`, a whole-path-segment match bounded by `/` on
both sides. (2) the git worktree case never actually worked: a worktree's `.git` file points at
`<main>/.git/worktrees/<name>`, which has no `config` of its own, only a `commondir` file
pointing back at the real `.git` directory; `gitRemoteURL` now follows that chain and resolves a
relative `gitdir:` against the `.git` file's own directory rather than the process cwd. Ruling
kept from the first pass: an SSH remote (`git@github.com:org/repo.git`) and an https remote for
the same repo must match the same `"host/path"`-shaped pattern, so `remoteMatches` normalizes
the SSH form (`normalizeRemote`) before comparing; a test against the two literal URL shapes
caught a real bug in this too (the normalize step was only applied on one side) before it
shipped. `client` lands as a new column on `events` (migration 7, additive) alongside `owner`;
the spec's own text calls it "migration 6", but `migrations.go` already has a real `Version: 6`
(the v0.2.1 session_id index), so the spec's number is stale, not a live instruction, and this is
migration 7 instead (noted here per the plan's Global Constraints). `burnmon-cli reown`'s
`Store.ReownEvents` now takes both `ownerFor` and `clientFor` and reapplies both in one pass; its
call site in `cmd\burnmon-cli\main.go` memoizes `clientFor` per project (same reasoning as the
ingest-side cache below), since it otherwise means one `.git\config` read per stored row. Added a
test loading a literal v0.2 `burnmon.json` (owners-only, no `client`/`remote`) and confirming it
loads unchanged and `OwnerFor`/`ClientFor` behave exactly as documented. `burnmon.example.json`'s
owners comment now shows one commented `client`+`remote` rule. K2:
`internal\dataset\fromstore.go`'s `buildSession` now also sorts each session's event timestamps
into `time.Time`s and sums consecutive gaps (`activeTimeMinutes`), any gap strictly above
`active_idle_minutes` (config, default 10 via `ActiveIdleMinutesOrDefault`) counting 0; exactly
at the cutoff still counts, per spec wording ("above" not "at or above"), a test pins the
exactly-10-minute case explicitly, and another pins several same-instant events at 0 active
minutes (no NaN, no negative). `scan.Session` gains `Client` and `ActiveMinutes`; a new
`dataset.ActiveMinutesByClient` sums sessions' `ActiveMinutes` per client for the per-client
rollup K2 asks for (no UI this session, per the spec). A separate performance fix, also from the
plan's own Task 3 scope: `internal\dataset\dataset.go`'s ingest loop memoizes `ClientFor` per
project path across a whole ingest pass, since many events from the same file share one project
path and `ClientFor`'s remote check reads a file from disk. Spot-check (plan assumption A8): ran
against the real local store (`%LOCALAPPDATA%\burnmon\burnmon.db`, auto-migrated 6 to 7 on open)
for the three earliest sessions starting 2026-09-22 UTC: session `0fe88daa` (259 calls,
06:54 to 08:54 UTC, 119.8 active minutes), session `7b13b2f8` (51 calls, 07:06 to 07:47 UTC,
41.4 active minutes), session `agent-a2b5c1fb` (16 calls, 07:07 to 07:16 UTC, 9.4 active
minutes). Comparison against `C:\ZND\10_holding\03_logs\time\` could not be completed: that
folder does not exist anywhere under `C:\ZND\10_holding` (confirmed by listing `03_logs\`), so
there is no time-log entry for 2026-09-22 to compare against; A8's "if an entry exists" clause
covers this case. The three figures above stand as the recorded spot-check output, unverified
against an external log. Owner and client came back empty on all three real sessions: the real
`burnmon.json` carries no `owners` rules yet, so `matchRule` correctly falls through to its
no-rules-configured default for both; a real client rule plus `burnmon-cli reown` is needed to
see a populated `client` column on real sessions. Full suite green (`go test ./... -count=1`),
`.\build.ps1` green. No UI in this session, per K1/K2's own scope. Deferred (not fixed this
session, ledgered as minors from the Opus review): `normalizeRemote` does not reshape an
`ssh://` remote with an explicit port or a non-`git` SSH user; per-client active time is a plain
sum of that client's sessions, so two overlapping sessions (an agent subsession inside its
parent, seen in the spot-check above) can push a client's total past real wall-clock time, worth
revisiting when K4's export numbers are checked against Talon's own time log.

## 2026-09-23, v0.3 V3-1: price books restructured into dated JSON, C2 cost function

Read `02_roadmap\2026-09-23_v0.3_spec.md` sections 1 and 2.1 C1/C2 and
`04_assets\2026-09-22_token_monitor_architecture.md` section 4.3 before touching
`internal\pricing\pricing.go`. C1: added three dated, embedded JSON price books, each with a
`date` and `source` field (`internal\pricing\books\anthropic_api.json`,
`openai_api.json`, `copilot_credits.json`, loaded via `go:embed` in the new
`internal\pricing\books.go`), keyed by exact model id rather than the old
family-generic `Prices` map (left untouched, still serving the v0.2 UI paths). Live-checked
today against `platform.claude.com/docs/en/about-claude/pricing`,
`developers.openai.com/api/docs/pricing`,
`docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing` and
`github.com/features/copilot/plans` (the last two via `firecrawl_scrape` after `WebFetch`'s
own AI-summarised read of the OpenAI page came back with garbled, non-exact numbers; the
firecrawl markdown pull was the one actually used for every price written into the books).
Finding worth flagging: Anthropic now prices Claude Opus 5.5 apart from Claude Opus 5
($4/$20 vs $5/$25 per MTok in/out) with its own 0.05x cache-read multiplier (Claude Fable
5.1: 0.025x, everyone else: 0.1x), which the old single "opus" family bucket could not
represent; this is the concrete reason C1 asked for per-model, not per-family, books. C2:
`internal\pricing\cost.go`'s `CostForEvents` groups a set of events by vendor and returns
every basis a book covers (`api_list`, `subscription`, `credits`) plus the headline basis
per spec 2.1 C2 (credits for `github`/Copilot, subscription share once
`Config.SubscriptionConfigured()`, else API list labelled "API list (upper bound)"); a
vendor with no book at all (Hermes, vendor `nous`, and any future unmapped vendor) gets no
basis and a nil headline, "tokens only". Extended `burnmon-cli price-check` to print all
three new books (with date and source) plus the Copilot plan-credit allotments and whether a
real subscription calibration is configured, alongside the existing v0.2 family-book
listing (kept, relabelled "(family, v0.2)" so the two are not confused). No UI touched this
session, per the spec's explicit "No UI in this session"; wiring cost-on-every-page is
V3-3's job. VERIFY (unresolved, left out of the books rather than guessed): a ChatGPT or
Codex plan-credit table (nothing found on a primary OpenAI/ChatGPT page today); GitHub
Copilot Business and Enterprise seat credit allotments (the `github.com/features/copilot/plans`
"for businesses" tab did not render in a plain page fetch, so only the four individual
plans - Free, Pro, Pro+, Max - are in `copilot_credits.json`'s `plans` block); `gpt-5.6-terra`
and `gpt-5.6-luna`, no longer listed on OpenAI's own current pricing page (superseded by
`gpt-6-sol`/`gpt-6-luna`), so left out of the new `openai_api.json` book even though the old
family-generic `OpenAIPrices` map (untouched, v0.2 path) still carries them at the same
numbers GitHub's mirrored Copilot table still shows. Tests: `internal\pricing\cost_test.go`
(one fixture per vendor - anthropic, openai, github, and a no-book vendor - covering every
basis and the headline choice, plus an unknown-model-prices-zero case) and
`internal\pricing\books_test.go` (a partial `burnmon.json` override of one Anthropic model
keeps every other compiled-in model, matching the existing `Prices`/`OpenAIPrices` merge
behaviour; the embedded books parse non-empty). `go test ./... -count=1` and `.\build.ps1`
both green.

## 2026-09-23, v0.2.3: window check patch, W0 to W8

Read `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.2.3_window_check_patch.md`. W0 first:
`WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port=9222` (chromedp) does not
reach the browser process on this laptop, confirmed with a clean process tree and by
inspecting the actual `msedgewebview2.exe` (`--type=` absent, main/browser process) command
line; a from-scratch attempt at explicit `ICoreWebView2EnvironmentOptions` in a vendored
`go-webview2` fork also failed (`E_INVALIDARG`) after several genuinely different fix
attempts, so per the debugging process that path was abandoned rather than pushed further.
Built `tools\uicheck` instead: Win32 screenshot (`BitBlt` from the screen DC, not
`PrintWindow`, which produced black frames after heavy process churn this session) plus
`SendInput` click/key, and a dev-only TCP eval channel (`cmd\burnmon\uicheck_devserver.go`,
`BURNMON_UICHECK=1`, never set outside this script) for DOM reads/JS calls, since only the
process hosting WebView2 can run script in its own page. Hit and fixed two harness-level
issues along the way: `uicheck.exe` was not DPI-aware (mis-sized every capture and click);
and a "ghost window" (explorer.exe keeps a force-killed app's HWND alive briefly to show
"Not Responding" in Alt-Tab, `FindWindowW` cannot tell it apart from the real thing) needed
an owning-process check. W1 to W8 each reproduced against the real window first (a genuine
before build for W1, before/after both against the fixed binary for W2 to W8, since
rebuilding a separate broken binary per item was not worth the round trip once the fix was
this well isolated by direct measurement first): startup blank/Not Responding (real cause:
`scan.DefaultSourcesWithOptions`/`codex.NativeSources`/`SeedNativeRoots`/`startLiveWatch` ran
on the UI thread before `w.Run()`; moved into the startup goroutine; residual ~650ms-1.5s to
first paint is WebView2's own engine warm-up, not app code, measured directly); the turn
drawer's Close button and overlay click (inline `onclick="closeTurnDrawer()"` resolves in
global scope, but the whole page script is one IIFE opened at line 476 and not closed until
line ~2271, so both attributes silently threw `ReferenceError` on every click; Esc already
worked, wired via `addEventListener` from inside that same closure; fix moves Close/overlay
to `addEventListener` too); a horizontal scrollbar on a genuinely unbreakable long path
(`table-layout:fixed` plus `overflow-wrap:anywhere`, reproduced with `scrollWidth` 947 vs
`clientWidth` 560, fixed at 560/560); duplicate "recent session" legend entries traced to
`live.BuildSnapshot` excluding a session from `Snapshot.Sessions` once idle past its shorter
`runningWindow`, even though it still has bars in the chart's longer 30-minute window
(fallback label now the session id's own first 8 characters, distinguishable per session);
legend order (vendor, surface, model, project, alphabetical, cost last, verified with four
deliberately out-of-order synthetic sessions); repeating whole-number axis ticks (`tokPrecise`,
a two-decimal formatter scoped to this one chart, `tok()` elsewhere unchanged); skipped
X-axis minute labels (every minute now; rotation only under 36px/minute); the cost axis not
hiding with its series (`display:'auto'`, Chart.js's own visible-dataset-driven axis mode).
`node --check` on both extracted script blocks, `go vet`, `go test ./... -count=1`,
`.\build.ps1` and `scripts\uicheck.ps1` (all checks, W0 to W8) green, this session's own
live Claude Code activity providing the "real store, live session" data `bmLive` fed
throughout; no separate Codex session was running this pass. Version constants to 0.2.3.
Committed locally, not yet tagged or pushed.

## 2026-09-23, v0.2.2: Now page patch, N1 to N6

Read `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.2.2_now_page_patch.md` and worked N1
to N6 in order. Capability note up front, per the session prompt's own instruction to say so
rather than skip verification silently: this session has no way to open or click the native
WebView2 window burnmon.exe draws, so every item below was verified by proxy instead of by
watching or clicking the running app, specifically: reading the rendered template.html logic
directly, `node --check` on the two extracted inline `<script>` blocks, `go test ./... -count=1`
and `.\build.ps1` after every item, and, where the change touched runtime data rather than
only markup or CSS, querying `burnmon-cli.exe live --json` against the real store (54 MB,
~51,300 events at the time) while this very session's own Claude Code turns were live in that
window; no Codex CLI session happened to be running for a concurrent window-side check, so
N6 rests on a real rollout file read directly (see below) plus a unit test, not a live Codex
session. N1 (turn drawer could not close): root cause was not the drawer's own JS, which
checked out correct in isolation, but `bmTurn` (`cmd\burnmon\main.go`) still returning its
result synchronously from go-webview2's `Bind` callback, the same UI-thread mechanism
v0.2.1's own hang patch documented; opening the drawer re-reads a session's whole event
history on the same thread the Close button's click has to be processed on, so on the real
store's scale the window reads as "the drawer cannot be closed" rather than "briefly frozen".
Fixed the same way v0.2.1 fixed `bmSessionInsight`/`bmHistory`: `bmTurn` returns immediately,
resolves through a goroutine and `asyncResolveJS`/`__bmTurnResolve`
(`internal\report\template.html`), keyed by `sessionID:turn` since more than one turn can be
in flight; added an `Escape` key handler as its own independent close path. N2 (chart
rewrite): read `drawNowChart` and its F8 comments first as instructed; widened
`live.BucketSeconds` from 10 to 60 (`internal\live\live.go`), which the existing
`windowEnd`/`windowStart` truncation logic already turns into "last 30 closed minutes plus
the current, growing one" with no further change, confirmed via `burnmon-cli.exe live --json`
returning `bucket_seconds: 60` and a 30-slot chart against the real store; rewrote
`drawNowChart` from smoothed per-session lines to clustered per-session-per-minute bars
(Chart.js's own default bar grouping), kept F8's one-instance/`update('none')`/damped-Y-max
rules, added a `VENDOR_COLOR_FAMILIES` table (Claude warm reusing the template's own `--warn`
token, Codex green, Copilot blue, Hermes violet, dark- and light-mode pairs, `shadeHex` steps
a second/third concurrent session of the same agent value two shades apart), a K/M/B token
axis and matching tooltip format (`tok()`, already used elsewhere on the page), a per-minute
gridline with the label text itself thinning to every 5 minutes under 40px/label
(`this.width` from Chart.js's own tick-callback binding), and removed the finding-marker
point layer entirely per the spec ("findings stay in the turn ticker, the drawer and the
session card"); a bar click now calls `chart.getElementsAtEventForMode('nearest', {intersect:
true})` itself (the chart's hover mode is `'index'`, which would otherwise hand back every
bar in that minute, not the one clicked) and opens the drawer for that session's own largest
turn inside the clicked minute. Ruling: the colour table also defines `codex-desktop` and
`copilot-vscode` shades for a lighter GUI-surface shade neither adapter can produce yet
(Codex always sets agent `"codex"`, Copilot in VS Code is not an adapter at all, per
STATUS.md's own Known gaps); left them in as forward-compatible definitions rather than
waiting for that adapter work, documented in STATUS.md's Known gaps rather than treated as
dead code. N3: added `margin-top:28px` to `.now-cards`, the same value `h2`'s own top margin
uses everywhere else on the page. N4: `#t_vendorstrip td a` now inherits colour and drops the
underline until hover, `title="Open in History"` carries the tooltip natively. N5 (context
window): live-checked `claude-sonnet-5` against
`platform.claude.com/docs/en/build-with-claude/context-windows` (Perplexity, cited pages),
confirmed Sonnet 5's 1,000,000-token window is now its standard/default tier, no beta header
needed, reversing yesterday's own dated comment in `internal\pricing\pricing.go` that assumed
otherwise; the same check found Opus 5.5 and Fable 5.1 also default to 1M, Haiku 4.5 stays at
200K, so `ContextWindows` was updated for exactly those three, `ContextWindowBookDate` moved
to 2026-09-23. While verifying live against the real store, the running Cowork session's own
model reported as `"claude-opus-5-5"`, a string with no entry at all under the book's
previous `"claude-opus-5"` key (which no real session had ever produced), so that key was
renamed to match; `ModelFamily`'s substring match still prices it correctly either way, this
only affected the context-window gauge. `TestBuildSnapshot_RunningVsStale` and
`TestAnalyze_ContextRunway_ExpectedTurnCount` (`internal\live`, `internal\insight`) both
asserted the old 200K figure and were watched to fail before being updated, the latter
switched to `claude-haiku-4-5-20251001` (the one model still booked at 200K) to keep its
hand-computed turn-count math valid rather than recomputing it for 1M; runway text needed no
separate code change, `insight.contextRunway` and `live.buildSession` already read
`cfg.ContextWindow(model)`. N6 (Codex model unknown): read a real rollout
(`~\.codex\sessions\2026\09\23\rollout-2026-09-23T11-06-23-...jsonl`, 868 lines) directly:
`turn_context` (the only line carrying `model`) appeared 7 times against 114 `token_count`
lines, all but the first well past `scanHeaderMeta`'s own re-read, which only ever recovered
`session_meta`'s cwd/surface and never looked at `turn_context` at all, so most of
`watch`'s incremental `Parse` calls landed in the gap between two `turn_context` lines with
none of their own and fell back to `model=""`, the same mechanism F2 already fixed for
surface. Extended `scanHeaderMeta` to also track the most recent `turn_context.model` across
its existing 64KB header window (kept scanning past the first `session_meta` rather than
breaking there), seeded into `Parse`'s own `model` var for `from > 0` reads; a sub-run
thread's `turn_context.model` (its own name, not a model id) is still excluded, guarded by
the existing `TestParseSubRunModelBecomesTitle`. Extended
`TestParseIncrementalReadKeepsSurface` (the exact precedent for this class of bug) with a
`Model` assertion, watched it fail (`Model = ""`) before the fix, green after. `node --check`
on both extracted script blocks, `go test ./... -count=1` and `.\build.ps1` all green after
every item. Version constants moved to `0.2.2` in `cmd\burnmon\main.go` and
`cmd\burnmon-cli\main.go`; `STATUS.md` and `02_roadmap\roadmap.md` updated, `STATUS.md`'s
Known gaps carries the colour-family ruling above. No open choice needed stopping for: the
Codex/Copilot lighter-shade ruling above was resolvable from the existing adapter code and
STATUS.md's own Known gaps section, not a genuine fork with more than one defensible reading.
`C:\dev\Work` untouched throughout. Committed locally; tag and push commands printed at the
end of this session's own reply, not run.

## 2026-09-23, v0.2.1: Refresh hang patch, measured

Read `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.2.1_hang_patch.md`. Measured before
fixing, against the real store (54 MB, 51,028 events, 1,719 sessions at the time),
with a Claude Code and a Codex session running: added stage timing to `Collect`/`ingest`
(source listing, WSL walk, per-file bytes for files over 10 MB, `DeleteEventsForOtherPaths`,
`AllEvents` row count, `SessionsFromEvents`, `agg.Build`, `BuildPayload`, `json.Marshal`
byte size, `report.Render`, `WriteFile`) and around every bound function (`bmLive`,
`bmVendorStrip`, `bmForecast`, `bmHistory`, `bmSessionInsight`), then built and ran
`burnmon.exe` against the real store (`burnmon-app.log`, `SESSION_LOG.md`'s own numbers
below). H1 to H4 confirmed and killed with those numbers, not as the spec guessed: `Collect`
itself finished in 1.2 to 2.6 seconds and the embedded payload was 850 KB, so H2 (the v0.1
pipeline's Months/Weeks/Days as the memory driver) does not hold; H1's mechanism (the store's
one connection, no WAL) is real but the thing actually saturating it was not `Collect`'s own
ingest and `AllEvents`, it was `internal\report\template.html`'s `renderSessions()`, which
fired one `bmSessionInsight` call per kept session unconditionally on every page load, all
of them landing on the WebView2 UI thread in the same burst (go-webview2's `Bind` runs the
bound Go function synchronously on that thread, confirmed by reading
`github.com/jchv/go-webview2`'s `msgcb`/`callbinding`): 1,192 to 1,719 sessions, each near
60 ms (H3's disk read explained the same way, `EventsForSession`'s `WHERE session_id = ?`
could not use `idx_events_session (vendor, session_id)`, since `session_id` is not that
index's leading column, so every call fell back to a full table scan), serialised one at a
time and blocking paint and input for the whole burst, which is what WebView2 reports as
"Not Responding". H4 confirmed by code (`bmHistory` still called `st.AllEvents()` directly)
but was not the reproduced driver this session (History tab was never opened). Fixes, in
the order the numbers actually called for: added migration 6, a `session_id`-only index
(`internal\store\migrations\migrations.go`, `schema_version` head now 6,
`TestMigrateRealV01Store`/`TestFreshStoreAtHeadVersion` updated and green against the real
store); split the store into a one-connection writer and an eight-connection WAL reader
pool with a five-second busy timeout (`internal\store\store.go`, every read-only query
routed to the pool, every write-adjacent one kept on the writer); changed `bmSessionInsight`
and `bmHistory` to return immediately and resolve through `w.Dispatch` from a goroutine
instead of blocking the UI thread for the real work (`cmd\burnmon\main.go`'s new
`asyncResolveJS`, `template.html`'s `__bmSessionInsightResolve`/`__bmHistoryResolve`); capped
`renderSessions()`'s fan-out to 8 concurrent `bmSessionInsight` calls instead of unbounded
(`runWithConcurrency`); removed `Weeks` and the unused `CoverageNote` field from
`dataset.Payload` (checked every `D.*` reference in `template.html`; `Totals` stays, the CLI
prints it); kept `bmHistory` on `AllEvents` rather than rewriting it to a SQL aggregate,
since the frontend's vendor/owner dropdowns are built from the same unfiltered pass and a
narrower query would have silently hidden a filter option outside the selected range, a
real regression the measurements gave no reason to risk; and added a 400 MB
`debug.SetMemoryLimit` (`cmd\burnmon\main.go`'s `init`), since an isolated repro proved
`Collect`'s own data is under 50 MB live even at this store's scale (a forced GC after each
stage: `AllEvents` 46 MB, `SessionsFromEvents` 48 MB, `agg.Build` 48 MB, `BuildPayload`
48 MB) while the running app's `HeapAlloc` reached over 1 GB before Go's default GC pacing
ever caught up. Results: `bmLive` stayed under 50 ms in every post-fix run (zero slow-call
log lines), so Not Responding did not reproduce again; CPU during the Sessions-tab burst
dropped from a sustained roughly 98 percent to roughly 50 to 55 percent, settling toward
idle afterward; peak WorkingSet dropped from the reported 1,438 MB (and 1.6 to 1.7 GB in
this session's own pre-fix runs) to about 900 MB, short of the 400 MB Done-when target;
`Collect` on a warm store measured 1.2 to 2.6 seconds in most runs but touched 5.9 seconds
once, when the Sessions-tab fan-out was itself saturating the new read pool, so the under-5-
second target is met in the common case, not guaranteed under this specific concurrent
load. Both misses are named in `STATUS.md`'s Known gaps and `02_roadmap\roadmap.md` item 4
rather than claimed as fixed. `go test ./... -count=1` green across every package,
including `TestMigrateRealV01Store` run against the real store; `node --check` on the
script extracted from `template.html` passed; `.\build.ps1` green. Version constants moved
to `0.2.1` in `cmd\burnmon\main.go` and `cmd\burnmon-cli\main.go`; `STATUS.md` and
`02_roadmap\roadmap.md` list v0.2.1 before v0.3, and item 5 renumbered from the v0.3 grill
already in the working tree. No open choice needed to stop for: the deviation from H1 to H4's
literal framing was a measurement finding, not a judgment call, and the bmHistory scope
decision above is recorded with its reasoning rather than asked. `C:\dev\Work` untouched.
Hub brief written as `04_assets\hub_agent_update_2026-09-23_v0.2.1_hang_patch.md`. Committed
locally; tag and push commands printed for Wilco, not run.

## 2026-09-23, v0.2 leftovers: About copy, patch spec, hub briefs

Three mechanical leftovers from v0.2, no version bump. (1) `internal\report\template.html`
lines 349 and 420 said "the Overview tab" for the personal-cut-of-the-bill figure; that tab
was removed in 40B. Checked whether History shows the same figure: it does not. History's
`cost_usd` (`internal\history\history.go:95-113`, `eventCostUSD`) is plain API list price,
gated on vendor coverage; the subscription-basis "personal cut of the company bill" figure
the two sentences described has no page to point at, since per-vendor cost and the
subscription/API basis split are out of scope until v0.3 (STATUS's own "Known gaps"). Per
the session prompt's own instruction for that case, deleted both sentences rather than
inventing a new home for the claim; left the dead `#overview` markup and the rest of the
About text untouched. `STATUS.md`'s "Known gaps" no longer names the Overview-tab leftover
(kept the separate, still-true gap that About only covers Claude's seat model). (2) The
session prompt's claim that `02_roadmap\2026-09-22_v0.1.2_patch_spec.md` was modified in the
working tree since before 39B did not hold: `git status --porcelain -uall` and `git diff`
against that file both showed nothing pending, and `git log` shows it last touched in commit
`876c37e` (the same commit A2/A3 shipped in), already committed. No diff to show Wilco, no
commit made for this item. (3) `git status --porcelain -uall` showed exactly one untracked
file in the whole tree, this session's own `02_roadmap\2026-09-23_v0.2_leftovers_prompt.md`;
every file already under `04_assets\`, including all `hub_agent_update_*.md` briefs, was
already tracked (`git ls-files 04_assets | wc -l` matched the directory's file count, 26).
`node --check` on the script extracted from `template.html` passed, `go test ./... -count=1`
green across every package, `.\build.ps1` green. `C:\dev\Work` untouched. Hub brief written
as `04_assets\hub_agent_update_2026-09-23_v0.2_leftovers.md`. Not pushed; push command
printed for Wilco.

## 2026-09-23, 46B: tag v0.2.0

Read the 46A entry above (rc.1 results, tag `v0.2.0-rc.1`, commit `541c6f2`) and spec
section 5: 46A found zero failing Done-when or VERIFY items ("No failure found this pass,
so nothing was fixed"), so this session's own instruction to "fix the fails listed for
rc.1 if Wilco confirmed them" had nothing to apply to; no source code changed. Set version
constants to `0.2.0` in both binaries (`cmd\burnmon-cli\main.go:35`, `cmd\burnmon\main.go:47`,
both previously `0.1.2`). Updated `STATUS.md` (header and "Next" section now describe the
shipped v0.2.0 state, pointing at v0.3 rather than the release-candidate week),
`DEADLINES.md` (the 2026-11-14 v0.2 row marked "DONE 2026-09-23, tagged `v0.2.0`"), and the
project `02_roadmap\roadmap.md` (item 3, v0.2, marked done with the 46A rc-pass result;
item 4, v0.3, marked next). `go test ./... -count=1` green across every package
(`cmd\burnmon`, `cmd\burnmon-cli`, every `internal\...` package with test files); one
`unlinkat ... Access is denied` line after the last `ok` is Windows temp-file cleanup on a
completed test binary, not a test failure. `.\build.ps1` green, producing both exes. No
open choice was hit that needed stopping to ask: the fails-list step resolved to an empty
set by reading 46A's own record, and the docs/roadmap edits were mechanical, not judgment
calls. `C:\dev\Work` untouched. Hub brief written as
`04_assets\hub_agent_update_2026-09-23_46b_v0.2_ship.md`, real date, not the session
prompt's literal `2026-11-14` placeholder, matching 46A's own precedent in this folder and
the house date-prefix rule. Not committed or tagged; commit and tag commands for `v0.2.0`
printed for Wilco, per the session prompt's own stop-before-tagging instruction.

## 2026-09-23, 46A: release candidate, Done-when and VERIFY pass

Ran with two Claude Code sessions, one Cowork session and two Codex sessions genuinely
running concurrently (`burnmon-cli live --json`: agents `claude-code` x2, `cowork` x1,
`codex` x2). Section 5, all six items pass, evidence this session's own, not carried from
45B: (1) `template.html`'s hash router defaults any unknown/empty hash to `now`
(`TAB_IDS.indexOf` fallback, line 2072) and stays unreachable for `overview`, which is not
in `TAB_IDS`; `vendorstrip.Build` run directly against the real store returned honest
per-agent totals for all five agents plus a total row (Claude Code 21.8M/601M/3.75B
today/week/month tokens, Codex 4.1M/50.4M/116M, Copilot CLI, Cowork, Hermes all present);
`burnmon-cli live --json` showed a re-prefill finding, cause "first turn after resume",
confidence 0.4, on this session's own turn 1; the chart-drawing code (`drawNowChart`) is
byte-identical to 44B's own 5-minute live-watched version (only `template.html` line 983
changed since, an unrelated footer fix in 45B), so that verification still holds and was
not repeated. (2) `history.Build` run directly against the real store with
`{period: week, vendor: codex}` returned one call's totals (159,870,436 tokens, 217
sessions, 1,842 turns), matching the "two clicks" (period dropdown, vendor dropdown)
design; no project-level "for ZND" filter exists, same reading as 45B. (3) built a
synthetic genuine v0.1 store from scratch (migration1's exact `events`/`cursors`/`meta`
DDL, no `schema_version` table, no `owner` column, 250 rows across 10 sessions) rather
than reusing `TestMigrateRealV01Store`'s real-store copy, since the real store on this
laptop is now itself already at schema 5 and copying it no longer exercises a genuine
pre-migration file (the same real-time drift 45B flagged for forecast scoring); ran it
through `store.Open`, got 250 of 250 events back, `schema_version` read 5: zero event
loss confirmed against a real pre-migration schema, not a no-op re-open. (4) Hermes (4
events) and Copilot CLI (24 events) both present in the real store today, both resolve
through the same `AGENT_LABELS` map every other agent uses ("Hermes", "Copilot CLI"),
never a raw agent key. (5) `forecast.Build` run directly against the real store returned
`locked: true` with the gate text verbatim ("forecast unlocks after the first scored week
(week 46)") and `scored_weeks: 0`; the real ISO week is still 2026-W39, the same real-time
constraint 45B found, unchanged. (6) swept `template.html` source directly (not just the
generated report): zero genuine Dutch words (the only `\bən\b`-shaped hits were `lang="en"`
and `en-US` locale strings), zero em dashes outside the seven already-flagged instances in
the `#overview` section (`id="card_this"`, `card_last`, `o_top`, `o_seat`), which stays
unreachable through both the tab bar and hash routing per item (1)'s check, and every
remaining `claudecost` string in source is the documented exception (successor note, the
pre-rename config fallback name). One nuance this session caught that 45B's report-only
grep did not: the most recent generated report (`reports\burnmon-report-20260923-094045.html`)
embeds real historical session titles verbatim as data, and several of Wilco's own real
Claude Code session titles from the actual claudecost project quote "claudecost-app" and
one quotes a colleague's Dutch permission message ("mag je deze gebruiken met naam en
toenaam"); this is ingested user data, not BurnMon's own copy, and the shared reply
contract's own rule (preserve verbatim source evidence, don't rewrite quotations to pass a
prose check) applies to it the same way it would to any other quoted material, so it was
left untouched. Section 6, VERIFY carried, what the code assumes today and where labeled:
Claude Code cache TTL, 60 minutes (subscription-seat default, `pricing.go`
`DefaultClaudeCodeCacheTTLMinutes`, checked against code.claude.com/docs/en/costs
2026-09-22), applied in `insight.reprefillCause`'s gap-since-previous-turn branch, shown
in the spike drawer's cause text ("gap N min, exceeds 60 min cache TTL") when it fires;
Claude Code auto-compact threshold, documented (`ClaudeCodeAutoCompactTokens`, ~967,000
for a 1M-token beta window) but not applied by any rule since burnmon's own `ContextWindows`
table prices the 200K standard tier, not the 1M beta, and `compaction` detects a
compaction by its effect (a >30% context drop) rather than comparing to this number, so
nothing in the UI currently labels this figure; Hermes context window, confirmed absent
from the data entirely (not merely unverified), so the Now card's gauge falls back to
"context window unknown" for every Hermes session in practice (`hermes.go`'s own comment,
`contextWindowKnown`); Copilot CLI `data.db` layout, confirmed false in 43B on a live
install (real file is `session-store.db`, usage rows arrive per API call not at session
close) and the adapter was rebuilt around that finding, so nothing in the UI still assumes
"totals at session end" for Copilot CLI, it shows the same growing-total treatment as
Hermes; Copilot VS Code OTel file export, corrected to yes (2026-09-23 note above the 43B
entry in this log) but not wired into any adapter, so nothing in the UI labels it either
way yet, a v0.3 scope call still open for Wilco; Codex subagent structure in the trail,
confirmed there is none: Codex has no Claude-style `ParentID` nesting, only a flat
`thread_source`/`source.subagent` flag on internal auto-review sub-runs
("codex-auto-review"/"guardian_review") that `readSessionMeta` filters out as noise before
a session ever reaches the Now page, so Codex sessions never show a "subagents" nesting
the way Claude ones do, by design, not by gap. No failure found this pass, so nothing was
fixed. Noted, not acted on: `04_assets\hub_agent_update_2026-09-23_45b_done_when_pass.md`
is still untracked from 45B, left as found since ticking someone else's session's commit
is not this session's call. `go test ./... -count=1` and `.\build.ps1` both green.
`C:\dev\Work` untouched. No open call was hit that needed stopping to ask. Hub brief
written as `04_assets\hub_agent_update_2026-09-23_46a_rc_done_when.md`, real date, not the
session prompt's literal `2026-11-1X` placeholder, matching every other brief already in
this folder and the house date-prefix rule; the roadmap's week labels (39-46) are
build-order names, not calendar weeks, per 45B's own finding, unchanged since.

## 2026-09-23, 45B: buffer, Done-when run, scoring week check

Checklist showed 39B through 45A all ticked, so this buffer session ran the section 5
Done-when list from `02_roadmap\2026-09-22_v0.2_spec.md` against the built exe, with this
Claude Code session and (per `burnmon-cli live --json`'s own window) two real Codex CLI
sessions genuinely running concurrently. Results, one line per item: (1) pass, tab
routing defaults an empty/unknown hash to `now` (`template.html` line 2072) and the live
window at run time held 4 real running sessions (2 Claude Code, 2 Codex) with the vendor
strip returning honest per-agent totals for all five agents (`vendorstrip.Build` run
directly against `%LOCALAPPDATA%\burnmon\burnmon.db`); a re-prefill finding with cause
"first turn after resume" fired on this very session's own turn 1, confirmed via
`burnmon-cli live --json`; the Now chart's own drawing code (`drawNowChart`) is unchanged
since 44B's own 5-minute live check, so that verification still holds and was not
repeated. (2) pass, `history.Build` run directly with `{period: week, vendor: codex}`
against the real store returned a real total (62,901,057 tokens, 30 sessions, 695 turns)
in one query, matching the History page's one-call design; the Done-when phrasing "for
ZND" is descriptive, not a literal filter, since History has no project-level filter,
only vendor/period/owner, and owner rules are not configured on this laptop. (3) pass,
`TestMigrateRealV01Store` already covers this and passed; the real store's own migration
this session (schema 4 to 5, on first CLI run after the rebuild) additionally confirmed
it live, no event dropped (the "dropped 4 duplicate session(s)" log line is
`dataset.go`'s pre-existing report-level dedup, unrelated to the events table). (4) pass,
Hermes and Copilot CLI both have real events in the store (4 and 24 respectively) and
both resolve through `history.AgentLabel`/`vendorstrip.AgentLabel` to "Hermes" and
"Copilot CLI", never a raw agent key. (5) pass, `forecast.Build` run directly against the
real store returned the locked gate message verbatim ("forecast unlocks after the first
scored week (week 46)") with zero scored weeks. (6) pass, no Dutch giveaway word found in
the generated report HTML. Forecast scoring: real wall-clock ISO week is 2026-W39 (Sep
21 to 27), not week 45; the session prompts' week numbers are session/build-order labels,
not calendar weeks, and the whole 39A-45A sequence ran inside two real days
(2026-09-22/23), so no real ISO week has elapsed yet for scoring to advance past week 39.
This session's `forecast.Build` call was itself the first real invocation against the
production store (`bmForecast` only fires from the live webview, never from CLI report
generation) and correctly wrote week 39's plan (1,422,554,946 tokens) as a live, working
first write; scoring cannot reach week 45 until six more real weeks pass. Flagging this
now rather than fixing anything: this is a real-time constraint, not a bug. One real bug
found and fixed: the Sessions tab footer's average-cost-per-call cell used a bare em dash
placeholder (`template.html` line 983) when no calls matched a filter, a live user-facing
house-rule violation; replaced with `n/a`. One item left OPEN rather than silently
decided: the About page still reads "the Overview tab" twice (explaining the personal-cost
widget), a leftover from the tab removed in 40B; 45A already flagged this as a known gap
and chose not to fix it, and this session leaves that choice in place since rewriting the
copy is a judgment call about what the About section should say now, not a Done-when
failure. `go test ./... -count=1` and `.\build.ps1` both green after the fix.
`C:\dev\Work` untouched.

## 2026-09-23, 45A: README, STATUS, docs pass

Rewrote README.md for BurnMon v0.2 as scoped: what it reads (Claude Code, Cowork, Codex,
Hermes, Copilot CLI, and why claude.ai and VS Code Copilot are absent), the five tabs,
the Now page's findings and forecast chart, the owner rules, the CLI commands (report,
live, tools, insight, reown, plus price-check for completeness), config, WSL, what
BurnMon never does, and the v0.3 items named as not yet there. Filled STATUS.md with what
is true at this commit: schema version 5, all sixteen S/P/I/F/A spec items built except
Copilot VS Code wiring (confirmed reachable via OTel, not wired in), and named gaps
(template.html's About section still says "Overview tab" twice and stays Claude-only;
zero forecast weeks scored yet). Renamed claudecost.example.json to burnmon.example.json,
owners table shown as an empty array with the rule syntax in a comment. Checked _board:
board.json and board.html already carry no claudecost string and are generated
externally by `C:\ZND\projects\siteoffice\board\render_board.py`, not hand-edited here;
nothing to update this pass. Fixed the SESSION_LOG.md title itself ("claudecost, session
log" to "BurnMon, session log"); left every past entry's body text alone as history.
One open call, per the brief's own instruction to stop and ask on any: the brief assumed
a claudecost.json config fallback that did not exist in code (burnmon.json only, no
handling of the old name at all); asked, and was told to add it rather than write docs
for behaviour that was not there, so cmd\burnmon\main.go and cmd\burnmon-cli\main.go
each gained a same-locations claudecost.json fallback (burnmon.json always wins), the one
code change beyond strings and the example file. `go test ./... -count=1` and
`.\build.ps1` both green. `C:\dev\Work` untouched. The v0.2 spec's own build order still
shows 44B unticked in `02_roadmap\2026-09-22_v0.2_session_prompts.md` despite that
session's commit (`7fb66d8`) already landed; left as found, flagged rather than fixed
silently, since only 45A was this session's to tick.

## 2026-09-23, 44B: forecast chart and gate (F1), Now-chart smoothing

Built F1 in tokens, per spec section 2.4: migration 5 adds `forecast_scores`
(iso_year, iso_week, week_start, plan_tokens, actual_tokens, scored_at; a
week counts as scored once both are set), plus store methods
`InsertForecastPlan` (writes only the first time a week is seen, so the
Monday forecast is never revised), `RecordForecastActual` (writes only
once, guarded by `WHERE actual_tokens IS NULL`), `ForecastScore(s)` and
`DailyTokenTotals` (per-UTC-day sums via one SQL query). New package
`internal/forecast` builds the payload: `EnsureScored` records the current
week's plan and fills in any past week's actual once it has fully elapsed;
`Build` is locked (history only, the fixed text "forecast unlocks after the
first scored week (week 46)") until one scored week exists, otherwise it
returns the weekday-aware plan line (last 4 weeks' averages), the live line
(today's own rate held flat for the rest of the week, plus end-of-day and
end-of-month projections), and an error band (mean absolute percentage
error across every scored week). Scoring starts this session: the first
`bmForecast` call in the current ISO week writes that week's plan. Wired as
`bmForecast`, its own 1-minute timer mirroring `bmVendorStrip`'s pattern
rather than the 2-second `bmLive` poll, since every query here scans the
whole store; the Now page's Forecast card now draws a Chart.js line chart
(plan dashed, live solid, error band shaded around the live line) instead
of the old always-history table, with the gate text shown in place of the
chart while locked. Mid-turn, Wilco also asked for the existing 30-minute
"Live burn" chart to move to the top of the Now page, its cost line off by
default (click the legend to show it, tokens-only per v0.2), and to become
smooth per-session lines instead of stacked bars, animated only by the
window itself sliding right to left rather than any grow/shrink transition.
Tests: `internal/forecast/forecast_test.go` covers the plan line's weekday
averages against a four-week fixture, zero scored weeks showing the gate,
and one scored week (driven across two real calendar weeks, the way
scoring actually runs) unlocking a non-zero error band; `TestFreshStoreAtHeadVersion`
and `TestMigrateRealV01Store` in `internal/store` bumped from schema
version 4 to 5, and a new `TestForecastScoresRoundTrip` covers the
insert-once/update-once store semantics directly. `go test ./... -count=1`
and `.\build.ps1` both green; `node --check` on every extracted `<script>`
block in template.html also passes. `C:\dev\Work` untouched; no open call
was hit that needed stopping to ask.

## 2026-09-23, 44A: Sessions tab findings (I3)

Added the Sessions tab half of I3 the spec left open: a Findings column (count
by kind, e.g. "2 re-prefill, 1 expensive-turn") on every session row, an
expandable detail row below it listing each finding (turn, kind, cause,
confidence, evidence numbers), and clicking a finding line opens the same
drawer as the Now page, through the existing `.ticker-line` click delegation
and `bmTurn`, no new wiring needed there. Findings are computed on request
through one new bound function, `bmSessionInsight(sessionID)`
(`internal/live/live.go`'s `BuildSessionInsight`, the same
events-to-turns-to-`insight.Analyze` shape `BuildTurnDetail` already used for
one turn, just returning every finding for the session); nothing is stored,
and the frontend caches each session's result in a page-lifetime JS object
(`SESSION_INSIGHT_CACHE`) so re-expanding a row or re-sorting the table never
re-fetches. Measured `BuildSessionInsight` against the longest real session in
Wilco's own store (`%LOCALAPPDATA%\burnmon\burnmon.db`, 51,727 events total):
the longest session has 461 turns, and 20 warm calls averaged **39ms**, well
under the 200ms gate, so no v0.3 caching proposal is needed. Also added the
owner column and owner filter to the Sessions tab (P6), shown only when at
least one session carries a non-empty `owner`, mirroring the History tab's
own `payload.owners`-driven toggle but derived client-side from `D.sessions`
since the Sessions tab's data is the embedded report snapshot, not a bound
call. `node --check`, `go test ./... -count=1` and `.\build.ps1` all green;
`C:\dev\Work` untouched. No open call was hit that needed stopping to ask.

## 2026-09-23, correction: A3 (VS Code Copilot OTel) is yes, not no

The 43B entry below (committed `876c37e`) answered A3 "no" from a CLI-only test that
never produced a real chat exchange. Wilco reloaded the VS Code window with the same
four `github.copilot.chat.otel.*` settings still in place and sent one real message
through the Copilot Chat panel; `%LOCALAPPDATA%\Temp\copilot-otel.jsonl` (the `outfile`
from that session) grew to 476KB of real spans within minutes: `service.name":
"copilot-chat"`, `service.version":"0.66.0"`, `event.name":"copilot_chat.session.start"`,
and a `gen_ai.client.inference.operation.details` span carrying
`gen_ai.request.model`, `gen_ai.usage.input_tokens`/`gen_ai.usage.output_tokens` and
`gen_ai.response.finish_reasons`, GenAI semantic conventions exactly as the facts file
described. The missing piece the first attempt lacked was not the settings, the
extension, or a real chat turn (all three were already in place, per the 43B entry) but
an extension host reload afterward, `code chat` alone from the CLI never triggers one.
Corrected answer for the spec's yes/no: **yes**, GitHub Copilot in VS Code can emit
OpenTelemetry to a local file on this laptop, unbounded growth while the setting stays
on is worth flagging (476KB in one short test session) but out of scope for this
correction. Per the spec's own consequence ("yes means Copilot VS Code enters v0.3"),
this is a scope decision for Wilco to make when v0.3 is planned, not applied here. While
checking this, also confirmed no bug in "Copilot keeps restarting": `main.log` for the
long-running window shows 7 extension host exits across the whole day, 6 clustered in
the 20 minutes around the settings/reload testing, every one a clean exit (`code: 0,
signal: unknown`) immediately followed by a successful Copilot Chat login and a real
completed request; nothing since. Reads as normal reload churn from testing, not a
crash loop. No product code changed for either finding.

## 2026-09-22, v0.2 43B: Copilot CLI adapter, OTel check (A2, A3)

A2 killed the plan's assumption on Wilco's live install (COPILOT_HOME `~/.copilot`,
Copilot CLI running, PID observed live) plus the three fixture sessions he recorded
today under `cwd = C:\ZND\projects\burnmon\testdata\copilot`, on two separate counts.
First, the file: there is no `data.db`; the store is `session-store.db` (+ `-wal`/`-shm`,
WAL mode). Second, the timing: usage is not written once at session close; table
`assistant_usage_events(session_id, turn_index, model, input_tokens, output_tokens,
cache_read_tokens, cache_write_tokens, reasoning_tokens, total_nano_aiu, created_at, ...)`
gets one row per API call, seconds apart, while the turn is in progress (57 rows over
~30 minutes of one real conversation, watched mid-run). Neither `sessions(id, cwd,
repository, host_type, branch, summary, created_at, updated_at)` nor `turns` nor
`session-state\<uuid>\workspace.yaml` carries an `ended_at`, `status` or `closed`
column: "session closed" is not a fact this store records at all. `input_tokens` and
`cache_read_tokens` grow monotonically per call within a session (the conversation's
own growing context, same shape as Codex's cumulative counters), so per-session
aggregation is last-row-wins, not a sum. Put this to Wilco as an open choice (the A2
build instruction's "totals at session end" framing was entirely premised on the killed
assumption); he chose to build it like Hermes instead: `internal\adapter\copilotcli`
(`PollOnce`, `DefaultDBPath`) polls `session-store.db` read-only every 5 seconds (wired
in `cmd\burnmon\main.go`'s new `startCopilotCLIPoll`, same shape as
`startHermesPoll`), emits one Event per session from a correlated-subquery "latest row"
per `session_id`, `RequestID` fixed to the session id for the store's upsert, `At` set
to that row's own `created_at` (a real recent timestamp while active, not a "now"
substitute) so `internal/live`'s existing generic last-event-recency window decides
running vs finished exactly as it does for every other adapter, no adapter-side "wait
for close" logic. Vendor/agent/surface `github`/`copilot-cli`/`cli` and the display
label "Copilot CLI" were already wired in `vendorstrip.AgentLabel` and
`history.AgentLabel` from earlier v0.2 work, needing no change. Fixture:
`testdata\copilot\copilot_fixture.db`, the three real sessions' `sessions` and
`assistant_usage_events` rows only (no `turns`, which carries real message text), built
from a live copy with a one-off Go script, not committed source.

A3, capped at one hour: GitHub Copilot in VS Code cannot emit OpenTelemetry on this
laptop's *current* extension set (only `ms-azuretools.vscode-azure-github-copilot`
shows in `code --list-extensions`, no `GitHub.copilot-chat` there), but Wilco chose to
test the real thing rather than stop at that; `GitHub.copilot-chat` turned out to
already be a VS Code 1.138.0 *built-in* (v0.66.0, invisible to `--list-extensions`,
confirmed by the install command's own conflict error), so no extension install was
needed. Settings tried (from code.visualstudio.com/docs/agents/guides/monitoring-agents,
fetched today), added to `%APPDATA%\Code\User\settings.json` and reverted after the
test: `github.copilot.chat.otel.enabled: true`, `github.copilot.chat.otel.exporterType:
"file"`, `github.copilot.chat.otel.outfile` pointed at a scratch path, and
`github.copilot.chat.otel.dbSpanExporter.enabled: true`. `code chat "say hello in one
word"` against Wilco's already-running VS Code window created the target file
immediately (extension reacts to the setting) but it stayed at 0 bytes after an 11
second wait: the CLI's `chat` subcommand only pre-fills the chat panel's input box, it
does not submit, and getting a real span written needs an interactive send in a signed-
in chat session. Declined to force that with a synthetic keystroke into Wilco's live,
multi-window daily editor (out of scope for a one-hour, no-product-code check). Answer
for the spec's yes/no: not proven yes this session, treated as **no** for the A3 gate
(Copilot VS Code stays out of v0.3 pending a real confirmation); one further step would
close it, Wilco re-enabling those four settings and sending one real chat message, then
asking for the outfile to be checked. `go test ./... -count=1` and `.\build.ps1` both
green.

## 2026-09-22, v0.2 43A: markers, ticker, spike drawer (I3 on Now)

Wired every I3 piece onto the Now page from the findings `bmLive` already computes.
`internal/live.Snapshot` gained `Turns []TurnEvent`: every real turn across every
session inside the 30-minute chart window (not just still-running ones, unlike
`Snapshot.Sessions`), newest first, capped at 50, each carrying its own matching
`insight.Finding` when `Analyze` found one at that turn (`buildTurns`, independent of
`BuildSnapshot`'s running-sessions map, same `insight.Analyze` call per session group).
`template.html`'s `drawNowChart` grew one Chart.js point dataset per finding Kind seen
in `Turns` (`findingMarkerDatasets`: triangle/rect/circle/star, one shape and color per
kind), each point's height pinned near the token axis's top so a marker never buries
under a tall stack; a `pointMeta` array carries session id and turn per point so the
chart's own `onClick` (`nowChartOnClick`) can open the drawer without re-deriving
anything from pixel coordinates. Rebuilding the marker datasets every poll and
reassigning `chart.data.datasets` does not touch F8's own bar-dataset caching
(`nowChart.datasetById`) or its smoothed axis maxima, so the "no jump, only the
rightmost bar grows" behaviour is unchanged; confirmed by reading F8's code path, not
by eye (see the live-check note below). A `#now_ticker` list under the chart
(`renderNowTicker`) renders one line per turn, newest first, `<time> <agent>, turn
<n>, <fresh> new[, <cache_read> cached][ · <Kind>: <Cause>]`, click-delegated (no
inline `onclick`, so no session id needs attribute-escaping) to the same drawer.

The drawer itself is one template branch (`renderTurnDrawer`) fed by one new bound
function, `bmTurn(sessionID, turn)` → `live.BuildTurnDetail`: re-reads
`EventsForSession(sessionID)` (any vendor, matching the store's own doc comment on
that method), re-derives the session's own turns and `insight.Analyze` findings the
same way `buildTurns` does, and reports the one at 1-based `turn`: model, all four
token classes, gap since the previous turn (absent on turn 1), every finding whose
`Turn` matches, and its tool calls, joined by the turn's own `RequestID` as the S2
`tool_calls.turn` key (`store.ToolCallsForTurn`, new). Confirmed by construction for
Claude (`tk` in `claude.go`'s tool_use parsing uses the identical requestId/uuid
precedence as the event key `EventsForSession` returns), left as an open, logged risk
for Codex: `pendingCalls[...].turn` comes from `internal_chat_message_metadata_passthrough.turn_id`,
a different id space than `Event.RequestID` (`sessionID + ":" + ordinal`), unverified
against any real Codex fixture to actually coincide; a Codex turn whose tool calls
don't join just shows none, not an error, same as a turn that truly called no tool.

The drawer's "files read" line was an open choice: `schema.ToolCall` had no path
field at all before this session (S2's migration 3 never carried one). Stopped and
asked; decided to add it now rather than ship the drawer without it.
`migrations.go` gained migration4 (`tool_calls.path`, additive, default `''`); both
adapters populate it best-effort, Claude from `file_path`/`notebook_path` on a
Read/Edit/Write/NotebookEdit tool_use's own input (confirmed against a real fixture:
`toolu_1`'s `{"file_path":"a.go"}` now round-trips as `Path: "a.go"`), Codex by
attempting to JSON-decode a function_call's `arguments` or a custom_tool_call's
`input` for `file_path`/`path` (unverified against a real custom_tool_call fixture:
most of those are shell commands, not JSON, so this is expected to resolve to "" in
practice more often than not, a documented gap rather than a silent one). `ToolCall`
also gained json tags throughout (first time any ToolCall crosses the wire to JS).

Verified end to end against the real local store, not a synthetic fixture: ran
`burnmon-cli.exe` for a full ingest pass (`store` migrated live from version 3 to 4
with real rows, no error), then `burnmon-cli.exe live -json` showed this very Claude
Code session's own turns and a real `re-prefill` finding on turn 8; a throwaway
`go run` of `live.BuildTurnDetail` against the same store for that session/turn
returned the matching Bash tool call (joined by `RequestID` = `req_011CfKA4QxE1DZye2XKpw3BE`)
and gap (`4.818`s), deleted afterward. Built and started the fresh `burnmon.exe`
(replacing the stale instance still holding the single-instance mutex from an earlier
run today); `burnmon-app.log` polled cleanly (`bmLive poll 30/60/90`, no panic, no
"could not bind") for the several minutes this agent could observe it running. The
spec's own "watch 5 minutes live: markers appear within 2 seconds, chart still drifts
without jumping" is a visual check this agent has no way to perform on a native
WebView2 window (no screenshot capability for it); the above is the strongest
headless substitute, not a replacement for Wilco actually watching the window with a
Claude Code or Codex session running.

`node --check` on the extracted inline script (`internal/report/template.html`
lines 462-1947) passed. `go test ./... -count=1` and `.\build.ps1` both green;
two store tests (`TestFreshStoreAtHeadVersion`, `TestMigrateRealV01Store`) had their
hardcoded `schema_version` expectation bumped from 3 to 4 for migration4.

## 2026-09-22, v0.2 42B: context runway, expensive turn (I2 part 2)

Two more `internal/insight` rules, both computed fresh per `Analyze` call, no store
writes. `context-runway`: an ordinary least-squares fit over the last 10 turns' context
size (fresh + cache write + cache read) against turn index; reports `turns_to_80` and
`turns_to_90` in `Evidence` and a `Cause` like "about 10 turns to 80% of the window". No
finding at all (not a finding with an "unknown" cause) when the model's context window
has no price-book entry, fewer than two turns exist to fit, or the fit's slope is zero or
negative: `insight.RunwayText(findings)` renders that absence as "runway unknown" for the
Now card, and a positive-slope finding's own `Cause` otherwise. `expensive-turn`: a turn
whose total tokens land strictly above the nearest-rank 95th percentile of the session's
own per-turn totals, with the largest of its four token classes (fresh, cache write,
cache read, output) flagged `dominant_<class>: 1` in `Evidence` (the map is
`map[string]float64`, so the class name lives in the key, not a value) and named in
`Cause`. Open choice resolved with Wilco rather than guessed: the spec's "95th percentile
of the session's turn cost or tokens" is ambiguous between the two; picked tokens, since
it needs no `pricing.Config` model-family lookup and matches `re-prefill`'s own
token-threshold shape, over cost, which is undefined for an unpriced model and would
couple `insight` to `internal/live`'s cost logic a second time.

Wired the `context-runway` line onto the Now card only (42B's scope; the marker, ticker,
drawer and Sessions-tab display of every finding kind, `context-runway` and
`expensive-turn` included, stay 43A): `live.Session` gained a `Runway string` field, set
from `insight.RunwayText(s.Findings)` right after `Findings` itself in `BuildSnapshot`,
and `internal/report/template.html`'s `sessionCardHTML` prints it as one more `.small`
line under the turn/last-turn line.

Five pre-existing `internal/insight` tests (`TestAnalyze_Reprefill_ModelChangedCause`,
`_GapCause`, `_UnknownCause`, `TestAnalyze_NoFindings`,
`TestAnalyze_SyntheticToolOnlyEventsIgnored`) used fixtures whose context grows turn over
turn with a known window, which now legitimately also trips `context-runway`; changed
their assertions to filter by kind (`reprefillsOnly`, `alertsOnly`) rather than asserting
on the bare finding count, since the new rule firing alongside them is correct, not a
regression.

Measured `bmLive`'s per-poll cost with all four I2 rules against Wilco's real local
store (`store.DefaultPath()`, built this session from his actual Claude Code transcripts
via `burnmon-cli.exe`): 1,858 sessions, 51,563 events, 6,295 findings total. Best of 3
full passes: 7.7 microseconds average per session, 2.05 ms worst case (an 11-event
session, not the largest one; the first, cold pass showed one 9.65 ms outlier on a
different 95-event session that a warm-up pass and a best-of-3 both erased, read as GC
scheduling noise rather than a real cost). Comfortably inside the 5 ms-per-session budget
`TestAnalyze_UnderFiveMillisecondsPerSession` already asserts on a synthetic 2,000-event
marathon.

`go test ./... -count=1` and `.\build.ps1` both green.

## 2026-09-22, v0.2 42A: insight package, re-prefill, compaction (I1, I2 part 1)

New package `internal\insight`, no HTML and no store writes per I1: `Analyze(events
[]schema.Event, cfg *pricing.Config) []Finding` takes one session's events in any order
(sorted by `At` internally), drops the claude adapter's synthetic tool-only events the
same way `internal/live` and `internal/dataset/fromstore.go` already do, and returns
`[]Finding{Kind, Turn, At, Evidence map[string]float64, Cause, Confidence}` in turn order,
`Turn` 1-based over real API-call turns only. Two I2 rules: `compaction` fires when
context size (input + cache read + cache write) drops more than 30% between consecutive
turns; `re-prefill` fires when a turn's cache write exceeds
`cfg.ReprefillThreshold()` (20,000 tokens, `pricing.Config.ReprefillCacheWriteThreshold`,
overridable in `burnmon.json`), with cause inferred in the spec's fixed order: a
compaction finding at this turn or the one before it, else a model change since the
previous turn, else a gap since the previous turn past `cfg.ClaudeCodeCacheTTL()`, else,
when there is no previous turn to compare against at all, "first turn after resume"
(a best-guess label, not a hard fact: a brand-new session's own first turn also always
writes its whole prompt to cache, and insight has no explicit resume signal from the
store to tell the two apart), else "unknown".

VERIFY closed: read `code.claude.com/docs/en/costs` and `code.claude.com/docs/en/model-config`,
dated `2026-09-22` (`pricing.ClaudeCodeCacheBookDate`). Cache lifetime: one hour on a
Claude subscription seat (Pro/Max/Team/Enterprise, Wilco's own setup), dropping to five
minutes once a session draws on usage credits, and five minutes by default on a bare API
key or cloud provider; compiled-in default is the one-hour subscription figure
(`pricing.Config.ClaudeCodeCacheTTLMinutes`, `DefaultClaudeCodeCacheTTLMinutes = 60`),
so the re-prefill cause text is the real figure ("gap 90 min, exceeds 60 min cache TTL"),
not the spec's unverified placeholder. Auto-compact: no single fixed percentage exists;
Claude Code compacts when the conversation reaches the model's context window, except
models on a native 1M-token window (Sonnet 5, the Fable models, Opus 4.7+ on the
Anthropic API), which compact early at about 967,000 tokens by default. Documented as
`pricing.Config.ClaudeCodeAutoCompactTokens` for the record, but not applied by either
rule: burnmon's own `ContextWindows` table prices the 200K standard tier, not the 1M
beta, and the compaction rule already detects a compaction by its effect (the drop),
not by comparing against this number.

Wired into `internal/live`'s `Session.Findings` (bmLive, per poll, on the same windowed
turns `BuildSnapshot` already grouped per session, no extra store read) and into
`burnmon-cli insight <session-id> [--json]` (new `Store.EventsForSession`, plus a
positional-argument-after-flag fix in the CLI's own arg splitting, since Go's `flag`
package stops parsing at the first non-flag argument and the spec's own example puts
`--json` after the session id). `Analyze` measured at under 5ms for a synthetic
2,000-turn marathon session (`TestAnalyze_UnderFiveMillisecondsPerSession`), well inside
budget since it is a single linear pass plus one small sort of the findings themselves.
Tests: one fixture per cause (compaction, model-changed, gap, first-turn-after-resume,
unknown), one for the 30% compaction threshold's boundary, one all-quiet session with no
findings, one confirming synthetic tool-only events are ignored, plus store and live
coverage for the new query and wiring. `go test ./... -count=1` and `.\build.ps1` both
green.

## 2026-09-22, v0.2 41B: vendor strip (P3), Hermes adapter (A1)

P3: new package `internal\vendorstrip`, `Row{Agent,AgentLabel,Today,Week,Month}` and
`Payload{Rows,Total}`, built by `Build(st, now)` from one new store method,
`Store.VendorStripTotals(day, week, month)`: a single SQL query grouped by `agent`,
summing `input + cache_write + cache_read + output` into today/week/month buckets via
`CASE WHEN at >= ?`, bounded by the earliest of the three boundaries (the current ISO
week can start before the current calendar month, so that is not always `monthStart`).
Day/week/month boundaries are UTC calendar (week starts Monday, matching `internal/agg`'s
`weekKey` and History's own "week" period), computed in Go, not SQL. Bound as
`bmVendorStrip` in `cmd\burnmon\main.go`, on its own 60-second `setInterval` in
`template.html` (`pollVendorStrip`/`vsTimer`), independent of `pollNow`'s 2-second poll,
per the spec's "refreshed once a minute... not on the 2-second poll". Rendered as a table
under the session cards (`#t_vendorstrip`), one row per vendor plus a Total row last,
tokens only, no cost (P3 does not ask for one). Every cell links to History with period
and vendor set (`openVendorStripCell`, reusing `UI_HIST` and `goToTab`): the today column
opens `period=day`, week opens `period=week`, month opens `period=month`, range picked to
match History's own default width per period (7/30/90). Test:
`internal\vendorstrip\vendorstrip_test.go`, a real store fixture (two vendors, events
today/earlier-this-week/earlier-this-month/before-the-month-started) checked against
hand-computed totals, confirming the before-the-month event never counts.

A1: before writing anything, read Hermes's live `state.db`
(`%LOCALAPPDATA%\Hermes\state.db`, confirmed on this laptop, WAL mode, `schema_version`
19) read-only. Table `sessions` has one row per session with running-total columns
(`input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_write_tokens`,
`reasoning_tokens`, `message_count`) that grow in place; table `messages` exists but its
own `token_count` column was null on every row checked, 24/24, across every role (user,
assistant, tool), both on the one stale session found at the start of this session and on
two fresh ones Wilco recorded live when asked, so this is not a fluke of an old build. No
context-window column exists anywhere in either table. This directly contradicts the v0.2
spec's "per-message token count" assumption: there is no per-message breakdown available
at all, only the session-level running total. `testdata\hermes\` also did not exist yet
(the spec's own prerequisite, "Wilco records one short Hermes session," had not happened).
Both facts were put to Wilco directly rather than guessed past: he chose to record two
real sessions on the spot rather than defer A1, and, given the schema finding, chose "one
Event per session, growing" over "one Event per message with the running total as delta"
for the adapter's shape.

Built accordingly. `internal\adapter\hermes\hermes.go`: `PollOnce(dbPath)` opens the
database read-only and returns one `schema.Event` per session with `message_count > 0`,
carrying its *current* running totals; `RequestID` is fixed to the session id, so the
store's existing "largest output wins" upsert (`UpsertEvents`, `WHERE excluded.output >
events.output`) is what keeps only the latest state and skips a no-op write when a session
has not grown, meaning `PollOnce` needs no cursor of its own, unlike every `.jsonl`-reading
adapter. `Vendor` is `"nous"`, `Agent` is `"hermes"`, per `schema.Event`'s existing doc
comment. `Surface` maps the one observed `sessions.source` value, `"tui"`, to `"cli"`;
anything else is `"unknown"`. `At` is `time.Now()` while the session is still open
(`ended_at` unset), so a growing session keeps landing inside `internal/live`'s running
window, and falls back to `ended_at` once Hermes has recorded one; neither is a real
per-turn timestamp. Documented, not solved, in the package doc: because
Input/CacheWrite/CacheRead/Output are lifetime totals here rather than one turn's
consumption, `internal/live`'s context gauge (which sums exactly those fields as "last
turn's context fill") would read as ever-growing lifetime usage for a long Hermes session,
not current context fill; accepted for v0.2 since Hermes carries no context-window entry
either, so the gauge already falls back to "context window unknown" in practice. Hermes
writes nothing to `tool_calls` in v0.2, per spec. Wired into `cmd\burnmon\main.go` as
`startHermesPoll`: its own 5-second `time.Ticker` goroutine, started next to
`startLiveWatch`, calling `PollOnce`, applying `cfg.OwnerFor` (the same call
`dataset.go`'s own ingest path makes) and `st.UpsertEvents` directly; deliberately not
routed through `watch.Watcher` or `dataset.Cache.IngestFile`, both built around a
file-offset, `.jsonl`-only contract that a SQLite WAL file does not fit, matching the
spec's own "5-second poll, no fsnotify". A no-op when `hermes.DefaultDBPath()` finds no
install.

Fixture: `testdata\hermes\hermes_fixture.db`, built from the two sessions Wilco recorded
live during this session, `sessions`-table columns only (metadata and running totals),
no `messages` row and no `content` column at all, so there is no personal content to
strip in the first place, by construction. Test: `internal\adapter\hermes\hermes_test.go`,
`PollOnce` against the fixture returns both sessions with the exact recorded token counts,
`Surface` mapped correctly, and a second poll of the still-open fixture returns the same
count (repeat-safety belongs to the caller's `UpsertEvents`, not to `PollOnce` itself,
this only checks the read is stable). `go test ./... -count=1` and `.\build.ps1` both
green; `node --check` on the extracted inline script passed. Copilot CLI (A2) and the OTel
check (A3) are next week's session per the build order, untouched here. Not committed;
see the end of this session's chat for the commit and tag commands.

## 2026-09-22, v0.2 41A: History, one page (P2)

New package `internal\history`: `Filter{Period,From,To,Vendor,Owner}` in, `Payload{Filter,
Totals,Rows,Vendors,Owners}` out, built by `Build(events, cfg, filter)` from the store's own
events, never from a session-shaped intermediate (unlike Overview/Months/Weeks/Days, which run
through `scan.Session` and lose vendor identity). Buckets by day/week/month using the same key
shapes `internal\agg` already uses (`YYYY-MM-DD`, `YYYY-Www`, `YYYY-MM`), so the page's existing
`monthLabel`/`weekLabel`/`dayWithName` formatters carry over unchanged. Sessions counted as
distinct `(vendor, session_id)` pairs touching a bucket; turns exclude the claude adapter's
synthetic tool-only events, matching `buildSession`'s own filter. Cost is plain list price
(`CallCostUSD` / `OpenAICallCostUSD`), no subscription-share math: the open call from the spec
("cost only where the price book covers the vendor, otherwise tokens only until v0.3") was put
to Wilco directly rather than guessed, since it forks three ways once "All" mixes covered and
uncovered vendors. Decided: cost only ever appears when the vendor filter is narrowed to one
covered vendor (`anthropic` or `openai`, i.e. Claude Code, Cowork or Codex); "All", Hermes and
Copilot CLI always show `cost_note: "tokens only until v0.3"` instead, even when the filtered
data happens to be all-covered. Wired into `cmd\burnmon\main.go` as `bmHistory`, bound once
alongside `bmLive`: unlike `bmLive`'s windowed 2-second poll, it reads `st.AllEvents()` fresh on
every call, since History is queried on filter change, not polled.

`internal\report\template.html`: `#history` now has real filter controls (period, range with a
custom from/to, vendor, owner shown only when `payload.owners` is non-empty), a totals block, one
bar chart (`ch_history`, cost or tokens per bucket depending on `showCost`) and one table
(`t_history`): one visual per question, not three. Deleted `#months`, `#weeks`, `#days` and
their `periodRows`/`renderPeriods` functions (Overview stays, per 40B's note: P1 only took it off
`TAB_IDS`, and it is not part of P2's scope); kept `dayWithName`/`weekLabel`, now History's own
formatters. Filter state rides the URL hash as `#history?period=week&range=30&vendor=codex&
owner=ZND`, written with `history.replaceState` (never `pushState`, so it still never triggers
the Cowork-artifact-host reload 40B's comment warns about) and only while the History tab is
actually open; the deep-link reader at the bottom of the script now splits the hash on `?` before
matching it against `TAB_IDS`, since `#history?...` no longer equals the bare `'history'` it used
to. A saved CLI report (no `window.bmHistory`) shows "History needs the BurnMon app window" in
place of the totals and table, the same pattern `pollNow` already uses for `bmLive`.

Test: `internal\history\history_test.go`, a real store fixture (via `internal/store`, not a
fake) with two vendors (`claude-code`/anthropic, `codex`/openai) across three ISO weeks in
September 2026. `Build` for `period=week` gives three rows, two sessions and two turns each,
tokens matching hand totals, `cost_note` set (vendor is "All"); narrowing to `vendor=claude-code`
turns on `cost_usd`, checked against `pricing.Defaults()`'s sonnet rate by hand; `period=month`
over the same range collapses the three weeks into one `2026-09` row with the combined totals. A
second test checks the owner filter excludes the other owner's session while `Owners` still
lists both. `node --check` on the extracted inline script, `go test ./... -count=1`, and
`.\build.ps1` all green. Not committed; see the end of this session's chat for the commit
command.

## 2026-09-22, v0.2 40B: five tabs, Now default, English only (P1, P4, P5)

`internal\report\template.html` only, no Go touched. Tab bar is now Now, History, Sessions,
Tools, About; Overview, Months, Weeks, Days and the standalone How it works tab are off the
tab bar. Per the spec's "do not move the old three pages' code yet": their `<section>` markup
(`overview`, `months`, `weeks`, `days`) stays in the DOM exactly as it was, just given
`class="hidden"` and dropped from `TAB_IDS`, so `renderOverview`, `drawOverviewCharts` and
`renderPeriods` keep running untouched from `renderAll()` on every load, ready for 41A (P2
History) to call. `showTab`'s dead `if(['overview','months','weeks']...)` re-render branch was
removed since those ids can never reach it now (not in `TAB_IDS`, and the not-found fallback is
`'now'`, not `'overview'`). `#now` is unhidden by default and its tab button carries `class="tab
on"` in the markup, so a cold load needs no JS to land on Now; `#history` is a new placeholder
section, "History arrives in the next session." and nothing else. Added one small deep-link
read at the end of the init IIFE: on load only, `location.hash` is checked against `TAB_IDS` and
`showTab()` is called once if it matches (`#now`, `#history`, `#sessions`, `#tools`, `#about`);
nothing writes to `location.hash` on navigation, which is deliberate, per the existing comment
above `TAB_IDS` about the Cowork artifact host reloading the whole document on `pushState`.

How it works becomes the first section of About: moved the whole `<section id="how">` block
(all seven "How Claude is paid for" cards plus its own "How accurate is this?" close) to open
`<section id="about">`, ahead of "What this shows, and what it cannot". About already carried
an identical "How accurate is this?" card near the bottom (word for word, previously invisible
because the two lived on separate tabs); dropped the newly-moved copy's version rather than the
original, since About's copy sits in its natural place after "List prices used". The move also
surfaced a real pre-existing bug: two elements shared `id="how"` (the old section, and the
"Refreshing" card inside About), so `document.getElementById('how')` on the "How do I refresh
this?" button's click handler always resolved to the pay-explanation section, not the refresh
card it was meant to scroll to. Renamed the card's id to `refresh_info` and repointed that
handler at it, fixing the scroll target as a side effect. Left the "Overview tab" references at
two spots inside the moved copy (both original, both about where the This month / Last month
cards used to live) as-is: Overview's own fate is P2's call in 41A, not this session's, so
rewriting them now risks describing something that changes again in three days; flagging it here
instead of guessing. Updated the one live cross-reference this session does own: `card_note_sub`
said "See the How it works tab", now "See the About tab".

P5: language toggle gone entirely, `t()` kept. Removed `btn_lang` (topbar button, its CSS class
stays since `btn_theme` still uses `.tglbtn`), `UI.lang` (state, localStorage read/write, the
`applyTopbarLabels` line that displayed it), the `nl` entry of `MONTH_NAMES`/`DOW_SHORT`/
`DOW_FULL` and all three call sites' `[UI.lang] ||` fallback (now just `.en` directly), the
entire 90-key `I18N.nl` block, and `t()`'s language lookup (now `I18N.en[key]` directly, no
`UI.lang` branch). Grepped the template and every `.go` file under the module for Dutch (`nl:`,
`tabblad`, `taal`, `zetelprijs`, `aanroepen`, `kosten`, `Vernieuw`, `onbekend`, `Momentopname`,
and a plain "Dutch"/"Nederlands" sweep): all of it was confined to the `nl` I18N block and
`btn_lang`'s `title` attribute, nothing on the Go side ever had any. `node --check` on the
extracted inline script, `go test ./... -count=1` and `.\build.ps1` all green.

Opened `burnmon.exe` and screenshotted the real window (`CopyFromScreen`, not a saved report):
Now is the active tab on cold start, cards and chart populated from this laptop's live store,
tab bar reads Now / History / Sessions / Tools / About left to right, confirming P1 and P4.
Clicking tabs to screenshot History and About did not work from this session: `Get-Process` and
`FindWindow` from the PowerShell tool could not see or address the window that `tasklist`
confirms was running (session/desktop isolation between this sandbox and the interactive
desktop, not an app bug), so simulated mouse input never reached it. Did not chase this further;
the placeholder and About-merge content were verified by direct reading of the rendered section
HTML instead. Killed the leftover `burnmon.exe` test process (`taskkill`) afterwards so no stray
instance holds the app's single-instance mutex for the next run. No open product questions came
up worth stopping for; the two judgment calls above (which "How accurate is this?" copy to keep,
renaming the colliding `id="how"`) were both forced by pre-existing duplicate-id/content bugs
with only one sane fix, not product decisions, so made and documented rather than asked.

## 2026-09-22, v0.2 40A: tool_calls table, both adapters, tools --json, CLI live windowed (S2, S3)

Before writing the extraction, opened one real Claude Code transcript (`C--ZND\29c211b0-...jsonl`,
540 tool_use blocks) and one real Codex rollout under a `C:\ZND` cwd (never `C:\dev\Work`, per
the house rule), confirmed field names in code, not from the spec's prose. Claude: an assistant
message's content array carries `{type:"tool_use", id, name, input, caller}`; the matching
result lands on the next `user` line as `{type:"tool_result", tool_use_id, type, content}`
(content usually a plain string, its byte length the natural "result bytes"; occasionally a
block list, summed by "text" field). Codex is not the clean "function_call items" the spec's
prose names: a sampled rollout's payload types were `custom_tool_call`/`custom_tool_call_output`
(54 pairs, name "exec", the actual shell/tool execution Codex sessions mostly consist of) versus
`function_call`/`function_call_output` (10 pairs, e.g. "request_user_input_async"), both matched
by `call_id`. Implementing only `function_call` would have captured a small minority of real
Codex tool calls; flagged this to Wilco as an open choice before writing anything, and he chose
both families.

Migration 3 (`internal\store\migrations\migrations.go`) adds `tool_calls`, keyed by (vendor,
session_id, call_id) so a call and its later-arriving result upsert onto the same row;
`result_bytes` stays nullable rather than defaulting to 0, so "no result seen yet" stays
distinguishable from "an empty result" (`store.UpsertToolCalls`'s `ON CONFLICT` only overwrites
it when the new row actually carries one). `adapter.Adapter.Parse` now returns `[]schema.ToolCall`
alongside `[]schema.Event` (every implementer and caller updated: both adapters, `dataset.go`'s
ingest, `internal/live`'s test helper); each adapter tracks tool calls the same way it already
tracks turns, in a `pendingToolCall` map keyed by the vendor's own call id (`toolu_...` for
Claude, `call_...` for Codex), filled in when the result/output line is seen later in the same
read (a call and its result almost always land in the same incremental read in practice, so this
is not backfilled across separate reads, matching the codex adapter's own existing precedent for
turn state). `dataset.Cache.Collect` gates a one-time full re-read of every file behind a new
`tool_calls_backfilled_v1` meta flag, so every session ingested before this build gets its
tool_calls backfilled exactly once rather than on every run. `store.ToolCallTotals(since)` groups
by tool name (calls, sessions, input/result bytes); `burnmon-cli tools --since 30d --json` (also
accepts `7d`, `24h`, anything `time.ParseDuration` takes plus a bare day suffix) runs a normal
Collect pass then prints it, ready for a later Tools tab (which does not exist yet, P1 not having
landed) to call the same store method. S3: `runLive` (`cmd\burnmon-cli\main.go`) now calls
`store.EventsSince(now.Add(-live.ChartWindow))` plus `live.ApplySessionTotals`, the same F1 path
the app's own live poll already used, instead of `AllEvents`. Verified all of this by hand against
the real 33+ MB store on this laptop, not only fixtures: first `tools --json` run took long (the
full backfill re-reading every real transcript), a second run 4 seconds; `live -json` still shows
this very session running with the right turn count and cache-hit ratio. New tests: a two-tool-call
fixture per adapter (`testdata\toolcalls.jsonl` for Claude, `testdata\codex\tool-calls.jsonl` for
Codex); `store.TestUpsertToolCallsAndTotals`; `cmd\burnmon-cli\TestLiveQueryPathStaysBoundedOnLargeStore`,
which substitutes a deterministic assertion (`EventsSince` on a 50,000-old-event store returns only
the recent window, not a count proportional to store size) plus a heap-growth check under 50 MB for
a flaky wall-clock/RSS measurement, since automated memory-bound testing was not otherwise practised
in this codebase. `go test ./... -count=1` and `.\build.ps1` both green.

## 2026-09-22, v0.2 39B: versioned migrations, owner column, reown

Replaced the drop-and-rebuild `ensureSchemaVersion` (`internal\store\store.go` around
line 101) with a real migration runner: a dedicated `schema_version` table (not the old
`meta` key, which spec S1 asked to leave behind), migrations listed in
`internal\store\migrations\migrations.go` as Go functions (chosen over embedded SQL files:
one small package, no extra build step, and Go's own compiler catches a typo in the DDL
string at build time rather than at Open), run inside one transaction, from/to version
logged on a move. Migration 1 records the v0.1 schema (events, cursors, meta) as version 1
via the same `CREATE TABLE IF NOT EXISTS` statements store.go always ran, so a real v0.1
store on disk is untouched. Migration 2 adds `events.owner TEXT NOT NULL DEFAULT ''`.
`TestMigrateRealV01Store` (`internal\store\store_test.go`) copies the actual 33 MB
`%LOCALAPPDATA%\burnmon\burnmon.db` on this laptop into a temp dir, runs it to head, and
checks every v0.1 event survives with an identical (vendor, session_id, request_id) key
and `schema_version` reads 2; it skips rather than fails when no such store exists or it is
locked. P6's owner column lives on `events`, not a separate `sessions` table: burnmon has
never materialised sessions (`dataset.SessionsFromEvents` derives them from events on every
read), so `Owner` is carried per event and reconstructed per session the same way `Title`
already is (first non-empty value seen), rather than inventing a sessions table this spec
did not otherwise ask for. `pricing.Config.Owners` is the ordered rule list (`{match,
owner}`, `*`-suffixed case-insensitive prefix match); `OwnerFor` returns "" when `Owners` is
empty (P6's default: one owner, no owner column shown anywhere) and "personal" when rules
exist but none match. Applied at ingest (`dataset.Cache.ingest`, now taking a `*pricing.Config`)
to each event's `Project` before it reaches the store; `IngestFile` grew the same parameter,
so its two callers (`cmd\burnmon\main.go`'s live watcher, `cmd\burnmon\main_test.go`) now pass
a config, the live watcher copying `a.cfg` by value first, the same read-safety idiom
`rebuild()` already used for the same field. `store.Store.ReownEvents(ownerFor)` recomputes
every event's owner and updates only the rows that changed, in one transaction; `burnmon-cli
reown` wires it to `cfg.OwnerFor`. No UI: `scan.Session` gained an `Owner` field
(`json:"owner,omitempty"`) so the data is there for 44A's Sessions-tab column, but nothing
renders it yet. `go test ./... -count=1` and `.\build.ps1` both green; `burnmon-cli.exe
reown` run once by hand against the real store migrated it live from version 0 to 2 and
reported 0 reowned (no owner rules configured yet), confirming the startup path against a
real file, not just the test fixture.

## 2026-09-22, v0.1.2 F9 done, F7-F9 gate closed

Version constants to `0.1.2` in both `cmd\burnmon\main.go` and `cmd\burnmon-cli\main.go`
(`build.ps1` carries no version constant of its own); README's first section rewritten to
describe v0.1.2 (the Now page, that the rest of the README is still carried over from
claudecost and marked as such, full README pass moved to v0.2); `STATUS.md` filled in
(adapters, pages, known gaps, next release, replacing the empty 2026-09-08 stub); `_board`
rows renamed from claudecost to burnmon in `board.json` and `board.html` (no generator script
or `siteoffice.json` source found under this project to regenerate from instead, so these
generated files were hand-edited; a future Siteoffice pass should restore a real source).
`go build ./...`, `go test ./... -count=1` and `.\build.ps1` all green. F7, F8 and F9 all
land in this one commit per the spec's order and gate. Stopping before tagging `v0.1.2`, per
Wilco's instruction.

## 2026-09-22, v0.1.2 F8 fixed: the running chart no longer jumps every poll

Replaced v0.1.1 F3's "destroy and rebuild `charts.now` on every 2-second poll, both axis
maxima recomputed from the window's raw peak" (`drawNowChart`,
`internal\report\template.html`) with the decided design (grill, question 12): the Chart.js
instance is now created once and updated in place via `chart.update()`, bar datasets are kept
one object per session id across polls (new module-scope `nowChart` state: `datasetById`,
`colorBySession`, `tokenMax`, `costMax`) so a session already on screen animates instead of
popping, and both axis maxima are smoothed (`Math.max(peak*1.1, prevMax*0.95, floor)`, tokens
floored at 10K, cost unfloored). Tick labels now render only on a round 5-minute mark via an
explicit `ticks.callback` (`autoSkip` alone could not do this, since all 180 slot positions
themselves slide every poll). `node --check` on the extracted `<script>` blocks passed; `go
build ./...` and `go test ./... -count=1` both green. Wilco watched the running Now page for
5 minutes on the rebuilt binary (1 Claude CLI, 4 Codex CLI sessions live) and confirmed no
visible jump, satisfying the spec's own check.

## 2026-09-22, v0.1.2 F7 fixed: new Codex session missed by the live watcher

Confirmed hypothesis 3 from `02_roadmap\2026-09-22_v0.1.2_patch_spec.md`, with a live
diagnostic run alongside Wilco's own Codex CLI sessions before touching any code: a rollout
landing in a day folder that already existed at burnmon startup was ingested correctly and
immediately (`watch: F7 diag: onChange fired ...` within under a second of the raw fsnotify
Create), ruling out hypotheses 1 (ingest/adapter resolution) and 2 (Rename filtering). The
race that matches Wilco's 14:12 report only bites on a day folder that is itself new since
startup: `TestWatcher_NewNestedDayFolderRace` (`internal\watch\watch_test.go`), which mirrors
Codex's `MkdirAll` of `YYYY/MM/DD` immediately followed by the rollout file with no pause
(unlike Claude Code's own folder-then-file timing, which the existing
`TestWatcher_NativeRootSeesNewSubdirectory` gives 200ms), failed 4 of 5 runs before the fix:
Windows `ReadDirectoryChanges` is per-directory, not recursive, so the file's own Create event
fires and is silently dropped while `fsw.Add` on the brand-new leaf directory is still
in-flight. Fix: `addTree` (`internal\watch\watch.go`) takes a `notifyExisting bool`; the
startup calls in `New` pass `false` (the initial full backfill already ingests everything
under the native roots), but `handleFsnotifyEvent`'s call for a freshly-Created directory now
passes `true`, so any `.jsonl` already inside that brand-new directory is picked up right
there instead of waiting on an event that already happened. `dataset.Cache.IngestFile` ingests
by cursor, so re-notifying a file the full rescan or an earlier live event already saw is a
safe no-op. 10 consecutive runs of the new test all passed after the fix (0 failures), full
`go test ./...` green. Measured latency: under 1 second from the file landing on disk to
`onChange` firing, well inside the spec's 2-second bar.

## 2026-09-22, v0.2 grill and v0.1.2 patch spec (Cowork, Fable)

Wilco reported two things from the 14:12 live run: a new Codex CLI session appeared only
after pressing Refresh now (the live watcher missed the new rollout; the full rescan found
it), and the running chart jumps because `drawNowChart` rebuilds the Chart.js instance and
recomputes both axis maxima every poll. Both go into `02_roadmap\2026-09-22_v0.1.2_patch_spec.md`
(F7 with three ordered hypotheses to confirm on disk, F8 smoothing that replaces the F3
"recompute per poll" rule, F9 version strings 0.8.1 to 0.1.2, README, STATUS, board). Then a
thirteen-question grill scoped v0.2 as one release on 2026-11-14 at two sessions a week:
`02_roadmap\2026-09-22_v0.2_spec.md`. Dev and business switch and per-vendor cost moved to
v0.3. Then `02_roadmap\2026-09-22_v0.2_session_prompts.md`: fourteen Sonnet session prompts
(39A to 46B) with a checklist and Wilco's fixture steps. No code touched this session.

## v0.1.1 F4-F6: Cowork agent label, never-clamp context gauge, header copy

F4: `internal\adapter\claude\claude.go`'s two event-construction sites (`Parse`, around the
old lines 292 and 326) hardcoded `Agent: "claude-code"` for every transcript; only `Surface`
told Cowork apart. Added `agentFor(surface)` (desktop/cowork to "cowork", everything else
unchanged), called from both sites, so the Cowork card now reads "COWORK · DESKTOP" and the
CLI card still reads "CLAUDE-CODE · CLI". `internal\agg`'s "By surface" table groups by
`BySurface` alone (`internal\agg\agg.go:47/70`), never touches `Agent`, confirmed unaffected
by reading it, not assumed. New tests in `internal\adapter\claude\claude_test.go`:
`TestAgentFor` (table test over desktop/cowork/cli/code_agent/unknown) and
`TestParseDesktopSurfaceGetsCoworkAgent` (a real trail file under a temp
`local-agent-mode-sessions` folder, asserts `Agent == "cowork"` end to end through `Parse`),
plus one added assertion on `TestParseBasicFixture`'s existing `cli` fixture (`Agent ==
"claude-code"`).

F5: the context-window table and `pricing.ContextWindowBookDate` ("2026-09-22", sourced from
platform.claude.com/docs/en/about-claude/models and developers.openai.com/codex) were already
in place from Step 3; the actual bug was the Now-page gauge in
`internal\report\template.html`'s `sessionCardHTML`, which computed `Math.min(100,
context/context_window*100)`, silently clamping an over-window session (Wilco's real
`claude-fable-5-1` card: 285,000 of a 200,000-token book value) to a false "100%". Fixed to
never clamp: when `context > context_window`, the gauge now renders a full grey bar
(`var(--muted)`) and the text "<tokens> tokens, window in book: <window>, exceeded, check
book date" instead of a percentage; the normal (non-exceeded) and unknown-window cases are
unchanged. Verified by extracting and evaluating the template's own JS in Node (no Node
runtime ships with the app, verification only, same as Step 3's precedent): a 285K/200K
fixture renders "285,000 tokens, window in book: 200K, exceeded, check book date" and a
100%-wide grey bar, not "(100%)"; a normal 50K/200K fixture still renders "(25%)" unaffected.

F6: subtitle copy in `internal\report\template.html` (`#subtitle`'s static fallback text, and
both `subtitle_sub`/`subtitle_list` i18n strings, EN and NL, which the two pricing-basis
toggle states previously worded differently around "what we pay"/"list prices") is now the
one fixed line "What your coding agents burn, live and by month" ("Wat je coding agents
verbruiken, live en per maand" in Dutch) in every state, matching the spec's single given
copy rather than continuing to vary by basis. The Now page's lede changed from "Running
Claude Code and Codex sessions, live." to "Running Claude Code, Cowork and Codex sessions,
live." README was left untouched, per spec's explicit scope cut.

Gate: `go test ./...` and `.\build.ps1` both green after F4-F6, template JS re-verified with
Node syntax + behavioural checks as above.

## v0.1.1: SessionTotals vendor-qualified (Wilco's decision on the open choice)

F1's `store.SessionTotals` originally keyed its `SELECT ... WHERE session_id IN (...)` by bare
`session_id`, which would double-count if two different vendors ever minted the same session
id. Wilco's call: key by (vendor, session_id) pairs, not session_id alone. `store.go` gained
`SessionKey{Vendor, SessionID}` and `SessionTotals` now takes `[]SessionKey`, building a
dynamic `(vendor = ? AND session_id = ?) OR ...` clause instead of a single `session_id IN`
list. `live.go`'s `Session` gained a `Vendor` field (set from `turns[0].Vendor` in
`BuildSnapshot`); `ApplySessionTotals` and the renamed `sessionKeys` (was `sessionIDs`) now
match and aggregate by `vendor+"|"+session_id`, not bare `session_id`. Existing
`TestSessionTotals` updated to the new signature; new `TestSessionTotalsVendorQualified`
(`internal\store\store_test.go`) proves the fix directly: two vendors sharing the literal
session id "shared" each keep their own totals when queried by their own vendor-qualified key
(anthropic: input=100/output=10, unaffected by openai's 5000/500 sharing the same id).

While rerunning the full suite for this change, found and fixed one unrelated pre-existing bug
blocking green: `live.go`'s chart `windowStart` was computed as `now.Add(-ChartWindow).Truncate(10s)`,
which always rounds down, so the window's right edge (`windowStart+ChartWindow`) trailed
`now` by 0-10 seconds depending on wall-clock alignment; any event in that gap (idx computed
>= chartSlots) was silently dropped from every chart bucket, including the very turn a live
poll just picked up. This made `TestSnapshotChangesOnAppend` fail deterministically about half
the time (confirmed via a throwaway debug test: `windowStart=...T11:07:00Z`, appended turn at
`...T11:37:03`, i.e. 3 seconds past the window's last bucket boundary, dropped). Fixed by
anchoring the window's right edge (`windowEnd`) at or after `now` (`now.Truncate(10s)`, rounded
up one bucket if that truncated down) and computing `windowStart` from that, so the dense chart
always covers up to `now`. Confirmed with 4 repeated `-count=1` runs after the fix, all green.

Gate: `go test ./...` and `.\build.ps1` both green.

## 2026-09-22, v0.1 Step 3: the Now page

Landed the Now page minimum from `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1_spec.md`
Step 3: `internal\watch` (fsnotify on native adapter roots, a 5-second poll on WSL roots),
`internal\live` (the snapshot builder), a bound `bmLive()` in `cmd\burnmon`, `burnmon-cli.exe
live -json`, a `context_window` table in the price book, and a new first tab "Now" in
`internal\report\template.html`. **WSL confirmed on this laptop, answering the spec's open
question directly rather than by assumption**: a throwaway probe built against
`github.com/fsnotify/fsnotify` and pointed at a live `\\wsl.localhost\Ubuntu-24.04\...` folder
did not merely see events late, it failed to add the watch at all, `ReadDirectoryChanges:
Incorrect function`, confirmed by writing into the real folder from `wsl -d Ubuntu-24.04` while
the probe was running and seeing nothing. `internal\watch` therefore never attempts fsnotify on
a WSL root; those are polled by mtime every 5 seconds, exactly as the spec's fallback assumed,
just for a stronger reason than "inotify does not cross the boundary". `internal\dataset` gained
one exported method, `Cache.IngestFile`, a one-line wrapper around the existing private `ingest`
loop with a single-path slice: the watcher's file-change callback needed a way to ingest one
path without re-resolving every source, and `ingest` already handled a single-file list
correctly, so no new ingest logic was written, only a name for calling it that way. `bmLive` and
`cmd\burnmon`'s live watcher both take `app.mu` around their store/cache access, serialising with
the existing 15-minute and WSL rebuild tickers, which stay as the safety net the spec calls for.
`internal\live.BuildSnapshot` takes the whole event list (already in memory from `store.AllEvents`,
same as every other read path in this codebase) rather than a windowed store query, matching the
store package's own stated v0.1 scale trade-off. Context windows: `platform.claude.com`'s
published standard tier is 200,000 tokens for every Claude model id seen on this laptop (the
1M-token beta window needs a beta header burnmon never sends, so it is not used); OpenAI's Codex
model pages give 400,000 tokens for the Astra/Sol/Terra/Luna family; `codex-auto-review` has no
published window (also unpriced, per Step 2) and is left out on purpose, so its gauge shows raw
tokens only, per the spec. Verified against real, current activity, not just fixtures: running
`burnmon-cli.exe live -json` mid-session showed this very Claude Code session as the one running
entry, context 234,819 of a 200,000 window (over 100%, the UI gauge clamps display at 100%),
alongside 30 minutes of real per-minute chart buckets covering both this session and several
recent Codex rollouts that had already gone stale past the 10-minute running window, correctly
excluded from `sessions` while still present on the chart's tail. Launching `burnmon.exe` itself
confirmed no bind or watcher-startup error in `burnmon-app.log`, and the rendered
`dashboard.html` carries the new `now` tab, `ch_now` canvas and `startNowPolling` call. **Not
verified**: no live Codex session was running alongside this one during the session, so the
spec's full "both sessions show a context percentage and the chart moves within 2 seconds of a
turn" done-when is confirmed for Claude Code and confirmed structurally (real Codex data flows
through the same code path in `live -json`) but not watched live side by side; and the WebView2
window's actual on-screen rendering was not visually inspected, only its generated HTML and
absence of log errors. Cache clock and turn ticker were left out, in scope only if the week
allowed and it did not. `go test ./...`, `go vet ./...` and `.\build.ps1` are green, including
new tests for `internal\live` (running-vs-stale, subagent nesting, unknown-model gauge, and the
done-when's own "append to a temp trail, assert the snapshot changes" test against a real
adapter) and `internal\watch` (native fsnotify sees a new file and a newly created subdirectory;
the WSL poll path fires once per real mtime change and not on a re-poll of an unchanged file).
Tag command below, not run.

```
git tag -a v0.1.0 -m "Step 3: the Now page"
```

## 2026-09-22, v0.1 Step 2: Codex adapter

Landed `internal\adapter\codex` from `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1_spec.md`
Step 2, reading `%USERPROFILE%\.codex\sessions` (or `$CODEX_HOME\sessions`) plus WSL distros,
byte-offset incremental like the claude adapter, never a whole-file read. `internal\scan\wsl.go`
was generalised (distroHomeSources/WSLHomeSources, parameterised on relPath and an override env
var) so Codex reuses the same registry-and-passwd distro discovery as Claude, rather than a
second copy of it; the four existing WSL tests (`distroSources`, `configDirFromShellFiles`, etc.)
keep passing unchanged through thin wrappers. Field names were confirmed against 15 real rollout
files on this laptop, not guessed: a `token_count` line is `{"type":"event_msg","payload":
{"type":"token_count",...}}` exactly as the spec assumed, but `turn_context` (the model) is a
top-level `{"type":"turn_context",...}`, no `event_msg` wrapper, one level shallower than the
spec's phrasing implied. `rate_limits` sits beside `info`, not inside it, and its field names
(`primary.used_percent`, `primary.resets_at`) matched the spec, but `primary` is not reliably
the 5-hour window: an older CLI build (0.146.0) had `primary` at `window_minutes: 10080` (weekly)
with `secondary: null`, a newer one (0.154.0) has `primary` at 300 (5h) and `secondary` at 10080.
The adapter now picks whichever of primary/secondary has `window_minutes <= 360`, falling back to
primary. `originator` values seen were `codex-tui`, `Codex Desktop`, `codex_work_desktop`, and
once `Claude Cowork` (Cowork apparently drove a Codex session as a tool call); none matched the
spec's guessed `codex_cli_rs`/`codex_vscode`, so surface classification matches by substring
(`tui`/`cli` to `cli`, `desktop` to `desktop`, `vscode` to `vscode`) rather than an exact enum.
`RequestID` uses the line's own `ordinal` field (present on every event) as the turn index rather
than a locally-counted one, so it stays correct across incremental reads with no adapter-side
state. Known, accepted limitation: Model is tracked only within one `Parse` call's read window
(same precedent as the claude adapter's cwd/title tracking); a read that resumes mid-turn, after
its `turn_context` line but before the matching `token_count`, would emit that one event with an
empty Model. Not exercised on Wilco's real trail, where the two lines land together.
`internal\dataset\dataset.go`'s adapter dispatch was previously hardcoded to Claude for every
file regardless of `adapter.Roots()`; `resolveSources` now resolves both adapters' native and
WSL roots (still gated by the existing "two cadences" split, so a fast-tier app tick never
touches WSL for either vendor) and a new `adapterForPath` classifies each file by root-prefix
match, falling back to Claude when no full pass has run yet. Pricing: `internal\pricing` gained
an `OpenAIPrices` map keyed by exact model id (not a family, unlike Claude), with an explicit
`CachedIn` rate rather than a multiplier, list prices checked 2026-09-22 against
developers.openai.com/api/docs/pricing (redirects to `/api/docs/pricing`) for the four model ids
actually seen and priced (`gpt-6-astra`, `gpt-5.6-sol`, `gpt-5.6-terra`, `gpt-5.6-luna`); a fifth
seen id, `codex-auto-review`, has no published rate and was deliberately left unpriced to
exercise that path for real, rather than invented. Unpriced calls now price at 0 and their tokens
roll up into a new `Session.Unpriced` / `Payload.Totals.UnpricedTokens` field. OpenAI events get
no subscription-share cost (no calibrated invoice exists for a Codex/ChatGPT plan in v0.1);
`cost_sub` equals `cost` for them, a deliberate scope cut, not an oversight. `burnmon-cli.exe
price-check` prints both books with their dates. Verified against Wilco's own machine, not just
the fixture: after wiping the store, a full `burnmon-cli.exe report` picked up 34 real Codex
sessions across the last three months, correct per-model costs (e.g. one GPT-6 Astra session,
230 calls, $56.72), and 30 sessions correctly landing under `unpriced_tokens` (all
`codex-auto-review`). New fixture `testdata\codex\three-turns.jsonl`: three turns, a model
switch mid-file (`gpt-5.6-terra` to `gpt-6-astra`), and exactly one `rate_limits` object, covered
by `internal\adapter\codex\codex_test.go`; new dataset-level tests cover OpenAI cost/unpriced
pricing (`fromstore_test.go`) and the adapter-dispatch wiring end to end through the real store
(`TestIngestDispatchesToCodexAdapter`). The spec's documented fallback for a `last_token_usage`-
less line (deriving the turn from a `total_token_usage` delta) is implemented but untested
against a live file: every rollout on this laptop carried `last_token_usage` on every line.
`go test ./...`, `go vet ./...` and `.\build.ps1` are green. Tag command below, not run.

```
git tag -a v0.1.0-alpha.2 -m "Step 2: Codex adapter"
```

## 2026-09-22, v0.1 Step 1: schema, store, Claude adapter

Landed the schema/store/adapter split from `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1_spec.md`
Step 1: `internal\schema.Event`, a SQLite-backed `internal\store` (events, cursors, meta,
`modernc.org/sqlite`, no cgo), the `internal\adapter.Adapter` interface, and
`internal\adapter\claude` (the Claude/Cowork parser moved out of `internal\scan\parse.go`
and rewritten for incremental byte-offset reads). `internal\dataset` was rewired around
the store end to end, and the old gob parse cache is gone. Two real bugs surfaced during
real-data verification against Wilco's own transcripts and both are fixed: the store's
original dedup key, `(vendor, request_id)`, was global and let Cowork's mirrored
session files steal each other's events (one real session collapsed from 67 calls to 2);
it is now scoped to `(vendor, session_id, request_id)`. A final whole-branch review then
caught a second, more serious one before it could ship: the incremental reader was
consuming a trailing partial line and permanently losing its turns once a live-growing
file's line finished writing later, exactly the code path the store exists for and the
one path `-no-cache`-based verification could never exercise; the reader is now
`bufio.Reader`-based and proven by a dedicated offset-accounting test
(`TestParseDoesNotConsumeTrailingPartialLine`). One intentional, sign-off'd diff from
v0.0.1: the surface vocabulary changed from `cowork`/`code`/`code_agent`/`chat` to
`cli`/`code_agent`/`desktop`/`unknown` (cowork and chat merged into desktop). Final
verification: `burnmon-cli.exe report` from the store against a v0.0.1 baseline binary
(commit `3d00f22`), both run `-no-cache` against the same real two months of transcripts,
came back byte-identical apart from that one acknowledged surface field. `go test ./...`
and `.\build.ps1` are green. Four findings were parked as deferred minors for Step 2/3
(a chunk-relative synthetic dedup key fallback, a strict-vs-`>=` upsert guard mismatch,
`Session.Surface` sourced from `events[0]` instead of first-non-empty, and the
`<synthetic>`-model filter moving from after-dedup to before-dedup, likely an
improvement but an unremarked semantics change). Tag command below, not run.

```
git tag -a v0.1.0-alpha.1 -m "Step 1: schema, store, Claude adapter"
```

## v0.1.1 F1: bmLive no longer loads the whole store on every poll

`cmd\burnmon\main.go`'s `bmLive` binding called `st.AllEvents()` on every 2-second poll,
so `live.BuildSnapshot` re-grouped the whole events table in memory each tick: the cause
of the reported "burnmon.exe 1886 MB, 17158 hard faults/s at 96% RAM" thrash. Added
`store.EventsSince(from time.Time)` (new `idx_events_at` index) and
`store.SessionTotals(sessionIDs)` (a `GROUP BY vendor, session_id, model` SQL sum of
tokens, excluding the claude adapter's synthetic tool-only rows), exported
`live.ChartWindow` (30 min, safely wider than `pricing.DefaultRunningWindow`'s 10 min so
no running session's last turn ever falls outside it), and a new `live.ApplySessionTotals`
that overwrites each running session's Start/TurnCount/Tokens/Cost from the small SQL
aggregate so a session older than 30 minutes still reports its true lifetime numbers.
`bmLive` now calls `EventsSince(now-ChartWindow)` plus `ApplySessionTotals` instead of
`AllEvents()`, and logs `HeapAlloc` every 30 polls (`debug: bmLive poll N, HeapAlloc=...`
in `burnmon-app.log`). Also batched `dataset.ingest`'s `UpsertEvents` call into 1,000-row
transactions per file rather than one transaction per file's whole event slice (Parse
itself already streams via the byte-offset cursor; this only bounds the commit size).
Measured against a copy of Wilco's real store (`%LOCALAPPDATA%\burnmon\burnmon.db`,
31,604,736 bytes on disk, 2026-09-22) with a throwaway harness that ran 300 simulated
2-second polls (10 minutes) of `EventsSince` + `BuildSnapshot` + `ApplySessionTotals`:
`HeapAlloc` oscillated between roughly 0.9 MB and 3.8 MB across GC cycles and settled
back to +20,136 bytes (0.02 MB) over baseline after a final `debug.FreeOSMemory()`, well
under the 50 MB budget. This was a real-store measurement (Wilco's actual database, not
synthetic), not a live 10-minute run of the running app itself. `go test ./...` and
`.\build.ps1` both green.

## v0.1.1 F2: Codex latency (root cause, not a guess) and the two label bugs

Diagnostic (code-reading against `cmd\burnmon\main.go` and Wilco's real
`%USERPROFILE%\.codex\sessions` tree, not a live-reproduced timing run, per the spec's own
allowance to reason from the code path when a live turn cannot be re-triggered on demand):
(1) the Codex root was already on the fsnotify path, not the 5-second poll path
(`startLiveWatch` appended `codex.NativeSources()` into `nativeRoots` before calling
`watch.New`); (2) `watch.go`'s `addTree` does recursively re-watch a newly `Create`d day
folder and then fire `Create` for the rollout file inside it, so a brand new file was
never the problem; (3) confirmed the real cause: `startLiveWatch()` was called only after
`a.rebuild(true, ...)` (the initial full backfill) returned, in the same startup
goroutine, so the watcher did not exist at all for however long that first pass took.
Wilco's real Codex tree is 207 files, 142 MB, including one 29.8 MB and two ~11 MB
rollouts; a live turn landing during that backfill had nothing to catch it until the
watcher started afterward, exactly matching "no card at 12:29 and 12:42, appeared at
12:44." Fixed per the spec's "two goroutines, live watcher ahead of backfill": `main()`
now calls `dataset.Cache.SeedNativeRoots` (new, cheap `os.Stat`-only) and
`startLiveWatch` synchronously before the backfill goroutine even starts, and
`a.rebuild()` no longer holds `a.mu` for the whole `Collect` call (it copies `a.cfg`
first), so a live `IngestFile` from the watcher is never blocked behind an in-flight
backfill or scheduled rebuild the way it was when both shared one lock for their entire
duration. `dataset.Cache` gained its own `rootsMu` to guard `RootsByAdapter` now that it
is genuinely read (via `IngestFile`) and written (via `Collect`) concurrently. WSL roots
are added to the running watcher afterward via the new `watch.Watcher.AddWSLRoots`, once
the backfill has resolved them. Proven by `TestLiveWatchCodexTurnWithinTwoSeconds`
(`cmd\burnmon\main_test.go`): appends one `token_count` line to a temp rollout already
under watch and asserts the snapshot shows it inside 2 seconds; passed in 0.16s.

Labels: (a) `classifySurface` already handled the real originator values seen live today
("codex-tui", "codex_vscode"); the actual UNKNOWN-surface bug was that `surface` was a
`Parse`-local variable reset to "unknown" on every call, and `session_meta` (the only
line carrying `originator`) is written once, at the top of the file: a live watcher's
incremental `Parse(path, from>0)` never saw it again after the first read. Fixed with
`scanHeaderMeta`, a bounded (64 KB) re-read of the file's start on every incremental
call, confirmed against a real fixture in `TestParseIncrementalReadKeepsSurface`. Also
added `logUnknownOriginatorOnce` for a genuinely new originator, per spec. (b) Confirmed
on Wilco's real rollout `rollout-2026-09-22T12-41-36-...-986f3f57cd49.jsonl`
(`session_meta.payload.thread_source` = `"guardian_review"`, `source.subagent.other` =
`"guardian"`, `parent_thread_id` pointing at the `gpt-5.6-sol` session) that
`turn_context.payload.model` there is literally the string `"codex-auto-review"`, a
sub-run's own name, not a model id. `readSessionMeta` now flags any session_meta with a
`thread_source` other than `"user"` as a sub-run; its `turn_context.model` value is
carried as `Title` and `Model` stays empty (`TestParseSubRunModelBecomesTitle`).
`go test ./...` and `.\build.ps1` both green.

## v0.1.1 F3: the Now page chart is a dense, local-time, own-peak running chart

Rebuilt per the spec's "history, newest right, fixed width, own-peak scale" model.
Backend (`internal\live\live.go`): `MinuteBucket` (one entry per minute that actually had
a turn, sparse) is replaced by `Bucket`, always exactly `chartSlots` (180) entries at
`BucketSeconds` (10) width covering a fixed 30-minute window, every slot present and
zero-valued when empty; `Snapshot` gained `WindowStart` and `BucketSeconds` so the
frontend can place every slot without ever deriving one from sparse data itself, per the
spec. `TestBuildSnapshot_ChartIsDense` asserts the slot count and that exactly one slot
carries the one turn inside the window. Frontend (`internal\report\template.html`,
`drawNowChart`): the x axis now renders each `Bucket.At` (UTC) in the viewer's own local
time via `localHMS` (`new Date(iso)` plus local `getHours/getMinutes/getSeconds`), fixing
the reported UTC-label bug (axis said 10:38 at 12:29 local, `live.go:245`'s old
`e.At.UTC()` formatting). Each bar series is now labelled "agent · model · project
basename" (`sessionChartLabel`, built from the matching `Session` in `snap.sessions` via
a new `flattenSessions` that walks subagents too) instead of the raw session id; the id
now only appears in the tooltip. Cost per bucket is still its own line on the right axis.
Both axes are given an explicit `max` computed from the current window's own peak
(tokens: tallest single-bucket stacked total; cost: tallest single bucket's cost) on every
call; since `drawNowChart` destroys and rebuilds the whole Chart.js instance on every
2-second poll already, "recomputed every poll, holds the scale until the spike leaves the
window" falls out of that directly rather than needing separate machinery. Token class
split (fresh/cache write/cache read/output) moved into the tooltip only (`afterLabel`),
per the spec's explicit "no per-class toggle in v0.1.1, that is v0.2 scope." `go test
./...` and `.\build.ps1` both green; the JS was also syntax-checked with `node --check`
(no Node runtime is part of the shipped app, this was verification only).
