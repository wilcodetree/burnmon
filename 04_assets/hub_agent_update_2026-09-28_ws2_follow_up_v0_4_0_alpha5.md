# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-28 - **Owner:** Wilco de Tree
**Project:** BurnRate (BurnMon)
**Purpose:** Small fix on `main` after v0.4.0-alpha.4 (gap-break threshold vs the hidden-window sampling cadence), shipped as v0.4.0-alpha.5, committed, not pushed or tagged.
**Read order:** this file, `C:\ZND\50_projects\burnmon\SESSION_LOG.md` (top entry, 2026-09-28, "WS2 follow-up fix"), `C:\ZND\50_projects\burnmon\STATUS.md`'s v0.4.0-alpha.5 paragraph.
**Supersedes:** nothing (first brief for this fix; picks up the alpha.4 brief's open flag "Wilco to confirm whether a blank System chart during a hidden/minimized stretch is the wanted behaviour" - answer: no, it was a bug, now fixed).

## 1. Headline
The System chart and process-groups sparklines drew every minimized/hidden stretch as no line
at all (not "coarser") because the gap-break threshold (a hard-coded 5s in `page.html`) was
narrower than the app's hidden-sampling cadence (10s). Fixed and code-complete on `main` as
`v0.4.0-alpha.5` (`8d64ae9`), after a fresh independent review caught the new check proving the
wrong thing and it was fixed before commit. Not pushed, not tagged.

## 2. What changed on disk
- **Committed** in `C:\ZND\50_projects\burnmon` (`main`):
  - `8d64ae9` - v0.4 WS2 follow-up fix: gap-break threshold vs hidden sampling cadence,
    v0.4.0-alpha.5.
- **Files touched** (full paths, `8d64ae9`): `C:\ZND\50_projects\burnmon\README.md`,
  `SESSION_LOG.md`, `STATUS.md`, `cmd\burnmon-dev\app.go`, `cmd\burnmon-dev\main.go`,
  `cmd\burnmon-dev\page.html`, `tools\uicheck\check_d16.go` (one stale comment reworded),
  `tools\uicheck\check_d17.go` (new).
- **Not yet committed, this session's own docs-only pass still pending:** the amended
  `02_roadmap\2026-09-28_ws2_system_chart_time_axis.md` (its Proof section corrected
  2026-09-28) and the prior session's own uncommitted
  `04_assets\hub_agent_update_2026-09-28_ws2_follow_up_v0_4_0_alpha4.md` - both going into a
  separate docs commit alongside this brief, per this session's own instructions.
- **Left out on purpose:** `go.mod`'s pre-existing, unrelated line-ending-only change
  (working tree still shows it modified, same as the two commits before this one).

## 3. What did NOT happen (and why)
- **Not pushed, not tagged.** House rule: commit and stop, hand Wilco the exact commands.
- **No hub propagation.** This brief has not been folded into STATUS/DEADLINES/portfolio;
  that is the sibling `hub-update` skill's job, run separately.
- **No live Microsoft Graph read, no real minimized-window observation this session** - the
  whole check runs on synthetic data via `page.html`'s `__bdevEnterFakeMode`/
  `__bdevPaintFake` test hooks, never a real minimize/sleep cycle. The fix follows directly
  from reading `hiddenSampleInterval` and the old hard-coded `HIST_GAP_MS` side by side, not
  from reproducing the bug live.

## 4. Findings worth propagating
- [RESULT] Root cause confirmed by reading the code, not guessed: `app.go`'s
  `hiddenSampleInterval` (the cadence the sampler falls back to while the window is
  hidden/minimized) is 10000ms; `page.html`'s `HIST_GAP_MS` (the gap-break threshold for both
  the System chart and the process-groups sparklines) was a hard-coded 5000ms - every ordinary
  hidden-window sample gap already exceeded the break threshold.
- [RESULT] Fix: `app.go` gains `histGapThreshold = hiddenSampleInterval * 5 / 2` (an exact
  25000ms, no truncation - confirmed by the fresh review below), sent to the page as
  `snapshotPayload.HistGapMs` (`main.go`) and read into `page.html`'s `HIST_GAP_MS` inside
  `paintTick`, replacing the permanent hard-coded 5000 (a 25000 literal remains only as the
  page's bootstrap default for the handful of frames before the first snapshot arrives).
- [RESULT] New `check_d17.go` first reads `window.__bdevHistGapMs` off the real,
  already-running page (its last real tick's `HIST_GAP_MS`, not a synthetic one) and
  asserts it is exactly 25000 - proof the fix reaches the page end to end through the real
  `bdevSnapshotNow`, not only through the check's own fake snapshot. Then proves the System
  chart stays one continuous line across a synthetic 5-minutes-at-1s plus 5-minutes-at-10s
  stretch and breaks exactly once at a real 60s gap (331 points then 61, via the existing
  `__bdevHistDebug` hook `check_d16.go` also reads). The process-groups sparkline's own
  window (`PROCESS_GROUP_SPARK_WINDOW_MS`, 30 seconds) cannot hold both endpoints of a 60s gap
  at once, so that part of the check instead pins the exact 25000ms boundary directly: a 24s
  gap holds the line (one `M` moveto in the trend `<path>`'s `d` attribute), a 26s gap
  breaks it (two). Flagged in `SESSION_LOG.md` rather than silently forcing the literal 60s
  number onto a window three times narrower - same convention `check_d16.go` used for its own
  "one third vs two thirds" finding.
- [RESULT] A fresh, independent read-only review (Claude Opus 5.5) of the staged diff found
  one Important issue before commit: the first version of `check_d17.go` set its own fake
  `snap.hist_gap_ms`, so it proved the page honours that field but not that `histGapThreshold`
  actually reaches it through the real snapshot path - fixed by adding the
  `window.__bdevHistGapMs` live mirror described above, then re-verified live. Minor/nit
  wording issues (two comments still saying "5s"/">5s" after the threshold number changed, one
  sentence describing the wrong literal as "the fallback") reworded. `gofmt -l` flagging most
  of `cmd\burnmon-dev` and `tools\uicheck` traced to this checkout's own CRLF line endings
  (confirmed via `gofmt -d`, line-ending noise only, present even on untouched files), not to
  anything in this diff.
- [RESULT] `go vet ./...`, `go test ./... -count=1` (every package), `.\build.ps1`,
  `node --check` on `cmd\burnmon-dev\page.html`'s script: all green, run twice (before and
  after the review's fixes).
- [RESULT] `scripts\uicheck.ps1 d0`-`d17` (full sweep, `testdata\uicheck\out\*.png` deleted
  first): all 18 checks pass on the final code, run twice (before and after the review's
  fixes); `d17`'s own screenshot (`testdata\uicheck\out\d17-hist-gap-threshold.png`, not
  committed, gitignored like the rest of that folder) shows the System chart's line
  continuous across the fake 1s/10s stretches with a visible break near the right edge at the
  deliberate 60s gap.

## 5. Hub-level decision
Nothing - project-internal bug fix (a UI rendering-correctness threshold), no cross-project
time, park/unpark, consultancy, company positioning or CIPHER-wall question involved.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` (the hub one-pager) - current version is
  v0.4.0-alpha.5, committed not pushed. (This also picks up the alpha.4 brief's own
  not-yet-propagated state, since neither has gone through `hub-update` yet.)
- Nothing else canonical changes; this is a small fix, not a milestone.

## 7. Open flags for next session
- Push and tag `v0.4.0-alpha.5` once Wilco reviews (exact commands in this session's own
  final report).
- The separate docs commit (amended roadmap spec plus the prior session's own hub brief plus
  this one) still needs to happen; by the time this file is read it should already be
  committed alongside it, per this session's own instructions.
- Alpha.4's own still-open flag ("one third vs two thirds" wording in
  `02_roadmap\2026-09-28_ws2_system_chart_time_axis.md`) is unrelated to this fix and remains
  open for Wilco.

## 8. Related files
- `C:\ZND\50_projects\burnmon\02_roadmap\2026-09-28_ws2_system_chart_time_axis.md` (amended
  2026-09-28 with this fix's own Proof addendum)
- `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-09-28_ws2_follow_up_v0_4_0_alpha4.md`
  (the prior brief for this same track)
- `C:\ZND\50_projects\burnmon\SESSION_LOG.md` (top entry, full writeup and review findings)
- `C:\ZND\50_projects\burnmon\tools\uicheck\check_d17.go`
