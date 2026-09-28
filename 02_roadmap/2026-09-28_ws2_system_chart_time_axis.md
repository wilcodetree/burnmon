# WS2 follow-up: System line chart on a real 30-minute time axis

Owner: Wilco (request 2026-09-28). Executor: the same follow-up session as the month labels and
vendor colours fixes, main in `C:\ZND\projects\burnmon`, v0.4.0-alpha.4. Never use em dashes.

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

- uicheck with fake samples: 10 minutes of data puts the first point at about one third of the
  width from the left (report the pixel ratio); a 60 s gap gives a break; the labels match the
  burn chart's labels for the same `now`.
- Screenshot of the System chart with fake data, committed under
  `C:\ZND\projects\burnmon\04_assets\reference\2026-09-28_vendor_colours\` next to the colour one.
