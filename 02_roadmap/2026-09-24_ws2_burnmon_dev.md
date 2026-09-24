# WS2: BurnMon Dev, the AI-development monitor for a vertical screen

Owner: Wilco. Executor: fresh Claude Code session, Sonnet 5.
Repo: `C:\ZND\projects\burnmon`, branch `burnmon-dev`, worktree
`C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev`. WS1
(`2026-09-24_ws1_burnmon_cleanup.md`) runs on `main` at the same time and deletes the old
Monitor view. Rebase on `main` before the release step. Never use em dashes anywhere.

Read first: `AGENTS.md`, `C:\ZND\AGENTS.md`, `STATUS.md`, `SESSION_LOG.md` (top entry),
`C:\ZND\projects\perfadvisor\README.md` and its `docs\`,
`C:\ZND\_archive\marketadvisor\docs\2026-08-13_marketadvisor-design.md` and
`C:\ZND\_archive\marketadvisor\internal\server\web\index.html` (for the exact CSS tokens).

## Purpose

A developer runs AI agents on a laptop and cannot see what that does to the machine. BurnMon
Dev shows both on one screen: what the agents burn (tokens, cost, turns) on top, what the
laptop does (CPU, memory, disk, GPU, network, processes) below, on one time axis. An export
hands both to an AI model to find performance and token improvements. Audience: Wilco first,
other developers next, so no ZND-private paths or names in the UI.

## Decisions already taken (do not reopen)

- Name: **BurnMon Dev**. Exe `burnmon-dev.exe`, new `cmd\burnmon-dev\` in the burnmon repo,
  built by `.\build.ps1` next to `burnmon.exe` and `burnmon-cli.exe`.
- Reuse BurnMon's `internal\` packages (store, adapters, pricing, dataset) directly.
- System metrics: copy perfadvisor's collectors (`internal\collect\*_windows.go`, sampling
  logic from `internal\tui\sample.go`) into `internal\sysmon\`. Wilco owns both repos, so no
  licence issue; note the source commit (`perfadvisor` `main` at `2ed8046`) in a package
  comment. Do not change the perfadvisor repo.
- Look: marketadvisor. Black background, amber accents, green/red intensity, dense mono type,
  uppercase amber panel headers, keyboard-first. Dark only in this version.
- Layout: burn zone on top, system zone below. Finer grid than perfadvisor (it is a TUI; this
  is a WebView2 page on a 4 px grid), so more panels fit.
- Correlation: agent turns as markers on the system charts, and CPU/RAM/IO grouped per harness
  process tree.
- Export: a Markdown + JSON bundle.
- Stack: Go plus WebView2 (same library as `burnmon.exe`), `go:embed` page, bindings prefixed
  `bdev`.

## Viewports

1. Primary: screen 2, portrait 1440x2560 at 125 percent, so **1152x2048 CSS px**. Everything
   fits without scrolling.
2. Secondary: a half-width column on screen 1 (2560x1440). At 100 percent that is 1280x1440,
   at 125 percent 1024x1152. Burn zone stays whole; lower system panels may collapse into tabs.
3. Check both with `resize` in the uicheck harness. Nothing below 1000 px wide is required.

## Layout sketch (primary, top to bottom; refine in phase 0)

- Header line: app name, headline total tokens with running numbers (see "Taken from
  token-monitor"), tok/min, command/search line (`/`), pressure chip (perfadvisor's pressure
  score, the equivalent of marketadvisor's RISK-OFF chip), active sessions, burn per minute,
  CPU and RAM now, snapshot time, refresh countdown, layout slots L1 to L3.
- Burn zone, about 45 percent: live burn last 30 minutes as stacked bars per harness with
  optional CPU line overlay; active session cards (context bar, turns, cost); vendor strip as a
  dense table with sparklines (today, week, month); turn ticker, newest first.
- System zone, about 55 percent: CPU total plus a per-core heat grid; memory, disk, network,
  GPU as tiles with sparklines (like marketadvisor's FX panel); process groups per harness
  (claude, Claude Desktop and Cowork, node with claude-code, codex, Code, copilot, hermes,
  msedgewebview2, vmmem/WSL) with CPU, RAM, IO and a sparkline (like MOVERS); a heatmap of
  harness by minute, CPU-weighted; an advisor panel ("TODAY'S READ") with rule-based findings
  that combine both zones, for example "Codex turn 08:41, 1.4M tokens, CPU 96 percent for 40 s,
  node.exe".

## Taken from token-monitor (added 2026-09-24)

Source: github.com/Javis603/token-monitor (MIT, Electron). Reimplement the ideas below in our
own code; copy nothing verbatim, so no licence notice is needed. Read on 2026-09-24:
`src/electron/renderer/app.js` (`animateNumber`), `homeOverview.js`, `src/shared/exporter.js`.

1. **Headline total with running numbers.** A large total-tokens number at the top of the
   header (today, with week and month beside it) that counts up to each new value. Their
   method: one `requestAnimationFrame` tween per number, easeOutQuart, about 1000 ms, cancel
   the in-flight tween before starting a new one (else an old loop overwrites the new value),
   `font-variant-numeric: tabular-nums` so the width does not wobble, and an instant update
   when `prefers-reduced-motion` is set. Row values and bar fills in the vendor strip and
   process groups use the same tween (about 400 ms).
2. **Burn-rate framing.** Next to the headline, "about 1.2M tok/min" over the last 5 minutes.
3. **Activity heatmap, "N active days".** GitHub-style grid, one cell per day, rolling 26
   weeks, intensity by tokens, month labels below, count of active days in the panel header,
   hover shows date (yyyy-mm-dd), tokens and cost. Place it in the burn zone.
4. **Cache-hit breakdown.** Click a harness or model row to expand input (cache hit vs miss),
   output and hit rate.
5. **Export as pure functions.** The export builders take data and return strings, with no
   file I/O, so they are unit-testable and privacy is enforced by what the function accepts.
   Add a `daily.csv` (date, harness, model, token kinds, cost) next to `summary.md` and
   `data.json` for spreadsheet users.

Not taken: Electron, the tokscale parser (our adapters already cover our harnesses), multi-device
sync, tray and bubble modes. Plan limits (Claude 5-hour and weekly, Codex, Copilot) are **out of
WS2** by decision: they need credential reads and undocumented endpoints; parked as a separate
BurnMon item. Their supported-tools table (Cursor, OpenCode, Cline, Zed and more, with data
paths) is a reference for future BurnMon adapters, not WS2 scope.

## Data

- Token data: reuse the existing ingest. Phase 0 decides and writes down whether
  `burnmon-dev.exe` runs the adapters itself or reads the store that `burnmon.exe` fills, and
  what happens when both run (WAL, busy timeout, no double ingest).
- System samples: every 2 s live, persisted at 10 s resolution in a **separate** SQLite file
  (`burnmon-dev.db` next to the BurnMon store) so WS1 and WS2 never touch the same schema.
  Default retention 7 days, a config key changes it.
- Process-to-harness mapping by process name, command line and parent chain (gopsutil). Put the
  rules in one table-driven file with tests.

## Export (AI-ready bundle)

Button in the header plus `burnmon-dev.exe export --since --until --out DIR [--redact]`.
Writes `DIR\burnmon-dev_<yyyy-mm-dd_hhmm>\`:
- `summary.md`: machine profile (CPU, cores, RAM, GPU, OS, power plan, AC or battery), window,
  totals per harness and model, top 10 expensive turns each with the system state at that
  moment, pressure episodes, advisor findings, and a ready prompt at the top that asks a model
  for concrete performance and token-usage improvements.
- `data.json`: schema version, 10 s system timeline, turns (time, harness, session, model,
  tokens by kind, cost, cache hit), process-group timeline, machine profile.
- Never export prompt or response text. `--redact` replaces project, client and owner names
  with stable hashes. Default is not redacted.

## Phases (stop at each gate for Wilco)

0. Read. Write `04_assets\2026-09-24_burnmon_dev_design.md`: ASCII layout for both viewports,
   panel list with data source per panel, ingest decision, process-mapping rules, colour and
   type tokens taken from marketadvisor. **Gate: Wilco signs off before any code.**
1. Skeleton: `cmd\burnmon-dev`, `internal\sysmon` with tests, sample store, bindings, empty
   panels in the grid at both viewports.
2. Burn zone.
3. System zone and correlation (markers, process groups, heatmap).
4. Advisor panel and export.
5. Verify and release.

If context passes about 60 percent, stop at a phase boundary and write a handover in
`handovers\` with the `handover` skill.

## Verify and close

- `go vet ./...`, `go test ./... -count=1`, `.\build.ps1`, `node --check` on the page JS.
- Real-window checks through `scripts\uicheck.ps1` with new `d*` cases: both viewports, no
  scroll at 1152x2048, markers present, export writes both files and `data.json` validates.
- Measure and report `burnmon-dev.exe` own CPU and RAM over 10 minutes. It must not become the
  load it monitors: target under 2 percent CPU average and under 250 MB RAM; report the real
  number either way.
- A fresh read-only review agent (Opus) over the diff before each commit.
- Version: v0.4.0-alpha.1 on the branch (semver minor per `C:\ZND\AGENTS.md`).
- Do not push, tag or merge. Hand Wilco the exact PowerShell commands (with `cd`).
- Write one hub brief with the `hub-agent-update` skill in `04_assets\`.
