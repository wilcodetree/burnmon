# BurnMon, status

What is true at this commit (2026-09-26): **v0.3.2** (WS3, shared ingest performance and
local time everywhere, `02_roadmap\2026-09-26_ws3_shared_ingest_performance.md`). Step 0
profiling on Wilco's real store (54,855 events, ~12,700 Cowork session folders) found the
reported "climbs to 900MB+/13k handles within a minute" symptom was almost entirely one
`fsnotify` watch per subdirectory (13,125 of 13,541 open handles): `internal\watch` now
holds a small Windows-only recursive `ReadDirectoryChangesW` watcher
(`watch_windows.go`, one handle per native root) behind the same `OnChange` interface,
with the old per-directory fsnotify design kept only for B1's untested darwin/linux
build (`watch_other.go`). Fixed, measured for real over 10 minutes: peak RAM 957MB→87MB,
peak handles 13,541→371, both comfortably under the 250MB/2%-avg-CPU targets, alone and
with `burnmon-dev.exe` running alongside (its own numbers are unimproved until WS2
rebases onto this commit, out of this session's scope). The other three hypotheses
(the 400MB memory cap, whole-table `AllEvents` loads, double ingest between the two
exes) were tested and NOT confirmed as drivers of the reported symptom, so none were
changed; full reasoning and numbers: `04_assets\2026-09-26_ws3_profile_before_after.md`.
Separately, every day/week/month boundary ("today", History's buckets, the vendor
strip, the forecast, the export's `--since`/`--until`) now uses local time (Wilco's
decision), not UTC; storage stays UTC. Two genuine bugs found and fixed along the way,
both a bare `.Format(...)` silently rendering whatever Location a time.Time already
carried rather than the calendar day Wilco actually meant: `internal\dataset\fromstore.go`'s
per-session daily bucket (fed the Now page's own Days/Weeks/Months aggregation, wrong by
a UTC offset for early-morning local turns) and `internal\forecast\forecast.go`'s
`dateKey` (wrong specifically for a `WeekStart` round-tripped through SQLite storage,
which comes back UTC-located even though the instant it names is a local midnight).
Both caught by tests, not by inspection alone (`TestBuildOneScoredWeekShowsBand` broke
until `dateKey` was fixed). DST verified against both 2026 Europe/Amsterdam transition
dates (29 March, 25 October) with dedicated tests in `internal/history`,
`internal/store` and `internal/dataset`, each confirmed to actually fail without its
corresponding fix before being trusted.

What was true at v0.3.1 (2026-09-24), a cleanup pass over v0.3.0
(`02_roadmap\2026-09-24_ws1_burnmon_cleanup.md`). Dev is now the only mode: the
Dev/Business header toggle and Monitor mode are both deleted, code and all (not just the
buttons). The Sessions tab now labels every harness by its real name (Claude Code, Codex,
Copilot CLI, Copilot in VS Code, Cowork, Hermes), not "Claude Code" for every vendor that
happens to share Surface "cli"; a session priced by a vendor with no book entry for its
exact model shows "no price" instead of a guessed Claude figure. History's "Burn per
period" chart stacks one segment per harness, fixed colours shared with the Now page,
x-axis labelled by the period's own start date (`yyyy-mm-dd`) on every period. v0.3.0, the
release from `02_roadmap\2026-09-23_v0.3_spec.md`, shipped before it: price books (C1),
per-vendor cost on every basis a book covers (C2), the owner-then-client map with active
time (K1, K2), the per-client view (K3), export and merge (K4, K5), GitHub Copilot in VS
Code as a real adapter (A4), macOS and Linux browser-mode builds (B1), and the Now page
fixes and loading states (U1, U2). Release candidate Done-when and VERIFY pass:
`SESSION_LOG.md`'s v0.3.0 entry, all seven Done-when items pass against the built exe and
the real local store, no fail carried into the tag. v0.2.3 (real-window check harness),
v0.2.2, v0.2.1 and v0.2.0 shipped before that. Specs: `02_roadmap\2026-09-22_v0.2_spec.md`
through `02_roadmap\2026-09-23_v0.3_spec.md`.

## What it is

BurnMon is the successor of claudecost, a vendor-agnostic token and cost monitor for
your own AI coding usage: a portable Windows app (`burnmon.exe`) plus a single-file CLI
(`burnmon-cli.exe`), reading local session transcripts directly, no account or API key.
It is the token-cost slot in ZeroNonsense.dev's Groundwork Kit, per README's own section.

## Adapters

- **Claude** (`internal\adapter\claude`): Claude Code CLI, Cowork/desktop agent-mode
  sessions.
- **Codex** (`internal\adapter\codex`): Codex CLI/Desktop rollout transcripts, native and
  WSL roots.
- **Hermes** (`internal\adapter\hermes`, A1): local SQLite `messages`, WAL, 5-second poll.
  Default macOS/Linux path fixed this session: `~/.hermes/state.db` (or `$HERMES_HOME`),
  confirmed live against Hermes's own docs, replacing V3-5's guessed
  Application Support/XDG paths, which were wrong.
- **Copilot CLI** (`internal\adapter\copilotcli`, A2): reads `session-store.db`'s own
  `assistant_usage_events`, one row per API call while the turn is in progress, so a session
  is live the same way every other adapter's is (corrected here, v0.3.1 cleanup: this line
  used to say "totals read at shutdown, not live", the plan's assumption A2 itself disproved
  and replaced, per the adapter's own doc comment; stale, not code, was wrong).
- **Copilot in VS Code** (`internal\adapter\copilotvsc`, A4, v0.3): tails the OTel file
  the two VS Code settings produce, 5-second poll, last write wins per request. No
  workspace/project attribute exists in the real span data seen so far, so its events
  show client "unassigned" until GitHub adds one.

## Pages

Five tabs, deep-linkable: **Now** (default start page), **History**, **Sessions**,
**Tools**, **About**. English only.

- **Now**: every running session, live, polled every 2 seconds; a smoothed burn chart
  (per-session lines, cost line an opt-in legend toggle); the vendor strip; the turn
  ticker with finding markers; the forecast chart (tokens). Loading states throughout
  (U2): every visual shows "Loading..." until its first real data, never the
  saved-report fallback text while the app's bindings are still being injected.
- **History**: filters (period, range, harness, owner, client once client rules exist);
  a per-client table once client rules exist (tokens, headline cost, active time,
  sessions); "Burn per period" stacks one segment per harness (v0.3.1).
- **Sessions**: sortable/searchable table, a Harness column and filter (v0.3.1: the real
  harness a session ran under, Claude Code/Codex/Copilot CLI/etc., no longer relabelled
  by Surface's Claude-only vocabulary), Findings column, owner and client columns once
  configured.
- **Tools**: `tool_calls` totals as a chart and table.
- **About**: the Claude pricing explainer, plus "what this shows and what it cannot",
  plus the macOS/Linux untested note.

## Store

Versioned additive migrations. Schema version 7 at this commit (migration 7, v0.3 K1,
adds `client` on `events`; the spec's own text calls it "migration 6", stale against
`migrations.go`'s real `Version: 6`, which is the v0.2.1 session_id index). A v0.1 store
opens under this code with no event lost (`TestMigrateRealV01Store`, run against the real
store this session, schema version 7 confirmed, all events retained). `tool_calls` table
fed by the Claude and Codex adapters. `burnmon-cli live` and the app's Now page share the
same windowed query and `SessionTotals` path.

## Cost (C1, C2, C3)

Dated JSON price books built into the binary (`internal\pricing\books`), each with a
`date` and `source`, overridable per model from `burnmon.json`. `CostForEvents` (one Go
function, shared by every page and export) returns every basis a book covers plus the
headline basis per vendor: credits for GitHub Copilot, subscription share once
configured, else API list labelled "upper bound"; a vendor with no book (Hermes) is
"tokens only" everywhere, never a guessed figure. `pricing.Config.EventCost` (v0.3.1) is
the same per-vendor book routing at single-event granularity, used by Sessions' own
per-call/per-model breakdown (`internal\dataset\fromstore.go`'s `buildSession`) and the
Now page's per-session/per-minute cost (`internal\live\live.go`'s `turnCost` and
`ApplySessionTotals`), both of which used to fall through to Claude's family-generic
price table for every non-OpenAI vendor: a Copilot or Hermes call priced as if it were
Claude Sonnet, fixed this session
(`TestSessionsFromEventsPricesGitHubEventsThroughCopilotBook`,
`TestSessionsFromEventsNoPriceForUnbookedVendor`). Side effect, not just the
Copilot/Hermes fix: an anthropic event on these two pages is now also priced from
`AnthropicBook`'s exact model id, the same book History/export already used, rather than
the family-generic table these two call sites used on their own; a real, intended figure
shift (e.g. the family table's Sonnet was $3/$15, the book's `claude-sonnet-5` is
$2/$10), and a Claude model id absent from the book now shows "no price" instead of a
guessed Sonnet figure. Every model id seen live on Wilco's laptop is in the book
(`internal\pricing\books\anthropic_api.json`); an older or unconfirmed id is the risk
case.

## Owner and client map, active time (K1, K2)

`burnmon.json`'s `owners` table now carries optional `client` and `remote` per rule,
alongside the v0.2 `match`/`owner`. Match order per session: a `remote` rule (matched
against the project's git origin, read straight from `.git\config`, no git binary, worktree-
and submodule-aware) first, then a `match` path rule, else owner `"personal"`/client
`"unassigned"`. A v0.2 `burnmon.json` (`owners` only) loads unchanged
(`TestLoadV02ConfigWithOwnersOnlyIsUnchanged`). `burnmon-cli reown` re-applies both to
every stored event. Active time: the sum of gaps between a session's consecutive turns,
any gap over `active_idle_minutes` (default 10) counting zero; per-client active time
sums its sessions'. Spot-checked against three real sessions (V3-2); a proper comparison
against an external time log remains open, see Known gaps.

## Export and merge (K4, K5)

`burnmon-cli export` writes daily rows (day, owner, client, vendor, model, five token
classes, cost on every basis, active minutes); no paths, session ids, prompts or project
names, enforced by a test that scans the output bytes for a path separator or a
session-id pattern. `--owner` is required whenever `owners` rules exist. Verified for
real this session: an export with `--owner ZND` against the real local store held zero
"Valona" occurrences, zero path separators, zero session-id patterns (35 rows). `burnmon-
cli merge` combines any number of exports (no cap) into `merged.json` and a `report.html`
that opens offline (zero `<script>` tags), one column per label, totals by client, vendor
and week; verified for real by merging two exports of the real store under different
labels.

## Platforms (B1)

`burnmon`/`burnmon-cli` cross-compile clean for darwin and linux, amd64 and arm64,
`CGO_ENABLED=0` (verified this session against the current tree). Off Windows, the app
collects once, writes `dashboard.html`, opens it in the OS default browser, then keeps
re-collecting on `-interval` in the background; no bound-JS live path exists there
(WebView2 is Windows-only), so a page there falls back to the same "only available in
the app window" text a saved report already shows. README, About and `STATUS.md` all say
untested: built and cross-compiled, never run on real macOS or Linux hardware.
`.github\workflows\release.yml` builds and attaches all four binaries to a pushed `v*`
tag; this only runs once the tag is actually pushed, see Known gaps.

## Insight (I1, I2, I3)

`internal\insight`, computed on the fly, no store writes. Four finding kinds:
`re-prefill`, `compaction`, `context-runway`, `expensive-turn`. Shown as markers on the
Now chart, lines in the turn ticker, the spike drawer, and per-session on the Sessions
tab.

## Forecast (F1)

Plan line (last four weeks, weekday-aware) and live line (current rate to end of day/
month), error band once a week is scored, gate text until then. Tokens only.

## Config

`burnmon.json` is the current name; `claudecost.json` in the same two locations is still
read as a fallback. `burnmon.example.json` documents every key including `owners` (with
one commented client/remote example), `copilot_plan` and `copilot_vscode_otel_file`. The
`mode` and `view` keys (v0.3's dev/business and monitor/full start state) are gone as of
v0.3.1: an old config carrying either still loads without error (unknown JSON keys are
ignored), it just no longer does anything.

## Known gaps at this commit

- A `forecast_scores` row written before this session's local-time change (v0.3.2)
  keeps a UTC-Monday `week_start` (post-fix rows are local-Monday, `store
  .scanForecastScore` converts on read either way, but `AddDate` arithmetic on a
  UTC-Monday value still steps in UTC calendar days). Affects at most the one ISO week
  that was unscored at the moment of upgrade; every week scored afterward is unaffected.
  Flagged rather than fixed (a fresh review's own finding, 2026-09-26): a future pass
  could snap an old row via `weekStart(sc.WeekStart)` in `EnsureScored` before scoring it,
  if this ever turns out to matter in practice.
- `store.go`'s event-time bounds (`at >= ?`, `at < ?`) compare RFC3339Nano strings
  lexically. RFC3339Nano drops trailing zero digits in the fractional seconds, and `.`
  sorts before `Z`, so a stored `...T22:00:00.5Z` sorts below a bound of
  `...T22:00:00Z`: an event in the first second of a range can be wrongly excluded at
  the lower bound, or wrongly included at the new upper bound `DailyTokenTotalsUntil`
  added (v0.3.2). Pre-existing pattern (the lower bound predates this session), not
  something v0.3.2 introduced, and `actualTokensForWeek`'s own in-Go `d.Date >= end`
  filter already catches the upper-bound case; flagged rather than fixed (a fresh
  review's own finding, 2026-09-26) since a real fix (binding in a fixed-width format,
  or comparing via SQLite's `julianday()`) touches every timestamp write in the store,
  well beyond WS3's own scope.
- `internal\report\template.html`'s Now chart cost-axis toggle: the right-hand cost axis
  used to stay hidden after the cost series is shown through the chart legend
  (`scripts\uicheck.ps1 w8`). Flagging a discrepancy rather than silently picking one: the
  v0.3.1 cleanup session ran `w8` live against the current build and it passed
  (`axis-when-shown=true`), with no code change to the cost-axis logic itself in this
  session. Left as an open question rather than marked fixed: could be genuinely resolved
  by an unrelated change, or timing-flaky; a future session should re-run `w8` a few times
  before either closing this gap or reopening the Chart.js debugging session it used to
  call for. Ran again this session (WS3, incidental, no cost-axis code touched):
  passed clean a second time; still left open rather than closed, same reasoning.
- `bmHistory` re-reads the whole store (`store.AllEvents()`, ~3s against Wilco's real
  ~55k-row store) on every History tab filter change, not just once: a real UI-latency
  cost, measured but not fixed this session (WS3 hypothesis 3, confirmed real but not a
  driver of the shared-ingest resource climb that session's Done-when targeted; see
  `04_assets\2026-09-26_ws3_profile_before_after.md`). A future pass could move this to a
  SQL aggregate or a daily summary table kept up to date on upsert.
- `burnmon.exe` and `burnmon-dev.exe` share the same `burnmon.db` (both call
  `store.DefaultPath()`/`store.Open()`), so both independently watch, parse and ingest
  the same source files when both run (WS3 hypothesis 4: measured safe, SQLite's WAL
  mode plus `busy_timeout=5000` plus idempotent upsert already cover it, no fix built).
  `burnmon-dev.exe`'s own high RAM/handle numbers when run alongside the now-fixed
  `burnmon.exe` are its pre-rebase build still carrying the old per-directory watcher,
  not a new problem; resolves automatically once WS2 rebases onto this commit.
- GitHub Copilot Business and Copilot Enterprise: their AI-credit allotments are now
  confirmed live (1,900/user/month and 3,900/user/month respectively, pooled at the
  billing entity, `docs.github.com/en/copilot/concepts/billing-and-usage/organizations-
  and-enterprises/billing`, checked 2026-09-24), but the plans page's "for businesses"
  tab still does not render in a plain page fetch, so no per-seat monthly price is
  confirmed for either, and neither is wired into `copilot_credits.json`'s `plans` block
  (their pooled, org-level billing also does not fit `CopilotCreditsLeft`'s
  single-machine model without further design work).
- A ChatGPT/Codex plan-credit table comparable to GitHub's AI Credits does not exist:
  confirmed live 2026-09-24 (`help.openai.com`'s own credits article) that ChatGPT
  personal plans use their own usage limits plus an optional pay-as-you-go credit
  top-up for overage, not a fixed monthly allotment; nothing to build a book from.
- Two backend figures survive the dev/business toggle's removal computed but unused: the
  vendor strip's `copilot_credits_left` (its own display line, `#vs_credits_left`, is now
  permanently hidden) and `live.Session.BusinessCost`/`business_cost` (its only reader,
  `sessionCardBusinessBody`, is deleted). Left in place this session (harmless, not a
  correctness bug, real but small ongoing compute cost every poll); a future pass should
  either remove both or find them a dev-mode use.
- Plan assumption A8 (active time usable): the spot-check from V3-2 stands, but
  `C:\ZND\10_holding\03_logs\time\` still does not exist on this laptop (checked again
  this session), so the comparison against an external time log remains unverified.
- macOS and Linux release artefacts: the workflow and the cross-compilation both verify
  clean, but no artefact has actually landed on a GitHub release yet, since that only
  happens once the `v0.3.0` tag is pushed; this session tags locally and stops before
  pushing, per house rules.
- The Now chart's per-vendor colour families define a lighter "desktop"/"VS Code" shade
  for Codex and Copilot, but Codex is always agent "codex" and Copilot in VS Code sets
  no distinct agent value either, so those two shades stay unreachable.
- `internal\report\template.html`'s About section still describes only Claude's own
  seat/allowance model, not the other vendors.
- History's per-client table (restored this session with the dev/business toggle
  removed, gated on client rules existing rather than on the removed mode) was verified
  by reading the JS logic and the unchanged, still-passing Go tests
  (`TestBuildClientFilterAndRows`), not by a fresh real-window click-through: doing that
  needs a scratch `owners`/`client` config dropped next to the exe and reverted after,
  same as V3-6's own Done-when check 2, and this session did not repeat that live pass.

## BurnMon Dev (branch `burnmon-dev`, not yet merged to `main`)

`v0.4.0-alpha.1`: a second, developer-facing window (`cmd\burnmon-dev`, `burnmon-dev.exe`)
showing token/cost burn and system load on one time axis, following
`02_roadmap\2026-09-24_ws2_burnmon_dev.md`'s phases 0 through 4, three UI patches
(`2026-09-25_ws2_ui_review_patch.md`, `_burn_chart_no_scroll_patch.md`,
`_ticker_headline_patch.md`), and this session's phase 5 (verify and release). Reuses
`burnmon.exe`'s own `internal\` packages and store; system samples go to a separate
`burnmon-dev.db`. See README's own "BurnMon Dev" section for what it does and how to run
it; `04_assets\2026-09-24_burnmon_dev_design.md` for the design.

Phase 5 fixes this session: the headline total now adds tokens ingested since
`vendor_strip`'s own 60s cache refresh on every 2s tick (`cmd\burnmon-dev\headline.go`),
instead of only moving once a minute; `tools\uicheck`'s window sizing now reads
`GetDpiForWindow`/`AdjustWindowRectExForDpi` so a requested viewport size means real CSS
pixels regardless of display scaling, capped to the screen with a log line when it does
not fit; the process-groups panel's CPU percent is now normalized against the whole
machine (divided by core count client-side) instead of gopsutil's own unnormalized
per-process convention (100% meant one full core), which is what every other "%" on this
page already means; sampling now runs on three independent cadences instead of one
(paint/cheap system sample on `refresh_ms`, the process walk fixed at 3s, persistence to
`burnmon-dev.db` fixed at 10s wall clock), so a faster paint refresh no longer multiplies
process-scan or disk-write cost.

Known, not fixed on this branch (WS3 scope, shared `internal\` ingest code, out of scope
per the design doc's own "do not fix shared ingest code on this branch"): both
`burnmon.exe` and `burnmon-dev.exe` climb toward roughly 900 MB and 13k handles in their
first minute through the shared live-watch/store path. Reproduced again this session's
own measurement.

Rebased onto `main`'s `v0.3.1` (`1291ef9`) this session; one conflict, in `SESSION_LOG.md`
(both branches prepend entries to the same file), resolved by keeping both entries in
newest-on-top order. Not pushed, tagged or merged; see the hub brief and the commands
handed to Wilco for the exact next steps.

## Next

Decision 2026-12-19: continue to a paid team line, keep free, or stop. Out of scope,
named in the v0.3 spec's section 3: a local OTLP receiver, a merge cap and paid team
line, the BurnRate time-log join, a native window off Windows, signed builds,
project-file tags, a team server. Plan: `02_roadmap\roadmap.md`.
