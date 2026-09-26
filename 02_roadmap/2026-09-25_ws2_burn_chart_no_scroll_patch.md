# WS2 patch 2: burn chart bars only, no scrollbars anywhere (2026-09-25)

Branch `burnmon-dev`, worktree `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev`.
Overrides `2026-09-25_ws2_ui_review_patch.md` where they differ. Never use em dashes anywhere.

Reference screenshots (copied by Wilco): `04_assets\reference\2026-09-25_ui_review\`
`9_burn_chart_lines.png` (the lines to remove, circled in red) and
`10_scrollbars.png` (the scrollbars to remove). If missing, work from the words below.

## 1. Burn, last 30 min: bars only

The earlier patch said "keep the turn markers". That was wrong; Wilco wants none of it.
- Remove the grey turn tick marks along the top of the chart.
- Remove the coloured vertical finding lines (amber, red, cyan).
- Nothing in this chart except the stacked bars, the y-scale if any, and the `HH:mm` axis
  every 5 minutes.
- Findings stay visible in the turn ticker tags and the turn popup. Advisor rules stay for
  the export.

## 2. Clear colours per session

Today Claude Code and Cowork are both orange and the per-session shades are too close to see.
- Give each session in the 30-minute window its own colour from one fixed categorical palette
  with clearly different hues (at least 8), readable on the black background. Same session,
  same colour across refreshes (assign by first appearance, keep a stable map).
- Also fix the harness colours so no two harnesses share a hue (Claude Code, Cowork, Codex,
  Copilot CLI, Copilot VS Code, Hermes, BurnMon Dev, WSL, Other). Use them in the vendor strip,
  ticker, process groups and heatmap.
- Add a compact legend under the chart: colour chip, harness, short session label, tokens in
  the window. Crop the legend to one or two lines; drop the smallest sessions first.

## 3. No scrollbars anywhere

No visible scrollbar in any panel or on the page, at any window size. Content crops to what
fits, or is left out.
- Activity heatmap: show as many whole weeks as fit, newest on the right, oldest cropped on
  the left. Month labels only for visible weeks. "N active days" still counts the full 26 weeks.
- Harness x minute heatmap: as many whole minutes as fit, newest on the right.
- Turn ticker: show as many whole rows as fit, newest first. No scroll.
- Process groups: as many rows as fit, busiest first.
- To Do: as many tasks as fit, then "+N more".
- Turn popup: sizes to its content inside the window; crop long tool lists with "+N more".
- Recompute what fits on resize and on F11.

## 4. Window too small: drop low panels (decided with Wilco)

The page never scrolls. When the window cannot show every panel, panels leave in this fixed
order: To Do first, then process groups, then the Activity and heatmap row. Header, burn chart,
vendor strip, turn ticker and system panel always stay. A dropped panel comes back when the
window grows. At the default 1280x860 state which panels show in the report.

## 5. One refresh clock for the whole app (decided with Wilco: 2 seconds)

Every chart, number and list updates on the same beat, so the screen moves as one.
- One render tick every 2 s, driven from one place in the page JS. Config key `refresh_ms` in
  `burnmon-dev.json`, default 2000, minimum 1000. Remove every other `setInterval` or
  `setTimeout` loop that paints.
- Per tick: one batched binding call returns a single snapshot for all panels, then every
  panel renders inside the same `requestAnimationFrame`. No panel paints between ticks.
- Slow data (vendor strip week and month, activity heatmap, harness heatmap, To Do) may be
  fetched less often in the background, but is only painted on the shared tick.
- All time-axis charts (burn chart, system history, harness heatmap) use the same "now" from
  the snapshot, so they shift left together.
- Align system sampling to the same 2 s tick, so the numbers and the charts show the same moment.
- All tweens (headline total, row values, bars) use one duration and easing, shorter than the
  tick (about 600 ms, easeOutQuart). Reduced motion stays instant.
- The header clock shows the snapshot time of the tick (`HH:mm:ss`, it steps by 2 s).
- If a tick's data is late, skip that paint rather than painting part of the panels.

## Verify and close

`go vet ./...`, `go test ./... -count=1`, `.\build.ps1`, `node --check` on the page JS. A uicheck
case that fails if any element has a visible scrollbar (`scrollHeight > clientHeight` or
`scrollWidth > clientWidth` with overflow not hidden) at 1024x768, 1280x860, 1152x2048,
1920x1080 and 2560x1440. A uicheck case that finds no marker or line elements in the burn
chart. A uicheck case that records paint timestamps per panel for 30 s and fails if any panel
paints outside the shared tick, or if the panels' paints within one tick are more than one
frame apart. Report the app's own CPU before and after the refresh change. Screenshots at all
five sizes. Fresh read-only Opus review before the commit. Commit on
`burnmon-dev`, do not push.
