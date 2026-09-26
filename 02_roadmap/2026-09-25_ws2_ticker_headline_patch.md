# WS2 patch 3: turn ticker, process groups, heatmap hover, headline (2026-09-25)

Branch `burnmon-dev`, worktree `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev`.
Overrides `2026-09-25_ws2_burn_chart_no_scroll_patch.md` where they differ. Version stays
0.4.0-alpha.1; phase 5 runs after this patch. Never use em dashes anywhere.

Reference screenshots (copied by Wilco into `04_assets\reference\2026-09-25_ui_review\`):
`11_ticker_broken.png`, `12_process_groups.png`, `13_heatmaps.png`, `14_topbar.png`.
Wilco checked these on his real screens; everything not listed here he calls "very nice".

## 1. Turn ticker: broken rows, and it scrolls

`11_ticker_broken.png` shows the fit logic squashing every row to a few pixels: only dashes and
the top border of each finding tag remain, no times, names or token counts.
- Rows get a fixed height (about 22 px), never squeezed.
- The ticker is the one exception to "no scrollbars": it scrolls vertically with a thin
  scrollbar in the house style (dark track, amber-grey thumb). Newest first, still capped at 50.
- Row content as in patch 1 section 5: time, harness in its colour, optional outlined finding
  tag, tokens. Click opens the turn popup.
- Add a uicheck case that fails when any ticker row is shorter than its text line height.

## 2. Process groups: trend max half the panel

The trend column takes at most 50 percent of the panel width. Name, CPU, RAM and IO/s spread
over the other half (name left, numbers right-aligned, headers aligned with their numbers).

## 3. Harness heatmap: hover text like Activity

Each cell shows a hover text in the same style as the Activity heatmap: harness, `HH:mm`,
CPU percent for that minute, and tokens in that minute when there are any. Empty minutes show
"no activity".

## 4. Headline total: whole number, rolling, more room

- Show today's total as the whole number with thousands separators, no K, M or B
  (for example `399,812,345`). Comma separator, as the rest of the UI is English.
- Bigger and with more room: about 48 px at 1152 px wide and up, at least 36 px on smaller
  windows. Reserve width for 12 digits plus separators so the header never shifts while the
  number grows. Keep `tabular-nums`.
- It must visibly roll: this is the one exception to section 5's shared 600 ms tween. The
  headline tween runs over the whole 2 s tick with linear easing, so the digits keep running
  while tokens flow and land on the true value at the next tick. Other panels still paint on
  the shared tick. Reduced motion stays instant.
- `14_topbar.png` shows the "3 sessions" stat clipped at its left edge. Fix the overlap; no
  header element may clip another at any window size.

## Verify and close

`go vet ./...`, `go test ./... -count=1`, `.\build.ps1`, `node --check` on the page JS, uicheck
d0 to d11 plus the new ticker case. Size the uicheck windows in CSS pixels (correct for the
display scale), so "1152x2048" really means 1152x2048 CSS px; report the scale you found. A
fresh read-only Opus review over the diff before the commit. Commit with the four reference
screenshots on `burnmon-dev`. Do not push, tag or merge. Prepend SESSION_LOG, saying phase 5 is
still to run.
