# WS2 follow-up: one colour family per vendor, shades per session

Owner: Wilco (request 2026-09-28, screenshot at 06:28). Executor: the same short follow-up
session as the month labels fix, main in `C:\ZND\projects\burnmon`. Never use em dashes.
Supersedes the 2026-09-24 decision "distinct colours per session and harness": sessions are now
distinct shades inside their vendor's colour, not free hues.

## What is wrong today (read from `cmd\burnmon-dev\page.html`)

- The burn chart and its legend colour sessions from `SESSION_PALETTE` in order of appearance
  (`sessionColor`, line ~551), regardless of vendor. So the first of two Cowork sessions got
  orange, which is Claude Code's colour, and the second got blue.
- The session cards and the vendor strip colour by harness (`HARNESS_COLOR`, line ~449), so the
  same Cowork session is orange in the chart and blue on its card.
- Copilot CLI `#38d3e3` sits too close to Cowork `#4da6ff`.

## Wanted

1. One palette, per vendor, used everywhere a vendor or its session appears: burn chart bars,
   burn legend, session cards (bar and title), vendor strip, process groups, harness heatmap.
   One source in the code, no second copy.
2. Base colours stay as they are for Claude Code (`#ffa028`), Codex (`#a78bfa`), Copilot (VS
   Code) (`#4ade80`) and Cowork (`#4da6ff`). Copilot CLI gets a new base hue, clearly distinct
   from all of those and from Hermes (`#e879f9`), WSL and the other process-group rows. Name the
   hue and the reason in the report.
3. Several open sessions of one vendor: each gets a shade of that vendor's base, far enough
   apart to tell apart at a glance on the dark background (step lightness, not hue). The same
   session keeps the same shade in the chart, the legend and its card for its whole life. Cap
   the number of shades (for example 4) and wrap, rather than drift into another vendor's hue.
4. Two legend entries with identical text (today both read "cowork host-cwd") must be told
   apart: append a short session id when labels collide.

## Proof

- A uicheck d-check with fake sessions (two Cowork, two Claude Code, one Copilot CLI, never live
  To Do or real data needed): each session's chart colour equals its card colour; all shades of
  one vendor share the vendor hue within a few degrees; any two shades differ clearly (report
  the lightness step); no session colour falls inside another vendor's hue range.
- Screenshot of the burn zone with those fake sessions, committed under
  `C:\ZND\projects\burnmon\04_assets\reference\2026-09-28_vendor_colours\`.
- `README.md` BurnMon Dev section: one line on the colour rule.
