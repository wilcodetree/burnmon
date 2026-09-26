# WS2 patch: Wilco's UI review of BurnMon Dev (2026-09-25)

Branch `burnmon-dev`, worktree `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev`.
Parent brief: `C:\ZND\projects\burnmon\02_roadmap\2026-09-24_ws2_burnmon_dev.md`. This patch
overrides it where they differ. Never use em dashes anywhere.

Reference screenshots (copied by Wilco into the worktree):
`04_assets\reference\2026-09-25_ui_review\` with `5_topbar_target.jpg` (wanted header),
`6_ticker_target.png` (wanted turn ticker), `7_perfadvisor_system.png` (wanted system panel
style), `8_current.jpg` (current state). If a file is missing, work from the words below.

## Decisions taken with Wilco (do not reopen)

- Burn chart: bars only, the CPU line overlay goes.
- Wide screens: two columns from about 1600 px wide, one column below that.
- Microsoft To Do: own sign-in, not perfadvisor's token file.
- Time is 24-hour everywhere (`HH:mm:ss`, axis `HH:mm`). No AM/PM anywhere.

## 1. Top bar (see 5_topbar_target.jpg)

Smaller and quieter. Left: "BURNMON DEV" with "PRESSURE nn" as small coloured text under it
(green, yellow, red), no box. Then the running today total, large (keep the tween and
tabular-nums), with no sub text. Then plain inline stats: sessions, $/min, tok/min, CPU, RAM,
clock. Right: "EXPORT" as a small text button, no box. Remove the search bar and every boxed
chip. Week and month totals move to the vendor strip.

## 2. Burn, last 30 min

Bars per session per time bucket, stacked, colour by harness (shade per session inside a
harness), rebuilt on every refresh. X-axis labels `HH:mm` every 5 minutes. Keep the turn
markers. With no sessions, still draw the empty axis.

## 3. Vendor strip

Totals row at the top of the table (bold). Column headers right-aligned like the numbers; only
the name column is left-aligned. Columns today, week, month.

## 4. Activity and harness heatmap side by side

One row: Activity (about 40 percent width) and "Heatmap, harness x minute (CPU-weighted)"
(about 60 percent). In one-column layouts below 900 px they may stack.

## 5. Turn ticker (see 6_ticker_target.png)

One dense line per turn, newest first, capped at 50, scroll inside the panel: time in dim
grey, harness name in its harness colour (bold), an optional finding tag as an outlined amber
uppercase box (for example CONTEXT-RUNWAY), then tokens in white ("191K tok"). Hover highlights
the row. Click opens the turn detail popup (the equivalent of `burnmon.exe`'s turn drawer:
model, token split, cache hit, cost, tools, findings). Esc closes it.

## 6. System panel (see 7_perfadvisor_system.png)

Replace the CPU heat grid and the four tiles with perfadvisor's layout, drawn smoother and a
little smaller:
- CPU box: total bar with percent and base clock, then per-core bars in a 5-column grid with
  percent per core.
- History chart, newest right: cpu, ram, disk, net, gpu as smooth canvas lines (not braille),
  legend with current values, "disk and net scaled to own peak". Colours as perfadvisor: cpu
  green, ram blue, disk yellow, net magenta, gpu red.
- Bottom row of three boxes: memory (RAM bar, used of total, available, swap), disks (bar per
  drive, free, read and write rates), network (down and up rates with totals, wifi name and
  signal bar).

## 7. Process groups

Headers right-aligned like the numbers. The trend column takes all remaining width and the
sparkline stretches to fill it.

## 8. Today's read

Delete the panel. Keep the advisor rules: the export `summary.md` still uses them.

## 9. Microsoft To Do panel at the bottom

Port `C:\ZND\projects\perfadvisor\internal\todo\` (device code flow, `Tasks.Read
offline_access`, Graph v1.0) into `internal\todo\` with a source-commit note. Off by default,
a config key in `burnmon-dev.json` turns it on. Own token cache under `%LOCALAPPDATA%\burnmon\`.
Sign-in shows the code and URL in the panel and opens the browser. Read-only. Task data is live
only: never in exports, logs or screenshots committed to the repo. The panel fills the
remaining bottom space.

## 10. Window, fullscreen, responsive

- First launch: a normal app window, 1280x860, centered. After that, remember size, position,
  maximized state and monitor; restore them if that monitor still exists.
- F11 toggles fullscreen, Esc also leaves it.
- Responsive, no fixed viewport: below 900 px one compact column; 900 to 1599 px one column
  (the portrait design); from 1600 px two columns, burn panels left and system panels right,
  header and To Do full width. Panels grow and shrink with the window; no horizontal scroll.
- Check viewports: 1280x860, 1024x1152, 1152x2048, 1920x1080, 2560x1440. In fullscreen at
  1152x2048 and 2560x1440 everything fits without vertical scroll.

## 11. Startup: fast window, loading screen, then UI

1. Measure first. Log timings for: window created, store open and migrations, initial collect
   and backfill, watcher and poller start, first system sample, first full render. Report the
   numbers before changing anything.
2. Show the window at once with a loading screen in the house style that lists each step live
   ("Opening store", "Reading sessions: 412 of 1,190 files", "Starting watchers", "Sampling
   system") with a progress bar.
3. Show the full UI as soon as the store's existing data can render. Backfill and heavy work
   continue in the background with a small status line in the header until done.
4. Do not change shared ingest code on `main` (the memory and handle bug is separate). Startup
   ordering inside `cmd\burnmon-dev` is in scope.
5. Report time to window and time to full UI, before and after.

## Verify and close

`go vet ./...`, `go test ./... -count=1`, `.\build.ps1`, `node --check` on the page JS, uicheck
`d*` cases updated for the new layout, screenshots at all five viewports plus fullscreen. A
fresh read-only Opus review agent over each diff before each commit. Commit per section group
on `burnmon-dev`, do not push. Stop and report: commits, startup numbers, screenshots, what the
review caught, and anything you could not match to the reference screenshots.
