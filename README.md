# BurnMon

BurnMon is a vendor-agnostic token and cost monitor for your own AI coding sessions: a
portable Windows app (`burnmon.exe`) and a single-file CLI (`burnmon-cli.exe`), reading
the session transcripts your tools already write on this machine. No API key, no admin
rights, no network calls, no server, no telemetry: everything happens by reading local
files and everything stays on this device.

BurnMon is the successor of [claudecost](https://github.com/wilcodetree/claudecost),
renamed and extended from v0.2 onward. Repo:
[wilcodetree/burnmon](https://github.com/wilcodetree/burnmon). Plan:
`02_roadmap\2026-09-22_burnmon_plan.md`; the v0.2 spec building this release:
`02_roadmap\2026-09-22_v0.2_spec.md`.

## What it reads

- **Claude Code** and **Cowork** (Claude's desktop agent mode): `claude-code-sessions`
  transcripts, on Windows, macOS, Linux and inside WSL.
- **Codex** CLI and Desktop: rollout transcripts, native and WSL roots.
- **Hermes**: its local SQLite `messages` database, polled every 5 seconds.
- **GitHub Copilot CLI**: session totals, read at shutdown, so a Copilot CLI session's
  numbers appear once it closes, not while it runs.

Coverage is these five. `claude.ai` in the browser, and GitHub Copilot in VS Code, keep no
local transcript BurnMon can read, so neither appears here (checked directly for Copilot
VS Code: it can emit OpenTelemetry to a local file, but that path is not wired into
BurnMon yet, see "Not yet there" below).

## The five tabs

The app and every saved CLI report share one page, five tabs, deep-linkable
(`#now`, `#history`, `#sessions`, `#tools`, `#about`):

- **Now** (the start page). Every running session, live: polled every 2 seconds, new
  sessions picked up within about a second of their first write. A 30-minute smoothed
  burn chart (per-session lines, the cost line off by default), the vendor strip (one
  row per vendor seen in the store: today, this week, this month, tokens, refreshed once
  a minute), the turn ticker, and the forecast chart. See "The Now page" below.
- **History**. One page, filtered: period (day, week, month), range (last 7, 30, 90 days,
  custom), vendor, and owner once owner rules are configured. Totals, a chart and a table
  for whatever is selected; the filter state lives in the URL so a view can be reopened.
- **Sessions**. Every session, sortable, searchable, with a Findings column (count by
  kind) and an expandable row listing each finding; click one to open the same turn
  detail drawer the Now page uses. The owner column and filter appear once owner rules
  are configured.
- **Tools**. Which connectors, plugins and skills your sessions actually call, as a chart
  and a table, over the whole window.
- **About**. How Claude's own seat and allowance model works, in plain language, plus
  where the numbers come from and what they cannot show.

## The Now page

Every running session shows a card with a context gauge. Under the chart, each session's
turn ticker line can carry a marker; clicking a marker or its ticker line opens a drawer
with the model, token classes, tool calls that turn, files read, the gap since the
previous turn, and, when a finding applies, its cause and confidence.

Findings come from `internal/insight`, computed on the fly from the same windowed events
the Now snapshot already loads (nothing about them is stored):

- **re-prefill**: a turn's cache write exceeds the configured threshold. The cause is
  inferred in order: compaction just happened, the model changed since the previous turn,
  the gap since the previous turn exceeded the cache TTL, first turn after a resume, or
  "unknown".
- **compaction**: context size drops by more than 30% between consecutive turns while the
  session continues.
- **context-runway**: a linear fit over the last 10 turns of context size, reported as
  turns remaining to 80% and 90% of the model's context window.
- **expensive-turn**: a turn above the 95th percentile of the session's own turn cost or
  tokens, with its dominant token class named.

The forecast chart shows the plan line (the last four weeks, weekday-aware) and the live
line (the current rate carried to end of day and end of month), with an error band once
at least one week has been scored. Until then it shows history only, with the gate text
"forecast unlocks after the first scored week". Tokens only; euros wait for the v0.3
price book.

## Owner rules

`burnmon.json` can carry an ordered `owners` table of path-prefix rules, e.g.
`{"match": "C:\\dev\\Work\\*", "owner": "Valona"}`, applied to a session's project path
at ingest: the first matching rule wins, an unmatched path gets `"personal"`. Empty (the
default) means one owner and no owner column anywhere. After changing the rules, re-apply
them to sessions already in the store with `burnmon-cli reown`; nothing else re-runs
ingest for you.

## The CLI

    burnmon-cli                        build the dashboard and open it in the browser
    burnmon-cli live -json             one JSON snapshot of the Now page, for scripting
    burnmon-cli tools -since 30d       tool_calls totals: tool, calls, sessions, bytes
    burnmon-cli insight <session-id>   findings for one session, table or -json
    burnmon-cli reown                  re-apply owner rules to every event in the store
    burnmon-cli price-check            print every price book and when it was last checked

`burnmon-cli` also keeps every claudecost-era flag: `-months`, `-seat`, `-no-open`,
`-json data.json`, `-source DIR`, `-no-cache`, `-out DIR`, `-config`, `-version`. See
`burnmon-cli -h` and each subcommand's own `-h` for the full list.

## The app

Double-click `burnmon.exe`. One window opens, no console, no tray icon, no browser tab.
It collects on startup, again every 30 minutes (`-interval`, floor 5 minutes) while the
window stays open, and whenever you press "Refresh now". Close the window and the app is
gone; nothing keeps running in the background. No open ports, no admin rights
(`asInvoker`). Needs the WebView2 runtime (ships with Edge on Windows 10/11); without it,
the app writes the report once and opens it in your default browser instead.

Parsed transcripts are stored in `%LOCALAPPDATA%\burnmon\burnmon.db`, a SQLite database
migrated forward automatically on every startup (versioned, additive migrations only:
nothing already stored is ever dropped). Click the gear icon to edit subscription
numbers, seat counts and seat prices from inside the window; saving re-reads everything,
since cached sessions carry costs computed with the old numbers.

## Configuration

Prices and the subscription calibration are compiled-in illustrative defaults (see
`internal\pricing\pricing.go`), not a real invoice. Drop a `burnmon.json` next to the exe
(app and CLI both check there first, then `%LOCALAPPDATA%\burnmon\burnmon.json` for a
fixed, read-only-share install) with your own numbers; any subset of fields overrides the
defaults. See `burnmon.example.json` for every key, including `owners` (empty by
default).

If no `burnmon.json` is found, both binaries still read a `claudecost.json` in the same
two locations: the pre-rename config name, kept working for anyone who has not renamed
their file yet. `burnmon.json` always wins when both are present. Rename to
`burnmon.json` when convenient; there is no reason to keep using the old name once you
have.

## WSL

Claude Code running inside a WSL distribution (Ubuntu under WSL, most commonly) is a
Linux process with a Linux `$HOME`, so its transcripts live inside the distro, not under
any Windows folder. BurnMon finds them automatically: it reads which WSL distributions
are installed from the registry, then looks for each one's `.claude/projects` folder,
including one moved via a `CLAUDE_CONFIG_DIR` set in a shell startup file.

A few things are worth knowing:

- Reading Linux files from Windows goes over WSL's own file-sharing layer and is slower
  than reading a native NTFS folder. WSL transcripts are re-read on their own, slower
  clock (`-wsl-interval`, default 4 hours) while native Windows transcripts keep the
  normal refresh. Refresh now and saving Settings always re-read everything, WSL
  included.
- Accessing a stopped WSL 2 distribution's files can start it in the background, which
  costs a few seconds and some memory. This only happens on the slower WSL cadence above.
- Set `"wsl_scan": "off"` in `burnmon.json` to turn WSL scanning off entirely; the first
  thing to try if a scan seems slow and you want to rule WSL out.
- If detection misses a distro, add the folder directly with `"extra_sources": ["..."]`
  in `burnmon.json`, or with a repeatable `-source` flag.

## What BurnMon never does

No server, no network call of any kind, no telemetry, no account. It reads only your own
transcript folders, never aggregates or compares across people, and refuses no one
because there is nothing to refuse: other people's usage is simply not on your disk.
Costs shown for Claude are allocated shares of a flat monthly invoice, not money owed by
anyone.

## Build

Requires Go (winget install GoLang.Go). Then, in PowerShell, from this folder:

    .\build.ps1

First run needs internet access: it fetches `go-winres` for the icons and `go mod tidy`
resolves the WebView2 binding. Produces `burnmon-cli.exe` and `burnmon.exe`, both portable
single files. Copy them anywhere; no install. If a `build.local.ps1` exists next to
`build.ps1` (gitignored, machine-local), it runs afterward.

## Not yet there (v0.3)

Per-vendor cost and credits, the dev/business cost-view switch, the full client map
(active time, export, merge across owners), macOS and Linux builds, and GitHub Copilot in
VS Code (its OTel export is confirmed working on Wilco's own laptop; wiring it in is
scoped for v0.3, see `02_roadmap\2026-09-22_v0.2_spec.md` A3). None of these show partial
or misleading numbers today: where a feature is not built, the page says so (the price
book note on History, the forecast gate on Now) rather than guessing.

## Status

Actively developed toward the `v0.2.0` tag, due 2026-11-14 (`02_roadmap\2026-09-22_v0.2_spec.md`).
See `STATUS.md` for what is true at the current commit.
