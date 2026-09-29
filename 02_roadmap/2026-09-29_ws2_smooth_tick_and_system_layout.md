# WS2 follow-up: smooth shared tick, System gaps, System and To Do layout

Owner: Wilco (request 2026-09-28 and 2026-09-29). Executor: a Claude Code session in
`C:\ZND\projects\burnmon`, target `v0.4.0-alpha.7`. Never use em dashes.

Goal in Wilco's words: the app feels smooth, all panels update at about the same moment, never
clunky, and it never freezes.

## 1. The page freezes after about 45 minutes (read from the code, not yet measured)

What Wilco saw (2026-09-28): started 14:17:07, header clock stuck at 15:03:57. The Go process
stayed responsive (`Responding: True`) and kept logging ingest until 15:59. WebView2 group at
2.4 GB, machine at 89% RAM, 9.4 GB swap.

Likely mechanism:

- `bdevSnapshotNow` (`cmd\burnmon-dev\main.go`) returns the full `groupsHistBuf` (60 min, 1 s,
  every harness: about 25,000 rows at the end of the hour) and 30 min of `sysHistBuf` (1 s,
  20 per-core values each) on every 1 s tick. The payload grows for the first hour.
- `tick()` in `page.html` drops any answer that arrives later than 1.5 ticks. No limit on how
  many ticks in a row it drops. Once the round trip is always over 1.5 s, nothing paints again.

Measure first: log the snapshot build time, the JSON size and the page-side round trip once a
minute (p50, p95, max, dropped-paint count). Put the before numbers in the report.

Fix:

1. History as deltas, not full buffers. The page sends the newest `Ts` it holds for system and
   for process groups; Go returns only samples after it. The page keeps its own 60 min ring
   (system trimmed to 30 min for the chart) and prunes it. A first call, a reload or a cursor
   older than the buffer returns the full window once. The payload per tick becomes constant.
2. Burn part of the snapshot: check whether `st.EventsSince(30 min)` plus `BuildSnapshot` plus
   `ApplySessionTotals` every second is a real share of the time. If yes, cache on the ingest
   beat and rebuild only when new events landed.
3. Never skip forever. Keep "skip a late paint", but paint anyway when the last paint is older
   than 2 ticks. A slow snapshot then shows as lag, not as a frozen screen.
4. Keep the one-tick contract: every panel still paints from the same snapshot in one
   `requestAnimationFrame`.
5. While here: `sysHistBuf`/`groupsHistBuf` prune by reslicing, so the dropped prefix stays in
   the backing array until append grows it. Copy to a fresh slice when the dropped part is over
   half the length.

## 2. Missing data points in the System chart (2026-09-29, 09:48 to 10:17)

Screenshot: long stretches with no line and single isolated dots, around 10:03 to 10:12. The
chart breaks the line on any gap over `histGapThreshold` (25 s), so these are real gaps of over
25 s between two samples in `sysHistBuf`. A single dot is one sample with a gap on both sides.

Find the cause before fixing. Log every sample-loop pass that takes over 2 s, split into
`sampler.Tick` and `procSampler.Tick`, and every gap over 25 s between two stored samples, with
the hidden flag and the system power state if it is cheap to read. Candidates: sleep or screen
lock (then the gap is correct), a slow process walk under full CPU (cores C12 to C19 were at
100%), or lock contention on `a.mu`/`a.sysHistMu` with the snapshot handler.

Fix what the log shows. If the sample loop itself stalls, move the process walk to its own
goroutine so a slow walk never delays the system sample. If the gap is sleep, keep the break
but draw it as a dim band so it reads as "no samples" and not as a bug.

## 3. Layout: System chart 30% lower, To Do fills the bottom

- The System line chart (`.histwrap`) gets 70% of the height it has today at the same window
  size. Every panel below it moves up.
- The Microsoft To Do panel takes the freed height at the bottom and grows to fill the window.
  It keeps its own scroll (`.todobody`, the documented exception in the no-scroll rule).
- Only when the To Do panel is visible. With To Do off, the chart keeps its current height
  (decided by Wilco, 2026-09-29).
- Keep `applyPanelFit`'s drop order and check_d9 (no page scrollbar) at every window size.

## 4. Harness heatmap: longer row labels (2026-09-29)

Labels clip today: "BurnMon Dev (WebV...", "node (unclassifie...". The label column is a fixed
110 px (`.heatrow-label` in `page.html`) and `renderHarnessHeatmap` subtracts a hard-coded 116
(110 plus the 6 px row gap) from the width.

- Widen the label column so the longest current label, "BurnMon Dev (WebView2)", fits in full
  at 11 px: about 150 px. Keep the ellipsis for anything longer.
- Replace the hard-coded 116 in `renderHarnessHeatmap` with the label width plus the gap, read
  from one constant or measured, so the two never drift apart again.
- Raise `ACTIVITY_HARNESS_MIN_PX` by the same 40 px so the minute cells do not shrink.

## 5. Memory, Disks and Network boxes on one grid (2026-09-29)

Wilco's mock-up screenshot (2026-09-29) is the target. Every box uses the same row pattern:
a value line, then its bar, then a dim detail line. Rows line up across the three boxes.

- Memory: "86% · 27.1 / 31.6 GB used", bar (blue), "4.5 GB available", "swap 5.6 / 45.3 GB".
  The value line moves above the bar (today the bar comes first).
- Disks: per drive "C: 86%", bar (yellow), "137G free · R ..., W ...". Unchanged content.
- Network: three bars, not zero. "down 9.3 KB/s" with a bar (yellow), "up 28 KB/s" with a bar
  (yellow), "<SSID> · signal 89%" with a bar (blue). Down and up bars scale to their own peak
  over the chart's 30-minute window, the same rule as the chart's "disk and net scaled to own
  peak". The signal bar is the signal percent. No Wi-Fi: no signal bar, show the text only.
- One shared row height for value, bar and detail rows (CSS grid rows or fixed line heights),
  so row N sits at the same height in every box whatever the drive count. All three boxes
  stretch to the height of the tallest one.
- Below 900 px the boxes stack (existing `.sysbottom` rule); the grid rule then does not apply.

## Proof

- Before and after numbers from item 1's log, from a run of at least 90 minutes on the real
  store: after the fix, p95 round trip under 250 ms, no dropped-paint streak over 2 ticks, flat
  WebView2 working set in the second hour.
- Item 2's log from a real run, with the cause named, and the fix it led to.
- uicheck: all existing `d*` checks pass. New check: 70 minutes of fake history delivered as
  deltas renders the same chart as the same data delivered in full.
- Screenshot of the harness heatmap with every label in full.
- Screenshot of the three System boxes next to Wilco's mock-up, rows aligned.
- Screenshots at three window sizes with To Do on, and one with To Do off, under
  `C:\ZND\projects\burnmon\04_assets\reference\2026-09-29_smooth_tick\`.
- Version bump to `v0.4.0-alpha.7`, README and STATUS updated, hub agent update brief.
