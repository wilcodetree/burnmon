# BurnMon, status

What is true at this commit (2026-09-23, 46B): **v0.2.0 shipped**, all sixteen spec items
built, the release-candidate Done-when and VERIFY pass (46A) clean with no fails. Spec:
`02_roadmap\2026-09-22_v0.2_spec.md`. Next: v0.3 (2026-12-12), per-vendor cost and
credits, dev/business switch, full client map, macOS and Linux builds.

## What it is

BurnMon is the successor of claudecost, a vendor-agnostic token and cost monitor for
Wilco's own AI coding usage: a portable Windows app (`burnmon.exe`) plus a single-file
CLI (`burnmon-cli.exe`), reading local session transcripts directly, no account or API
key.

## Adapters

- **Claude** (`internal\adapter\claude`): Claude Code CLI, Cowork/desktop agent-mode
  sessions.
- **Codex** (`internal\adapter\codex`): Codex CLI/Desktop rollout transcripts, native and
  WSL roots.
- **Hermes** (`internal\adapter\hermes`, A1): local SQLite `messages`, WAL, 5-second poll.
- **Copilot CLI** (`internal\adapter\copilotcli`, A2): session totals read at shutdown,
  not live.
- Not an adapter: GitHub Copilot in VS Code. A3's one-hour check (this week, corrected
  2026-09-23) confirmed it can emit OpenTelemetry to a local file on Wilco's own laptop;
  wiring that file in as a source is scoped for v0.3, not built yet.

## Pages

Five tabs, deep-linkable: **Now** (default start page), **History**, **Sessions**,
**Tools**, **About**. Overview, Months, Weeks and Days are gone from the tab bar (their
markup and render code for Overview alone still sits unreferenced in
`internal\report\template.html`, kept only until it is safe to delete outright); How it
works became the first section of About. English only: the language toggle and the `t()`
table's second language are gone.

- **Now**: every running Claude Code/Cowork/Codex/Hermes/Copilot CLI session, polled
  every 2 seconds; a smoothed 30-minute per-session burn chart; the vendor strip (one row
  per vendor, today/week/month tokens, refreshed once a minute); the turn ticker with
  finding markers and a spike-detail drawer; the forecast chart (plan line, live line,
  error band once a week is scored, else the visible gate text).
- **History**: one page, filters (period, range, vendor, owner once configured), URL-hash
  state, replacing the old Overview/Months/Weeks/Days pages entirely.
- **Sessions**: sortable/searchable table, a Findings column and expandable detail row
  per session (I3), owner column and filter once owner rules are configured.
- **Tools**: `tool_calls` totals (S2) as a chart and table.
- **About**: the Claude pricing explainer, unchanged since v0.1, plus "what this shows
  and what it cannot".

## Store

Versioned additive migrations (S1): schema version 5 at this commit
(`forecast_scores` added in 44B). A v0.1 store opens under this code with no event lost
(`TestMigrateRealV01Store`). `tool_calls` table (S2) fed by the Claude and Codex adapters.
`burnmon-cli live` and the app's Now page share the same windowed query and
`SessionTotals` path (S3).

## Owner split (P6)

`burnmon.json`'s `owners` table, empty by default (one owner, no owner column shown
anywhere). Applied at ingest to a session's project path; `burnmon-cli reown` re-applies
current rules to every event already stored. Ships in `burnmon.example.json` as an empty
array with the rule syntax documented alongside it.

## Insight (I1, I2, I3)

`internal\insight`, computed on the fly, no store writes. Four finding kinds shipped:
`re-prefill`, `compaction`, `context-runway`, `expensive-turn`. Shown as markers on the
Now chart, lines in the turn ticker, the spike drawer (model, token classes, tool calls,
files read, gap since previous turn, cause, confidence), and per-session on the Sessions
tab (cached client-side per page load, measured at 39ms warm on the longest real session
in Wilco's own store, 461 turns).

## Forecast (F1)

Built, gated. Chart shows history only with "forecast unlocks after the first scored
week" until one ISO week is scored (`InsertForecastPlan` writes the plan once per week on
first Monday call; `RecordForecastActual` fills in the actual once, after the week fully
elapses). Scoring started this session (44B); weeks 44-46 are the three scoring weeks the
spec calls for. Tokens only; euros wait for the v0.3 price book.

## Config

`burnmon.json` is the current name; `claudecost.json` in the same two locations
(exe-adjacent, then `%LOCALAPPDATA%\burnmon\`) is still read as a fallback so an
un-renamed portable folder keeps working, `burnmon.json` always winning when both exist.
`burnmon.example.json` replaces `claudecost.example.json` this pass, `owners` shown
empty.

## Known gaps at this commit

- GitHub Copilot in VS Code: confirmed reachable via OTel (A3), not wired in as a source
  yet (v0.3).
- Per-vendor cost, the dev/business cost-view switch, the full client map (active time,
  export, merge), macOS and Linux builds: all out of scope for v0.2, named as such in the
  spec's section 3.
- `internal\report\template.html`'s About section still describes only Claude's own
  seat/allowance model, not the other vendors.
- Scoring has produced zero scored weeks as of this commit; the forecast chart is
  expected to show the gate text until partway through week 44.

## Next

v0.3 (2026-12-12): per-vendor cost and credits, dev and business switch, full client map
(active time, export, merge), macOS and Linux builds. Plan: `02_roadmap\roadmap.md` item 4.
