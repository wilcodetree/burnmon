# BurnMon

BurnMon is a vendor-agnostic token and cost monitor for your own AI coding sessions: a
portable Windows app (`burnmon.exe`) and a single-file CLI (`burnmon-cli.exe`), reading
the session transcripts your tools already write on this machine. No API key, no admin
rights, no network calls, no server, no telemetry: everything happens by reading local
files and everything stays on this device.

BurnMon is the successor of [claudecost](https://github.com/wilcodetree/claudecost),
renamed and extended from v0.2 onward. Repo:
[wilcodetree/burnmon](https://github.com/wilcodetree/burnmon). Plan:
`02_roadmap\2026-09-22_burnmon_plan.md`; the spec building the current release:
`02_roadmap\2026-09-23_v0.3_spec.md`.

## What it reads

- **Claude Code** and **Cowork** (Claude's desktop agent mode): `claude-code-sessions`
  transcripts, on Windows, macOS, Linux and inside WSL.
- **Codex** CLI and Desktop: rollout transcripts, native and WSL roots.
- **Hermes**: its local SQLite `messages` database, polled every 5 seconds.
- **GitHub Copilot CLI**: its own SQLite session store, polled every 5 seconds; a session's
  numbers update live, the same as every other adapter.
- **GitHub Copilot Chat in VS Code**: the JSON file its own OpenTelemetry export writes,
  polled every 5 seconds. Off by default: VS Code sends no telemetry anywhere until you
  turn these two settings on yourself, in `settings.json`:

      "github.copilot.chat.otel.enabled": true,
      "github.copilot.chat.otel.outfile": "C:\\path\\to\\copilot-otel.jsonl"

  Then point `burnmon.json`'s `copilot_vscode_otel_file` at that same path, and reload the
  VS Code window once (a setting change alone does not start the exporter; the extension
  host needs a fresh start). No workspace/folder attribute has been seen on any span
  emitted by the extension so far, so every event's client shows as "unassigned" until
  GitHub adds one; token counts and model are read regardless.

Coverage is these six. `claude.ai` in the browser keeps no local transcript BurnMon can
read, so it does not appear here.

## The five tabs

The app and every saved CLI report share one page, five tabs, deep-linkable
(`#now`, `#history`, `#sessions`, `#tools`, `#about`):

- **Now** (the start page). Every running session, live: polled every 2 seconds, new
  sessions picked up within about a second of their first write. A 30-minute smoothed
  burn chart (per-session lines, the cost line off by default), the vendor strip (one
  row per vendor seen in the store: today, this week, this month, tokens, refreshed once
  a minute), the turn ticker, and the forecast chart. See "The Now page" below.
- **History**. One page, filtered: period (day, week, month), range (last 7, 30, 90 days,
  custom), vendor, and owner once owner rules are configured. Totals, a "Burn per period"
  chart stacked one segment per harness (Claude Code, Codex, Copilot CLI and so on, fixed
  colours shared with the Now page), and a table for whatever is selected; the filter
  state lives in the URL so a view can be reopened. A per-client table appears once client
  rules are configured.
- **Sessions**. Every session, sortable, searchable, with a Harness column and filter (the
  real harness a session ran under, e.g. Claude Code, Codex, Copilot CLI, not the Surface
  it happens to share with another vendor), a Findings column (count by kind) and an
  expandable row listing each finding; click one to open the same turn detail drawer the
  Now page uses. The owner column and filter appear once owner rules are configured. A
  session priced by a vendor with no book entry for its exact model shows "no price"
  rather than a guessed figure.
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
"forecast unlocks after the first scored week". Tokens only.

## Cost

The Now page shows tokens, context fill and cache hit rate: what a developer watches
while working. History, Sessions and export also show cost, on the vendor's headline
basis, named next to the figure: plan credits for GitHub Copilot, your configured
subscription share when `burnmon.json` carries one, else the API list price labelled
"upper bound".

Cost comes from dated price books built into the binary (`internal/pricing/books`, each
with its own source URL and check date), overridable per model from `burnmon.json`.
`burnmon-cli price-check` prints every book, its date and its source, so you always know
how current a number is. A vendor with no book at all (Hermes today), or no entry for the
exact model a session used, shows "no price"/"tokens only" rather than a guessed figure.

## Owner and client rules, active time

`burnmon.json` can carry an ordered `owners` table of rules, e.g.
`{"match": "C:\\dev\\Work\\*", "owner": "Valona", "client": "Talon", "remote": "github.com/org/repo"}`,
applied to a session's project path at ingest. Owner: the first rule whose `match` path
prefixes the project wins, an unmatched path gets `"personal"`. Client (optional, v0.3):
a rule whose `remote` matches the project's git origin (read straight from `.git\config`,
no git binary) wins first, then a rule whose `match` path prefixes the project, else
`"unassigned"`. Empty `owners` (the default) means no owner or client column anywhere. A
v0.2 `burnmon.json` with `owners` only, no `client`/`remote`, keeps working unchanged.
After changing the rules, re-apply them to sessions already in the store with
`burnmon-cli reown`; nothing else re-runs ingest for you.

Each session also gets an **active time**: the sum of gaps between consecutive turns, any
gap over `active_idle_minutes` (config, default 10 minutes) counting zero. Labelled
everywhere it appears as "active time, from transcript timestamps, not billable": a
signal, not a time-tracking replacement. Once client rules exist, History's per-client
table shows tokens, headline cost, active time and session count side by side for every
client, comparable even with one selected in the filter above it.

## Export and merge

`burnmon-cli export --since <date> --until <date> --owner <name>... --label <name> --out
<file>` writes daily rows (owner, client, vendor, model, token classes, cost on every
basis available, active minutes); no paths, session ids, prompts or project names ever
leave in the file. When `owners` rules exist, `--owner` is required, so a row you did not
ask for never leaves by accident. `burnmon-cli merge <file>... --out <dir>` combines any
number of exports (free, no cap) into `merged.json` and a `report.html` that opens
offline with no network script, one column per export's `--label`, totals by client,
vendor and week.

## The CLI

    burnmon-cli                        build the dashboard and open it in the browser
    burnmon-cli live -json             one JSON snapshot of the Now page, for scripting
    burnmon-cli tools -since 30d       tool_calls totals: tool, calls, sessions, bytes
    burnmon-cli insight <session-id>   findings for one session, table or -json
    burnmon-cli reown                  re-apply owner and client rules to every event in the store
    burnmon-cli price-check            print every price book and when it was last checked
    burnmon-cli export -owner NAME     write a client-safe daily-rows export, see "Export and merge"
    burnmon-cli merge FILE... -out DIR combine exports into merged.json and an offline report.html

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

### macOS and Linux: untested

`burnmon` and `burnmon-cli` also build for darwin and linux (amd64 and arm64,
`CGO_ENABLED=0`, pure Go, no cgo), attached to every tagged release. Neither platform has
WebView2, so there is no app window there: `burnmon` collects once, writes
`dashboard.html`, opens it in the system default browser (`open` on macOS, `xdg-open` on
Linux), then keeps re-collecting and rewriting that same file on `-interval` in the
background; reload the browser tab to see newer numbers. No bound live functions exist
outside the WebView2 window, so the page falls back to its own "only available in the
BurnMon app window" text for anything live, the same as any saved report. **Untested**:
built and cross-compiled, never run on real macOS or Linux hardware.

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

## Default transcript roots, by OS

| Tool | Windows | macOS | Linux |
|---|---|---|---|
| Claude Code / Cowork | `%USERPROFILE%\.claude\projects` | `~/.claude/projects` | `~/.claude/projects` |
| Codex | `%USERPROFILE%\.codex\sessions` (or `$CODEX_HOME`) | `~/.codex/sessions` | `~/.codex/sessions` |
| Hermes | `%HERMES_HOME%\state.db`, else `%LOCALAPPDATA%\Hermes\state.db` | `$HERMES_HOME/state.db`, else `~/.hermes/state.db` | `$HERMES_HOME/state.db`, else `~/.hermes/state.db` |
| GitHub Copilot CLI | `%COPILOT_HOME%\session-store.db`, else `~/.copilot/session-store.db` | `~/.copilot/session-store.db` | `~/.copilot/session-store.db` |
| GitHub Copilot in VS Code | wherever `github.copilot.chat.otel.outfile` points, matched by `copilot_vscode_otel_file` | same (no default: the setting has none on any OS, you always choose the path) | same |

WSL distro detection (the registry-based scan for Claude Code and Codex transcripts
inside a WSL distribution) only exists on Windows; it is a no-op everywhere else.

## The Groundwork Kit

BurnMon is the token-cost slot in ZeroNonsense.dev's Groundwork Kit (the standard tool
set a Siteoffice client gets), replacing claudecost there from v0.3 onward. Setting a
Kit recipient up:

1. **Install**: copy `burnmon.exe`/`burnmon-cli.exe` and a `burnmon.json` next to them,
   no admin rights, no account. Portable: copy the folder to move it.
2. **Set client rules**: add `"owners"` rules with a `client` per project if the client
   bills more than one engagement through the same BurnMon install, so History's
   per-client table and the export below carry the right client name.
3. **Export**: on a cadence that suits the engagement (weekly is a reasonable default),
   `burnmon-cli export --owner <name> --label <machine-or-person> --out export.json`,
   then `burnmon-cli merge export1.json export2.json ... --out report\` once exports from
   more than one machine need combining into one number. Neither command needs the app
   open or a network call.

## Status

Shipped: `v0.3.0`, 2026-10-09 (`02_roadmap\2026-09-23_v0.3_spec.md`); `v0.3.1`, a cleanup
pass, 2026-09-24 (`02_roadmap\2026-09-24_ws1_burnmon_cleanup.md`). See `STATUS.md` for
what is true at the current commit and `SESSION_LOG.md` for the session-by-session record.
