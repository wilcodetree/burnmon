# WS2 follow-up: System line chart on a real 30-minute time axis

Owner: Wilco (request 2026-09-28). Executor: the same follow-up session as the month labels and
vendor colours fixes, main in `C:\ZND\50_projects\burnmon`, v0.4.0-alpha.4. Never use em dashes.

## What is wrong today (read from the code)

- The data is the last 30 minutes: `main.go` filters `sysHistBuf` on `live.ChartWindow`
  (`internal\live\live.go`, `30 * time.Minute`).
- But `renderHistoryChart` in `cmd\burnmon-dev\page.html` places each sample by index,
  `x = i / (vals.length - 1) * w`, not by its timestamp, and draws no time labels. So a fresh
  start stretches a few minutes over the full width, 10 s samples while hidden look as dense as
  1 s samples, and the chart does not line up with the burn chart above it.

## Wanted

1. X is time: left edge `now - 30 min`, right edge `now`, the same `now` the snapshot already
   shares with the burn chart. Each point at its own `Ts`.
2. Time labels under the chart in the same style, font and 5-minute steps as the burn chart's
   axis (24-hour local time), so both charts read as one timeline.
3. Less than 30 minutes of data: the line starts part-way across, the left part stays empty.
4. A gap longer than 5 s between two samples (app hidden, sleep) draws as a break in the line,
   not a straight segment across it.
5. The process-groups trend sparklines keep their own short window; say in the report what it is
   and whether it is time-based too. Do not change it in this item unless it is index-based in the
   same way, then make it time-based with the same rules.

## Proof

- uicheck with fake samples: 10 minutes of data up to now puts the first point at about two
  thirds of the width from the left (corrected 2026-09-28: the first version of this spec said
  "one third", which was wrong; the alpha.4 session implemented the correct two thirds); a 60 s
  gap gives a break; the labels match the burn chart's labels for the same `now`.
- Amended 2026-09-28 (v0.4.0-alpha.5): item 4's fixed 5 s break threshold hid every minimized
  stretch, because hidden sampling runs at 10 s. The break threshold is now 2.5 times the
  hidden sample interval (25 s), taken from the Go constant, not hard-coded in the page.
- Screenshot of the System chart with fake data, committed under
  `C:\ZND\50_projects\burnmon\04_assets\reference\2026-09-28_vendor_colours\` next to the colour one.
