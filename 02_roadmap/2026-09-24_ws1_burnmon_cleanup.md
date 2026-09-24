# WS1: BurnMon cleanup (v0.3.1)

Owner: Wilco. Executor: fresh Claude Code session, Sonnet 5. Branch: `main`.
Runs in parallel with WS2 (`2026-09-24_ws2_burnmon_dev.md`), which works in a worktree on
branch `burnmon-dev`. WS1 lands first; keep the diff tight so WS2 can rebase.

Read first: `AGENTS.md`, `C:\ZND\AGENTS.md`, `STATUS.md`, `SESSION_LOG.md` (top entry),
`02_roadmap\2026-09-23_v0.3_spec.md`. Never use em dashes anywhere. Line numbers below are from
a read on 2026-09-24 at v0.3.0; verify before editing.

## Decisions already taken (do not reopen)

- Dev/business toggle: pin **dev**. Remove the toggle and every business-only branch.
- Monitor view: **delete** the code, not only the button. It moves to BurnMon Dev (WS2), which
  reads the old code from tag `v0.3.0` if it needs it.
- Keep the Light/Dark theme button.

## Task 1: remove Dev and Monitor buttons, the mode and the Default view setting

Frontend, `internal\report\template.html`:
- Markup: `#btn_mode` (~251), `#btn_monitor` and the monitor exit button (~253-256),
  `#monitor_text_view` (~237-241), monitor CSS (~188-233).
- JS: mode and monitor handlers (~2965-2995), `setView`/`applyViewMode` (~2035-2050), monitor
  text/braille code (~1734-2030), monitor hooks in `renderNow` (~2328), `pollNow` (~2472), vendor
  strip (~2571). `UI.mode`, its localStorage key and `isBusiness()` (~583-603): replace every
  `isBusiness()` call with the dev branch, then delete the function.

Go:
- `internal\pricing\pricing.go`: `Config.Mode`, `BusinessMode()`, `Config.View`,
  `MonitorView()` (~223-263). Unknown keys in an old `burnmon.json` must still load without error.
- `internal\dataset\dataset.go`: `Payload.Mode` and view fields (~109-115, ~189-190).
- `cmd\burnmon\app.go`: `settingsPayload.DefaultView`, `applySettings`, the "Default view"
  select in `settingsModalHTML`, `defaultView` in the JS payload (~437-479, ~721-803), stale
  comment on `writeConfigKey`.
- `cmd\burnmon\main.go`: `ccSaveView` binding (~518-534).
- `burnmon.example.json`: key `"mode"` (~32) and any `"view"` key.
- Tests: `internal\pricing\pricing_test.go`; delete `tools\uicheck\check_u5.go`, `check_v3.go`
  and `check_v3b.go` cases that only test monitor/mode; update the lists in
  `scripts\uicheck.ps1` (~21, ~28).

Done when: no match for `btn_mode|btn_monitor|isBusiness|MonitorView|BusinessMode|DefaultView|ccSaveView|monitor_text`
in the repo (except SESSION_LOG and `02_roadmap\`), and an old config with `"mode":"business"` still loads.

## Task 2: Sessions tab shows every harness

Diagnosis (verified by read): all vendors already reach the sessions table. They look like
Claude because `scan.Session` (`internal\scan\types.go:33+`) has no vendor/agent field, only
`Surface`, and `SurfaceLabel` (types.go:15-20) plus `SHORT` (template.html ~797) map `cli` to
"Claude Code". Codex and Copilot CLI sessions are relabelled as Claude.

Fix:
1. Add `Vendor` and `Agent` to `scan.Session`; fill them in `SessionsFromEvents`
   (`internal\dataset\fromstore.go:18-44`) from the event. Keep them through `dedupSessions`
   (dedup key must include vendor) and `agg.Build`.
2. Label by agent: Claude Code, Claude Desktop, Cowork, Codex, Copilot CLI, Copilot (VS Code),
   Hermes. Surface becomes a secondary label, not the harness name.
3. Add a harness filter (chips or select) to the Sessions tab, default All.
4. Pricing: `fromstore.go:117-134` prices non-OpenAI vendors through Claude model families.
   Route cost through the V3-1 price books (`CostForEvents`) per vendor; where no price exists,
   show "no price" instead of a Claude price.
5. Copilot CLI writes totals only at shutdown (per STATUS). Show those sessions with a
   "totals at exit" note rather than hiding them.

Done when: with the real store, the Sessions tab lists sessions for every vendor with data in
the vendor strip this month (Claude Code, Codex, Copilot VS Code, Copilot CLI, Cowork, Hermes),
each with its own label, and a unit test covers two vendors sharing a session id.

## Task 3: History "Burn per period" as stacked bars per vendor, yyyy-mm-dd axis

1. `internal\history\history.go` `Build` (~149): add a per-vendor split per row, e.g.
   `by_vendor: {agent: {tokens, cost_usd}}`. Agent is on every event.
2. `drawHistoryChart` (template.html ~2722-2741): one Chart.js dataset per vendor,
   `stacked: true` on both axes, legend on, fixed colour per vendor shared with the vendor strip.
   Token and cost toggles both stack.
3. X-axis label is the period start date as `yyyy-mm-dd` for day, week and month
   (`histPeriodLabel` ~2703). Tooltip may add the weekday or week number.

Done when: day, week and month views stack correctly (segment sum equals the old single bar,
unit test in `history`), and every x label matches `^\d{4}-\d{2}-\d{2}$`.

## Verify and close

- `go vet ./...`, `go test ./... -count=1`, `.\build.ps1`, `node --check` on the template JS,
  `.\scripts\uicheck.ps1` for the remaining checks. `w8` and `w1` failures are pre-existing
  (see hub one-pager); report, do not fix.
- A fresh read-only review agent (Opus) over the diff before commit.
- Bump to v0.3.1 (semver patch per `C:\ZND\AGENTS.md`; version lives in
  `cmd\burnmon\app.go` ~35 and `cmd\burnmon-cli\main.go` ~39), update README and STATUS,
  prepend SESSION_LOG.
- Do not push or tag. Hand Wilco the exact PowerShell commands (with `cd`).
- Write one hub brief with the `hub-agent-update` skill in `04_assets\`.
