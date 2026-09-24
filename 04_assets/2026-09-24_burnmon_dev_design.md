# BurnMon Dev, design (phase 0)

Date: 2026-09-24
Status: phases 0-2 built against this note (this session, 2026-09-24); this session did not
itself confirm Wilco's phase 0 sign-off happened out of band before phase 1 started. The small
corrections below against what phases 1-2 actually built are this session's own changes,
flagged inline rather than folded in silently (found by this session's own pre-commit Opus
review of the phase 0-2 diff).
Plan: `02_roadmap\2026-09-24_ws2_burnmon_dev.md`
Read for this pass: `AGENTS.md`, `C:\ZND\AGENTS.md`, `STATUS.md`, `SESSION_LOG.md` (top entry),
`C:\ZND\projects\perfadvisor\README.md` and its `docs\`,
`C:\ZND\_archive\marketadvisor\docs\2026-08-13_marketadvisor-design.md`,
`C:\ZND\_archive\marketadvisor\internal\server\web\index.html`, and this repo's own
`internal\store\store.go`, `internal\adapter\adapter.go`, `internal\dataset\dataset.go`,
`cmd\burnmon\app.go`, and perfadvisor's `internal\collect\types.go` / `internal\tui\sample.go`.

## 1. Ingest decision

**Decision: `burnmon-dev.exe` runs its own ingest, into the same shared `burnmon.db`, exactly
the way `burnmon.exe` already does.** It is not a read-only viewer of another process's store.

Reasoning: the purpose statement is "a developer runs AI agents and cannot see what that does
to the machine" on one screen, on its own, screen 2, portrait. Making burn data depend on
`burnmon.exe` also being open would break that for anyone who runs `burnmon-dev.exe` alone,
and contradicts "reuse BurnMon's internal packages directly" (plan, decisions already taken).
So `burnmon-dev.exe`:

- Opens `store.DefaultPath()` (the existing `%LOCALAPPDATA%\burnmon\burnmon.db`), same as
  `burnmon.exe` and `burnmon-cli.exe` today.
- Starts the same live watcher (`internal\watch`, fsnotify on Claude/Codex roots) and the same
  5-second pollers (Hermes, Copilot CLI, Copilot in VS Code) that `cmd\burnmon\app.go` already
  runs, unchanged, imported from the same packages.
- Reads the store back through the same query packages the Now page already uses:
  `internal\live`, `internal\history`, `internal\vendorstrip`, `internal\insight`,
  `internal\pricing`, `internal\dataset`. No new burn-side query logic, only new panels
  binding to it.

**What happens when both `burnmon.exe` and `burnmon-dev.exe` run at once**, verified against
`internal\store\store.go` as it stands today (`Open`, `setPragmas`, `runMigrations`):

1. **WAL and busy timeout, unchanged.** `Open` already sets `PRAGMA journal_mode=WAL` and
   `PRAGMA busy_timeout=5000` on every connection, including a second process's. SQLite's WAL
   mode allows exactly one writer across processes at a time; a writer that lands mid-write
   waits up to 5 s on the lock, then retries, rather than failing outright. Each process still
   holds `db.SetMaxOpenConns(1)` for its own writer handle, so nothing new needs to change here.
2. **No double ingest, because upsert is idempotent, not additive.** `UpsertEvents` keys on
   vendor plus request id and keeps the largest output seen (the existing "largest output
   wins" rule, carried from claudecost). Two independent parses of the same trail file, one per
   process, converge on the same rows; running both is wasted duplicate parsing, never
   duplicated totals.
3. **Migrations do not race.** Both processes call `runMigrations` inside one transaction on
   open; whichever opens second sees the version already current and no-ops. No schema change
   ships with this workstream, so this only matters for the general case, not this release.
4. **Cost of running both is real but bounded.** Each process's fsnotify watcher fires
   independently on the same file writes, and each 5-second poller reads the same SQLite/OTel
   files independently. This is extra CPU and disk-read, not extra correctness risk, and
   directly bears on this workstream's own Done-when budget (under 2 percent CPU, under 250 MB
   RAM, measured over 10 minutes): that measurement must be taken with `burnmon.exe` also
   running, since that is the real ambient case, not an idealized standalone one.

System samples never touch `burnmon.db` at all: they live in their own file
(`burnmon-dev.db`, next to it), per the plan's explicit instruction, so none of the above
argument needs to extend to system metrics. Turn markers on the system charts are a read-time
join (by timestamp) between `burnmon.db`'s events and `burnmon-dev.db`'s samples, not a copy of
turn data into the system store.

## 2. Viewports and layout

Two viewports, both must fit with no scrolling (`resize` in `scripts\uicheck.ps1` covers both).
Grid unit is 4 px (finer than marketadvisor's 8 px, since more panels must fit in less width).

### 2.1 Primary: 1152 x 2048 CSS px (screen 2, portrait 1440x2560 at 125%)

```
0    +----------------------------------------------------------------+
     | BURNMON DEV   [/cmd______________]  <PRESSURE 42>               |  header, 44px
     | 3 sessions   $0.42/min   CPU 61%  RAM 74%   12:04:08   -02s      |  amber on black,
     |                                                  [L1][L2][L3]    |  2px amber rule
44   +----------------------------------------------------------------+
     | BURN, LAST 30 MIN                                     ~920px    |
     | .------------------------------------------------------------.  |
     | | stacked bars per harness, CPU% line overlay                |  |
     | '------------------------------------------------------------'  |
     | [session card] [session card] [session card]   <- context bar,  |
     |                                                    turns, cost  |
     | VENDOR STRIP  today | week | month  + sparkline, one row/vendor |
     | TURN TICKER, newest first, finding markers inline               |
964  +----------------------------------------------------------------+
     | SYSTEM                                               ~1084px    |
     | CPU total + gauge  |  per-core heat grid (N tiles)               |
     | MEM tile+spark | DISK tile+spark | NET tile+spark | GPU tile+sp. |
     | PROCESS GROUPS (per harness): CPU / RAM / IO / sparkline, table  |
     |   claude | codex | copilot-vsc | copilot-cli | hermes | wsl | .. |
     | HEATMAP: harness x minute, CPU-weighted                         |
     | TODAY'S READ (advisor): rule findings, both zones combined       |
2048 +----------------------------------------------------------------+
```

Burn zone is about 45% of the body (920 of 2004 px below the header), system zone about 55%
(1084 px), matching the plan's split.

### 2.2 Secondary: half-width column on screen 1 (2560x1440), 1024x1152 CSS px at 125%

Burn zone stays whole and full width; the lower system panels collapse into tabs (one visible
panel at a time) instead of stacking, since 1152 px is not enough height to show everything
at once without scrolling. **Not wired yet**: phases 1-2 built one CSS layout shared by both
viewports (the system zone still stacks, just shorter, rather than tabbing), so this viewport
currently scrolls the system zone instead. Tabs are open work, no phase assigned yet.

```
0   +----------------------------+
    | BURNMON DEV  <42> $0.42/m  |  header, chips wrap to 2 lines if needed
36  +----------------------------+
    | BURN, LAST 30 MIN          |
    | bars + CPU overlay          |
    | session cards (1 column)    |
    | vendor strip (compact rows) |
    | turn ticker                 |
~660+----------------------------+
    | SYSTEM  [CPU/MEM][PROC][READ]  <- tabs
    | (active tab's panel only)   |
1152+----------------------------+
```

At 1280x1440 (100%) the same layout gets more breathing room; nothing in the sketch changes,
only spacing. Nothing below 1000 px wide is required (plan, Viewports).

## 3. Panel list, data source per panel

| Panel | Source |
|---|---|
| App name / command line (`/`) | static chrome, client-side fuzzy match over session/vendor names already in the payload |
| Pressure chip | `internal\sysmon`, composite 0-100 score (section 8 below), same role as marketadvisor's RISK-OFF chip |
| Active sessions, burn/min | `internal\live` (same query the Now page's bound function already runs), reused as-is |
| CPU/RAM now | `internal\sysmon`, latest sample |
| Snapshot time, refresh countdown | client-side timer against the last successful poll (**not wired yet**: phase 1-2 shipped a plain clock, no countdown; deferred, no phase assigned) |
| Layout slots L1-L3 | `burnmon-dev.json`, same mechanism as marketadvisor's `state.json` slots (**not wired yet**, deferred, no phase assigned) |
| Burn bars (30 min, per harness) + CPU overlay | `live.Snapshot.Chart` (`bdevBurnNow`, the same windowed `EventsSince` plus `live.BuildSnapshot` the Now page's `bmLive` runs, not a separate `internal\history` query as originally written here) for burn, `internal\sysmon` for the CPU series, merged client-side by timestamp |
| Session cards | `live.Snapshot.Sessions` via `bdevBurnNow`, unchanged |
| Vendor strip | `internal\vendorstrip`, called directly, unchanged |
| Turn ticker | `live.Snapshot.Turns`, each already carrying its own `Finding` (`live` runs `internal\insight` internally to build it; `cmd\burnmon-dev` does not call `internal\insight` directly, correcting this row's original wording) |
| CPU total + per-core heat grid | `internal\sysmon` (copied perfadvisor `cpu.Percent`, per-core) |
| Memory / disk / network / GPU tiles + sparklines | `internal\sysmon` live sample plus `burnmon-dev.db` history for the sparkline |
| Process groups per harness | `internal\sysmon` process table (gopsutil) grouped by the mapping rules (section 4), summed CPU/RAM/IO, sparkline from `burnmon-dev.db` |
| Heatmap, harness x minute, CPU-weighted | `burnmon-dev.db`, 1-minute buckets per harness group, aggregated at query time |
| Advisor panel ("TODAY'S READ") | rule engine over `internal\insight` findings (burn side) joined by nearest timestamp to `internal\sysmon`/`burnmon-dev.db` pressure at that moment |

## 4. Process-to-harness mapping rules

One table-driven file, `internal\sysmon\harness.go`, tested with `internal\sysmon\harness_test.go`.
Match order, first hit wins: **command-line substring**, then **exact process name**, then
**walk the parent chain** (inherit the nearest ancestor's harness), else `"other"`.

| Harness | Match rule |
|---|---|
| Claude Code (CLI) | process name `claude`/`claude.exe`, or a `node`/`node.exe` whose cmdline contains `@anthropic-ai/claude-code` |
| Claude Desktop / Cowork | process name `Claude.exe` (capitalized Electron shell), or cmdline contains `Claude Desktop` or `anthropic-cowork` |
| Codex | process name `codex`/`codex.exe`, or `node`/`node.exe` with cmdline containing `@openai/codex` |
| Copilot CLI | process name `copilot`/`copilot.exe`/`gh-copilot`, or cmdline containing `github-copilot-cli` |
| Copilot (VS Code) | `Code.exe` (or `Code - Insiders.exe`) whose extension-host child has `copilot` in its cmdline; only grouped here when `copilot_vscode_otel_file` is configured, since that is the only signal that ties a VS Code process to Copilot at all |
| Hermes | process name or cmdline containing `hermes` |
| WSL | `vmmem` / `vmmemWSL` process name, its own bucket, not attributed to a harness |
| BurnMon Dev itself | `burnmon-dev.exe` and its `msedgewebview2.exe` child, its own bucket, excluded from the harness table, kept for the tool's own CPU/RAM self-measurement (Done-when) |
| node (unclassified) | any `node.exe` that matches none of the above, its own bucket rather than silently joining another harness |
| other | everything else, not shown in the per-harness table |

## 5. Colour and type tokens, from marketadvisor

Reused verbatim (`C:\ZND\_archive\marketadvisor\internal\server\web\index.html`):

```
--bg:#04060a; --panel:#0a0e14; --bd:#1c2532; --amber:#ffa028; --amber2:#ffc46b;
--txt:#cfd8e3; --dim:#66788c; --grn:#00d26a; --red:#ff4b4b; --blu:#4da6ff;
--cyn:#38d3e3; --mag:#d68cf0; --yel:#ffd75f;
```

- Font: `--mono: Consolas,"Cascadia Mono","IBM Plex Mono",monospace` (a custom property, so every
  rule that sets its own `font` shorthand still gets the full stack instead of resetting to a
  bare generic `monospace`, `13px/1.45` on `body`, dense mono type.
- `header`: black background, 2px solid `var(--amber)` bottom border, fixed `height:44px`
  through phase 2 (phase 2b's header content needs it to wrap instead; that CSS change lands
  with that phase).
- Panel headers (`.panelhead`, this file originally called it `.phead`): uppercase,
  `var(--amber)`, **11px** (not 12px), `letter-spacing:.08em` (not `.1em`), `font-weight:700`,
  background `#10151d`, bottom border `var(--bd)`.
- Panel body (`.panel`): background `var(--panel)`, border `var(--bd)`.
- Heatmap tiles: same `.tile` recipe (padding, `min-height`, hover outline `var(--amber)`),
  intensity by opacity on the same green/red pair rather than a new palette.
- Grid: marketadvisor's 8px `gap`/`padding` narrows to 4px at the zone/panel level (`main`,
  `.zonebody`) in BurnMon Dev, since this is a WebView2 page on a fixed viewport with more panels
  to fit, not a resizable browser tab. Individual components keep whatever gap actually fits
  their content (session cards and system tiles at 6px, the per-core grid at 3px), so "4px
  everywhere" (this file's original wording) overstated it: 4px is the zone-level grid, not a
  rule applied to every gap on the page. Every other token (colour, border, font) is unchanged.
- Dark only in this version, per the plan.

## 6. `burnmon-dev.db` (system samples only)

Separate SQLite file next to `burnmon.db`, own schema, so WS1 (on `main`) and WS2 never touch
the same file or the same migration sequence.

- `sysmon_samples` (ts INTEGER PRIMARY KEY, cpu_pct REAL, cores JSON, mem_used_mb REAL,
  mem_total_mb REAL, disk_read_bps REAL, disk_write_bps REAL, net_down_bps REAL,
  net_up_bps REAL, gpu_pct REAL) at 10s resolution (live sampling itself runs every 2s in
  memory, only every 5th tick is persisted). **`pressure JSON` is not in the phase 1-2 schema**:
  the pressure chip's own composite formula is still open (section 8), so there is nothing to
  store yet; phase 3 adds the column alongside that formula, and since no released build has
  written this table without it, that is a plain `ALTER TABLE`, not a migration concern.
- `process_group_samples` (ts, harness, cpu_pct, mem_mb, io_bps) so the per-harness sparklines
  and the heatmap read from stored history, not a fresh gopsutil scan per chart render.
- Default retention 7 days. **`retention_days` in `burnmon-dev.json` does not override it yet**:
  `app.go`'s `startRetentionPrune` takes a hard-coded 7 today; wiring the config key is phase 3
  work (`main.go` passes the literal `7`, not something read from `cfg`). A daily prune deletes
  rows older than the cutoff either way.
- `sysmon.DefaultPath()` mirrors `store.DefaultPath()`'s own per-OS split, returning
  `<appDataDir>\burnmon-dev.db`.

## 7. Reused packages versus new code

Reused directly, unchanged: `internal\store`, `internal\adapter\*`, `internal\schema`,
`internal\dataset`, `internal\pricing`, `internal\live`, `internal\history`,
`internal\vendorstrip`, `internal\insight`, `internal\scan`, `internal\watch`.

New: `cmd\burnmon-dev` (the exe, bindings prefixed `bdev`), `internal\sysmon` (collectors
copied from perfadvisor's `internal\collect\*_windows.go` and `internal\tui\sample.go`, source
commit `perfadvisor` `main` `2ed8046`, noted in a package comment; its own sample store per
section 6, and `harness.go`'s mapping table).

## 8. Open items carried into later phases

- Exact composite formula for the pressure chip (0-100): phase 3, alongside the system zone
  and the advisor panel, since it needs the real `PressureInfo` ranges from a live machine to
  calibrate against, not a guess at phase 0.
- Exact heatmap colour scale (opacity steps) for the system zone's harness x minute heatmap
  (CPU-weighted): phase 3.
- Export bundle's exact `summary.md` prompt wording and `data.json` field names: phase 4.
- Phase 2b (token-monitor items: headline running total, tok/min, activity heatmap,
  cache-hit breakdown) is next; its own design section lands with that phase's own commit.
