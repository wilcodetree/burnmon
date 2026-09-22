# claudecost, session log

One paragraph per work session, newest on top.

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
