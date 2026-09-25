# BurnMon Dev, design (phase 0)

Date: 2026-09-24
Status: phases 0-2 and 2b built against this note (this session, 2026-09-24); this session did
not itself confirm Wilco's phase 0 sign-off happened out of band before phase 1 started. Section
9 (added after phase 2 closed) and the small corrections against what phases 1-2 actually built
are this session's own changes, flagged inline rather than folded in silently (the corrections
came from this session's own pre-commit Opus review of the phase 0-2 diff).
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
| Headline total, tok/min, activity heatmap, cache-hit breakdown (phase 2b) | see section 9 |
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
- `header`: black background, 2px solid `var(--amber)` bottom border, `min-height:44px` (wraps
  to two lines at the secondary viewport if the header's chips overflow one, needed once phase
  2b's headline block joined the header).
- Panel headers (`.panelhead`, this file originally called it `.phead`): uppercase,
  `var(--amber)`, **11px** (not 12px), `letter-spacing:.08em` (not `.1em`), `font-weight:700`,
  background `#10151d`, bottom border `var(--bd)`.
- Panel body (`.panel`): background `var(--panel)`, border `var(--bd)`.
- Heatmap tiles: same `.tile` recipe (padding, `min-height`, hover outline `var(--amber)`),
  intensity by opacity on the same green/red pair rather than a new palette. (The burn zone's
  own activity heatmap, section 9, reuses this same opacity-intensity idea, not the `.tile`
  markup itself.)
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
  (CPU-weighted, a different grid from the burn zone's activity heatmap in section 9 below):
  phase 3.
- Export bundle's exact `summary.md` prompt wording and `data.json` field names: phase 4, resolved below.

## 10. Phase 4: advisor and export (added 2026-09-24)

- **Advisor** (`internal/advisor`): one table-driven file, `rules` is a
  `[]{ID, Eval}` slice, every threshold lives in one `Thresholds` struct
  (`DefaultThresholds`). Six rules: heavy-turn-pressure, harness-runaway,
  cache-hit-drop, context-window, hard-faults-memory, self-overhead.
  `Analyze(Input, *pricing.Config, Thresholds) []Finding` is pure; the
  binding (`bdevAdvisorNow`) builds `Input` from a 60-minute window
  (`advisorWindow`, `cmd\burnmon-dev\export_run.go`) and runs off the UI
  thread via the existing `bdevAsyncResolveJS` pattern, same reasoning as
  `bdevActivityHeatmap`'s own review finding (even a "cheap" per-poll store
  read must not run on the UI thread). `live.BuildTurns` (new, exported
  wrapper around the previously-unexported `buildTurns`) supplies turns
  over that window instead of the Now page's fixed 30-minute chart window.
- **Export** (`internal/devexport`): `Assemble` (events, sysmon samples,
  process-group samples, machine profile, window, findings) to a `Bundle`,
  then `BuildSummaryMD`/`BuildDataJSON`/`BuildDailyCSV` (pure, string/bytes
  out) and `Redact` (stable sha256-based hashes for project/owner/client,
  no per-run salt). `Write` (write.go) is the only function that touches
  disk, creating `DIR\burnmon-dev_<yyyy-mm-dd_hhmm>\`. Never reads
  `schema.Event.Title` (prompt-derived text) anywhere in the package, so
  "never export prompt or response text" holds by construction; guarded by
  `TestAssemble_NeverExportsPromptOrResponseText`.
- **Machine profile** (`internal/sysmon/machineprofile_windows.go`):
  gopsutil for CPU/RAM/OS, `wmic`/`powercfg` shell-outs (fixed arguments,
  no user input) for GPU name and power plan/source; a stub keeps the
  package building on non-Windows.
- **CLI**: `burnmon-dev.exe export --since --until --out DIR [--redact]`
  (`cmd\burnmon-dev\export_run.go`), `--since`/`--until` accept `YYYY-MM-DD`
  or RFC3339, default window is the 24 hours before `--until` (default
  now). Shares `buildExportBundle` with the header button's `bdevExport`
  binding, which always exports the last 24 hours, not redacted, to
  `dataDir\exports\`.
- **UI**: the "Today's Read" panel (previously a phase-4 placeholder) polls
  `bdevAdvisorNow` every 15s; the header gains an EXPORT button and a
  status line, both wired to `bdevExport`'s own async-resolve result.
- **uicheck**: `tools\uicheck` gained a parallel dev-eval-channel path for
  burnmon-dev.exe (`cmd\burnmon-dev\uicheck_devserver.go`, port 9334, env
  `BURNMON_DEV_UICHECK`, window title "BurnMon Dev"), driven by
  `scripts\uicheck-dev.ps1` (called by `scripts\uicheck.ps1` itself when a
  target check name starts with "d"). Four checks: `d0` (advisor panel
  renders), `d1` (export button writes real files), `d2`/`d3` (both
  viewports, screenshot plus a scrollHeight/innerHeight overflow check).
  Measured on this laptop: this exe's own cold start (window plus the
  startup backfill) can take upward of 80s, well past burnmon.exe's own
  20s eval-port wait budget, so `uicheck-dev.ps1`'s own wait is 120s.

## 9. Phase 2b: token-monitor items (added 2026-09-24, after phase 2 closed)

Plan section "Taken from token-monitor", items 1-4 (item 5, export, is phase 4). No new
store or SQL: every number below comes from a package the burn zone already calls
(`internal/history`, `internal/vendorstrip`, `live.Snapshot`), reused as-is.

- **Headline total with running numbers.** Header gains a headline block: today's token
  total (large), week and month beside it (small), from `bdevVendorStrip`'s existing
  `Total` row (`Today`/`Week`/`Month`), already polled every 60s. `animateNumber` (RAF,
  easeOutQuart, ~1000ms for the headline, ~400ms for vendor-strip row cells, cancels an
  in-flight tween before starting the next, `tabular-nums`, instant under
  `prefers-reduced-motion`) replaces the plain `textContent` writes. **Bar fills are not
  smoothed in 2b**: the burn chart's bars and the session cards' context bars are rebuilt
  wholesale via `innerHTML` on every poll (`renderBars`, `renderSessionCards`), so a CSS
  `transition` on their height/width has no previous value to animate from and would be
  dead weight; an earlier draft of this note claimed otherwise, caught by this session's
  own pre-commit Opus review. Smoothing those would need the same reuse-the-DOM-node
  approach `ensureVendorRow` already uses for the vendor strip, deferred rather than
  done here, since neither the brief's Done-when nor phase 2b's own scope requires it.
- **Burn-rate framing.** `hdrTokMin` next to the existing `$/min` chip: tokens summed
  across `bdevBurnNow`'s already-polled chart buckets falling in the last 5 minutes,
  divided by 5, formatted with the same `fmtTokens` the rest of the page uses ("about
  1.2M tok/min"). No new binding: `bdevBurnNow`'s chart already carries this.
- **Activity heatmap, "N active days".** New `bdevActivityHeatmap` binding: `EventsSince`
  (182 days back) into `history.Build(events, cfg, Filter{Period:"day"})`, the same
  function the History tab's `bmHistory` runs, returning per-day `Tokens` and
  `CostUSD`. The page lays that out as a 26-ish-column x 7-row grid (Monday-start weeks,
  matching this codebase's own ISO-week convention elsewhere), one cell per day,
  intensity by tokens (opacity scale against the window's own max, not a fixed
  threshold), month labels under the grid where the month changes, "N active days"
  (tokens > 0) in the panel header, hover title `yyyy-mm-dd: tokens, cost`. Placed in the
  burn zone, between the vendor strip and the turn ticker; the ticker (`flex:1`, already
  scrolling) absorbs the added height so neither viewport gains a scrollbar on the page
  itself. Polled once a minute, same cadence as the vendor strip: a 182-day scan is not
  something to run on `bdevBurnNow`'s 2s cadence (F1's own "windowed, not whole-table"
  reasoning, `internal/store/store.go`'s `EventsSince` doc comment).
- **Cache-hit breakdown.** Clicking a vendor-strip row calls a new `bdevCacheBreakdown
  (vendor string)` binding: today's events for that vendor through the same
  `history.Build`, returning its `Totals` (`Fresh`, `CacheW`, `CacheR`, `Out`) directly,
  no new struct. The row expands a detail line: input miss (`Fresh+CacheW`) vs. hit
  (`CacheR`), output, and hit rate. Scoped to harness rows only, not harness-or-model:
  the vendor strip has no model dimension today, and phase 2b does not add one (that
  would be a new panel outside this section's brief items, not a reuse of an existing
  one): a ruling, not a silent narrowing, recorded here per house process.

## 11. UI review patch (2026-09-25), sections 1-11

Plan: `02_roadmap\2026-09-25_ws2_ui_review_patch.md`, which overrides this note and the
phase 0-4 plan wherever they differ. Not a phase re-run: a review pass over the shipped
phase 0-4 UI. Written after the code, not before, per this session's own read order; the
corrections below are this session's own findings, not a separate design step.

- **Header (section 1)** is now small and quiet: plain `PRESSURE nn` text under the
  wordmark (no chip box), a bare headline total (no "tok today"/week/month sub-line, moved
  to the vendor strip's own totals row), inline dim stats, `EXPORT` as a text button. The
  search input (`/cmd`) is gone; nothing in this codebase ever wired it to anything, so
  nothing else needed to change.
- **Burn chart (section 2)** stacks by session now, not by harness: `AGENT_COLOR` still
  picks the base color per harness, but each session inside that harness gets its own
  shade (`shadeColor`, an HSL-free RGB lighten/darken by a fixed offset table) so two
  concurrent sessions of the same harness are visually distinct. The CPU line overlay
  (`renderCPULine`, the `#cpuLine` SVG) is deleted outright, per Wilco's own "bars only"
  ruling; the turn-marker SVG overlay is unchanged. An `HH:mm` axis row draws under the
  bars every 5 minutes, from the chart's own bucket timestamps (always 30 slots, section
  2's own "with no sessions, still draw the empty axis" is automatic since `buildChart`
  always returns a full window regardless of events).
- **Section 2.2's "not wired yet" tabbed secondary viewport is superseded**, not fixed: the
  UI review patch's own section 10 replaces the phase 1-2 two-viewport plan (1152x2048
  primary, 1024x1152 secondary, one CSS layout shared by both) with three width
  breakpoints instead (below 900px one compact column, 900-1599px one column, 1600px+ two
  columns, burn left/system right, each full height rather than a 45/55 vertical split).
  The system zone no longer scrolls at 1024x1152 in this new layout (measured this
  session); no tabs were ever built, and none are needed now.
- **Vendor strip (section 3)**: a bold `Total` row, pinned to the top via `insertBefore` on
  every render (created once, like the per-vendor rows, so `animateNumber`'s tween state
  survives). Header cells and numeric cells right-align by default now (`th{text-align:
  right}`, `th:first-child{text-align:left}`), a page-wide change also used by the process
  -groups table (section 7).
- **Activity heatmap and the harness x minute heatmap (section 4)** now share one flex row
  (`.splitrow`, 40/60), stacking below 900px. Both heatmaps' own rendering is unchanged;
  only their container moved.
- **Turn ticker (section 5)**: capped at 50 (was 30), one amber-outlined `.tag` box
  regardless of finding kind (was four separate finding colors; the chart's own turn
  markers keep their per-kind colors, only the ticker's badge changed), tokens in white,
  row hover, and a click-through popup (`#turnPopupOverlay`) - the burnmon-dev equivalent
  of burnmon.exe's own turn drawer (`internal/report/template.html`'s `renderTurnDrawer`,
  read for reference, restyled rather than reused verbatim). New `live.TurnDetail.Cost`
  field (populated via the package's own existing `turnCost` helper) and a `bdevTurnDetail`
  binding, mirroring `cmd\burnmon\main.go`'s `bmTurn` async-resolve pattern with a
  session-id:turn key so a fast second click cannot have its response overwritten by a
  slower first one (`currentTurnKey` guard, page.html).
- **System panel (section 6)** is a full redesign: perfadvisor's own layout (`cpu, N
  threads` total bar plus a 5-column per-core grid with `base X.X GHz`; a canvas history
  chart, cpu/ram/gpu sharing a 0-100% scale, disk/net each scaled to their own peak; three
  bottom boxes, memory/disks/network), replacing the old CPU heat grid and four flat tiles.
  `internal/sysmon.Sample` gained `SwapUsedMB`/`SwapTotalMB`/`Disks`/`Wifi` and a
  `BaseClockGHz()` helper, ported from perfadvisor's own collectors (source commit
  perfadvisor main `2ed8046`, same commit section 7 above already cites). The per-drive
  scan (`disk.Partitions`/`disk.Usage`) and wifi (`netsh`) both run on a slower cadence
  (every 5th 2s tick, never tick 0) rather than every sample: found by review that running
  them every tick both delayed the app's own "first system sample" startup mark and risked
  blocking a whole tick against an unreachable mapped network drive.
- **Process groups (section 7)**: right-aligned headers (the page-wide `th` rule above),
  and the trend sparkline's own SVG now uses `viewBox="0 0 100 12"` with
  `preserveAspectRatio="none"` so it stretches to fill the trend column's own width
  (`width:100%` via a CSS rule targeting that column) instead of a fixed 42px.
- **"Today's Read" (section 8) is deleted**: the panel, its polling, and the
  `bdevAdvisorNow` binding are gone. `internal/advisor` itself is untouched; only
  `cmd\burnmon-dev\export_run.go`'s `buildExportBundle` calls it now (it always did its own
  independent, whole-window advisor pass, never through the deleted binding).
- **Microsoft To Do panel (section 9)**: `internal/todo`, ported from
  `C:\ZND\projects\perfadvisor\internal\todo\todo.go` (same source commit as the sysmon
  collectors above), with its own token cache under `%LOCALAPPDATA%\burnmon\` instead of
  perfadvisor's own folder, and `Login` split into `StartLogin` (one fast HTTP call) plus
  `FinishLogin` (the slow interactive poll, run in a goroutine) so a WebView2 binding never
  blocks on OAuth approval time. Off by default (`burnmon-dev.json`'s
  `microsoft_todo_enabled`); the panel sits below `main` as its own flex-basis-`auto`,
  height-capped, internally-scrolling block, full width in both the one- and two-column
  layouts. Never touches `internal/devexport` or any log line: task titles are personal
  data the "never in exports, logs or screenshots" rule covers by this package simply never
  being imported from that path.
- **Window, fullscreen, responsive (section 10)**: first launch is 1280x860 centered
  (overrides this note's own phase-1 default of 1152x2048 for the *window*, not the
  viewport breakpoints above, which are unchanged); size/position/maximized/monitor persist
  across launches (`burnmon-dev-window.json`, polled every 3s, since go-webview2 exposes no
  move/resize/close hook to save on). F11/Esc fullscreen is hand-rolled Win32
  (`cmd\burnmon-dev\windowstate.go`): WebView2/Chromium reserves F11 as a browser
  accelerator key the page's own JS never sees, so a global hotkey plus a thread-local
  `WH_GETMESSAGE` hook reacts to it directly, gated on this window holding real foreground
  focus so it never hijacks F11 from other running apps.
- **Startup (section 11)**: measured before changing anything (this session's own baseline,
  a cold run against the real store): store open 24ms, first system sample 3.65s, watcher
  and pollers started 17.4s (almost entirely `scan.DefaultSourcesWithOptions`/
  `codex.NativeSources` filesystem scanning, run synchronously before the window is
  created), window created 18.67s, first full render 19.08s, backfill finished 31.85s.
  Startup ordering changes (loading screen, background backfill) are section 11's own
  steps 2-5, applied after every other section, per the plan's own stated order; see
  `SESSION_LOG.md`'s matching entry for the after numbers.
