# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-28 - **Owner:** Wilco de Tree
**Project:** BurnRate (BurnMon)
**Purpose:** WS2 follow-up on `main` (month labels, To Do privacy, vendor colours, System time axis) shipped as v0.4.0-alpha.4, committed, not pushed or tagged.
**Read order:** this file, `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry, 2026-09-28), `C:\ZND\projects\burnmon\STATUS.md`'s own v0.4.0-alpha.4 paragraph.
**Supersedes:** nothing (first brief for this follow-up; the alpha.3 brief `hub_agent_update_2026-09-27_ws2_alpha3_overnight_launched.md` is the prior one for this same track).

## 1. Headline
Four items Wilco asked for directly (month labels fixed for real, Microsoft To Do forced
off under `tools\uicheck`, session/vendor colours rebuilt as one shaded family per vendor,
System chart moved to a real time axis) are code-complete and committed on `main` as
`v0.4.0-alpha.4` (`a7b398e`), after a fresh independent Opus review caught and fixed two
Important issues first. Not pushed, not tagged.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (`main`):
  - `2d237bc` - docs: alpha.3 overnight logs and hub briefs, WS2 follow-up roadmap specs.
  - `a7b398e` - v0.4 WS2 follow-up: month labels, To Do privacy, vendor colours, System
    time axis, v0.4.0-alpha.4.
- **Files touched** (full paths, `a7b398e`): `C:\ZND\projects\burnmon\README.md`,
  `SESSION_LOG.md`, `STATUS.md`, `cmd\burnmon-dev\app.go`, `cmd\burnmon-dev\main.go`,
  `cmd\burnmon-dev\page.html`, `cmd\burnmon-dev\todo_gate.go` (new),
  `cmd\burnmon-dev\todo_gate_test.go` (new), `tools\uicheck\check_d14.go` (new),
  `tools\uicheck\check_d15.go` (new), `tools\uicheck\check_d16.go` (new),
  `winres\burnmon-dev.json`, `04_assets\reference\2026-09-28_vendor_colours\` (two
  reference screenshots, fake data only).
- **Docs-only, `2d237bc`:** `02_roadmap\2026-09-28_ws2_vendor_colour_families.md`,
  `02_roadmap\2026-09-28_ws2_system_chart_time_axis.md` (Wilco's own specs), plus
  alpha.3's own leftover logs and four hub briefs it never committed.
- **Left out on purpose:** `go.mod`'s own pre-existing, unrelated line-ending-only change
  (working tree still shows it modified, same as the alpha.3 commit before this one).

## 3. What did NOT happen (and why)
- **Not pushed, not tagged.** House rule: commit and stop, hand Wilco the exact commands.
- **No hub propagation.** This brief has not been folded into STATUS/DEADLINES/portfolio;
  that is the sibling `hub-update` skill's job, run separately.
- **`d7`/`d8` uicheck checks were not cleanly green on the first two sweep attempts** -
  traced to the laptop's screen locking mid-session (company policy) and to
  keyboard/window-focus contention with the terminal driving the checks, not to anything
  in this session's diff (neither check touches month labels, To Do, colours or the System
  chart). Confirmed clean on a later attempt once the contention cleared.
- **One near miss, caught before it reached disk:** a screenshot taken while the screen was
  locked briefly sat in the reference folder in place of a real one; noticed by visually
  inspecting the image (it showed the Windows lock screen), never committed, re-taken after
  Wilco unlocked.
- **No live Microsoft Graph read this session** - out of scope; item 2 is about *never*
  calling Graph under `tools\uicheck`, not about the panel's normal-use behaviour.

## 4. Findings worth propagating
- [RESULT] `go vet ./...`, `go test ./... -count=1` (every package), `.\build.ps1`,
  `node --check` on `cmd\burnmon-dev\page.html`'s script: all green, run twice (before and
  after the review's fixes).
- [RESULT] `scripts\uicheck.ps1 d0`-`d16` (full sweep, `testdata\uicheck\out\*.png` deleted
  first): all 17 checks pass on the final code, confirmed live after the screen-lock/focus
  contention above cleared.
- [RESULT] A fresh, independent read-only review (Claude Opus 5.5) of the staged diff found
  two Important issues before commit, both fixed and re-verified live, not just reworded:
  (1) the vendor-colour shade index could repeat for two sessions of one vendor open at the
  same time (the counter never freed a slot when a session ended); (2) the System chart's
  own axis was anchored to a locally-recomputed "now" instead of the burn chart's real
  `window_start`, so the two axes' labels did not actually align on real data despite a
  synthetic test fixture hiding the gap. Full findings list in `SESSION_LOG.md`.
- [STATE] `2026-09-28_ws2_system_chart_time_axis.md`'s own proof section says a
  10-minutes-of-data case should show its first point "about one third of the width from
  the left"; the physically correct result (data running up to now, not away from it) is
  the newest third filled, i.e. two thirds from the left. Implemented the correct math,
  flagged the apparent inversion in `SESSION_LOG.md` rather than silently matching the
  wording. Wilco has not yet confirmed which reading he meant.
- [STATE] A hidden/minimized stretch now draws as no line at all on the System chart
  (rather than a coarser one), a direct consequence of the spec's own 5-second gap
  threshold against the 10-second hidden-sample cadence. Flagged in `SESSION_LOG.md` for
  Wilco to weigh in on if that reads as "data vanished" rather than "app was minimized".

## 5. Hub-level decision
Nothing - every call this session was project-internal (UI/rendering correctness, a
privacy gate for an internal dev tool), no cross-project time, park/unpark, consultancy,
company positioning or CIPHER-wall question involved.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` (the hub one-pager) - current version is
  v0.4.0-alpha.4, committed not pushed.
- Nothing else canonical changes; this is a small follow-up, not a milestone.

## 7. Open flags for next session
- Push and tag `v0.4.0-alpha.4` once Wilco reviews (exact commands below).
- Wilco to confirm the "one third vs two thirds" reading in
  `2026-09-28_ws2_system_chart_time_axis.md`'s own proof section (see Findings above) -
  the doc itself may want a wording fix once he confirms which he meant.
- Wilco to confirm whether a blank System chart during a hidden/minimized stretch is the
  wanted behaviour, or whether the gap-break threshold needs a second look.

## 8. Related files
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-28_ws2_vendor_colour_families.md`
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-28_ws2_system_chart_time_axis.md`
- `C:\ZND\projects\burnmon\04_assets\reference\2026-09-28_vendor_colours\d15-vendor-colours.png`
- `C:\ZND\projects\burnmon\04_assets\reference\2026-09-28_vendor_colours\d16-system-time-axis.png`
- `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry, full writeup and review findings)
