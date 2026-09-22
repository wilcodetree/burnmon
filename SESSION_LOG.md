# claudecost, session log

One paragraph per work session, newest on top.

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
