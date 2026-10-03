# Station CPU climb, measurements of 2026-10-03

Step 2 of `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`.
Build: `HEAD` (`e42ecc3`, v0.4.0-alpha.10), `burnmon-dev.exe` copied to scratch, started with
`BURNMON_DEV_UICHECK=1`, Station opened by a synthetic `keydown` (`p`) over the eval channel
(no real keys). Tools: `C:\ZND\50_projects\burnmon\04_assets\2026-10-03_measure_dev_cpu.ps1`
(every 10 s: CPU seconds of the Go process and of its msedgewebview2 children, split by
`--type`), `C:\ZND\50_projects\burnmon\04_assets\2026-10-03_paintcost.ps1`. CPU is percent of
one core (20 logical cores). Window 1936x1119 CSS px at dpr 1 on the 3440x1440 monitor.
The laptop was in normal use, a Claude session was working in parallel (see "Not controlled").

## What was run

| Run | State | Length | Valid |
|---|---|---|---|
| 1 | Station open (P), site theme | 30 min | yes |
| 2 | Station open, window on the laptop panel | 10 min | no, page paused (rAF counter flat for 5 min) and the window moved |
| 3 | Station closed | 7 min | no, another session sent a synthetic `p` (HUD nodes appeared at about 209 s), restarted |
| 3b | Station closed | 30 min | yes |

## Five-minute windows (Go + all WebView2 processes)

Run 1, open: 22.3, 17.0, 13.0, 13.9, 18.2, 31.7 percent. Go 4.5, 3.9, 3.3, 3.6, 4.2, 5.3.
Run 3b, closed: 15.4, 8.6, 8.3, 9.0, 11.4, 11.9 percent. Go 6.4, 3.9, 3.3, 3.2, 3.4, 3.3.

There is no ramp of the size STATUS records (24.1, 44.6, 63.9 and 14.8, 25.7, 37.1).

## What moves the numbers

1. **Page animation rate.** The page runs `animateNumber` tweens (one `requestAnimationFrame`
   chain per changing number, 60 a second while tokens flow). Per minute, total CPU against
   page rAF calls a second: r = 0.76 open, r = 0.70 closed; about 0.33 to 0.37 points per
   rAF/s. Open run 1: minutes with 8.5 rAF/s cost 12.4 to 13.2, minutes with 43 to 50 rAF/s
   cost 27 to 40. A session burning tokens is what drives the tweens.
2. **The Station itself** adds a steady cost: quiet minutes (rAF/s at most 10.5, minute 8
   on) averaged 13.8 open against 9.9 closed; minutes 8 to 16 12.9 against 8.4, about
   4.5 points.
3. **A slow creep, with the Station closed too.** Quiet minutes: total slope 0.26 points a
   minute open, 0.22 closed. Closed, it is the GPU process (slope 0.21 a minute, 1.5 percent
   at minute 10 to 5.8 at minute 30); Go slope about 0. Renderer working set grew from 91 to
   149 MB closed and 94 to 190 MB open in 30 minutes.
4. **Page paint cost grows with the history rings** (fake mode, synthetic rows,
   `paintcost.ps1`, p50 of 15 `paintTick` calls): 4.5 ms (1 min of rows), 3.3 (5), 5.0 (15),
   7.0 (30), 8.5 (60), 10.1 ms (90 min; the Go side caps the groups ring at 60). JS only, the
   canvas raster work on the GPU process is not in this number.

## Not found, inferred, not controlled

- **Not reproduced:** a +10 to +20 point climb per 5-minute window with the Station open.
- **Inferred, not tested:** the GPU creep comes from `renderHistoryChart` (resets
  `canvas.width` and strokes up to 1800 points for each series every tick) and the heatmaps
  walking `groupsRing`; both grow for the first 30 to 60 minutes. No bisect was run.
- **Not controlled:** how many sessions were live (the Station showed 4 agents in the
  contaminated run), the other Claude session's token flow (it moves the tween rate), the
  display refresh rate, and the screen: the earlier A/B runs were recorded on the laptop's
  own screen at 200 percent scaling; today three monitors are attached, the window sat on
  the big one at dpr 1, and the laptop panel itself read dpr 1 (run 2). Not repeated at dpr 2.
- `performance.memory` stays quantised (9.5 MB, then 23 to 33 MB), so JS heap is not usable.
- Run 1 and 3b CSV files are in the session scratch directory only.
