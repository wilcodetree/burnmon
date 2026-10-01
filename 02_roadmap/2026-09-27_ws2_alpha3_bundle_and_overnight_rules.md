# WS2 alpha.3, part 2: bundled parked items and overnight rules

Companion to `C:\ZND\50_projects\burnmon\02_roadmap\2026-09-27_ws2_alpha3_system_cadence_todo_scroll.md`
(part 1, items 1 to 3). Wilco, 2026-09-27: bundle four parked items into the same session, run
it as one unattended overnight session. Source of the parked items:
`C:\ZND\50_projects\burnmon\02_roadmap\2026-09-26_parked_after_alpha2.md`. Never use em dashes.

## Bundled items (numbered on from part 1)

4. **Minimized RAM** (parked 1). The Go process peaked at 297.2 MB minimized against 195.4 MB
   active and a 250 MB target. Part 1 item 3 already drops everything, process walk included,
   to 10 s when hidden. Also reuse the process-snapshot buffer instead of a fresh 2 MB per walk.
   Measure 5 minutes minimized: Go peak under 250 MB, or report why not.
5. **To Do due dates in local time** (parked 3). Graph returns `dueDateTime` as
   `{dateTime, timeZone}`. Show the due date as a local calendar date, overdue compared with
   local today. Unit tests with UTC and non-UTC `timeZone` values and a DST date. Wilco is
   signed in, so one live read is allowed to confirm the format, but log only counts and field
   shapes, never task titles, lists or dates, and never screenshot the panel with real rows.
6. **d9 at 1920x1080** (parked 4). `.heatmonth` is 9 px wide with `overflow:visible`, month
   labels spill past their box and trip `tools\uicheck\check_d9.go`. Fix the layout, not the
   check. d9 must pass at all five sizes.
7. **w1 solo re-run** (parked 5). Run `w1` alone at the end, on an idle desktop, and report.

Out of scope: the WebView2 memory target (parked 2), everything on main-only code (parked 7 to 9).

## Overnight rules (nobody at the keyboard)

- Never push, tag, merge, rebase, reset, or delete a branch. Commit on `main` only.
- Never ask a question: there is nobody to answer. Take the conservative option, write it down
  as a decision in the report, move on.
- Order: code and unit tests first (items 1, 2, 4, 5, 6 and the code of item 3), then build,
  then the real-window checks and measurements, then the Opus review, then fix, commit, docs,
  hub brief. So a late failure still leaves committed, tested code.
- The laptop stays on and unlocked, but may lock anyway (company policy). If the desktop is
  locked or a real-window check sees a foreign foreground window, stop the real-window part,
  record exactly which checks did not run, and finish everything else. Never report a check as
  passed that did not run.
- A stuck process (build, uicheck, measurement) gets at most 15 minutes, then kill it and
  record it.
- One commit per finished item is fine; every commit gets its own fresh read-only Opus review.
- The report is the last thing you print. It must stand on its own for Wilco in the morning.
