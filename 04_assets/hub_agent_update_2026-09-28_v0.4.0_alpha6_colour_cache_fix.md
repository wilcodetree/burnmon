# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-28 - **Owner:** Wilco de Tree
**Project:** BurnMon (BurnRate node, `C:\ZND\projects\burnmon`)
**Purpose:** report a bug fix on main, v0.4.0-alpha.6: a session's burn-chart colour could lock onto the wrong grey shade for life before its vendor was known.
**Read order:** this file, then `C:\ZND\projects\burnmon\STATUS.md`'s v0.4.0-alpha.6 entry, then `C:\ZND\projects\burnmon\SESSION_LOG.md`'s matching entry (fullest narrative).
**Supersedes:** nothing.

## 1. Headline
Bug fix, committed on `main`, not pushed or tagged: a Cowork session could draw
HARNESS_HUE.other grey in the burn chart, its legend dot and its session card, while the
vendor strip and the legend text both correctly read Cowork. Root cause confirmed (Wilco's
own live report and hub-reading hypothesis both held up): `page.html`'s `sessionColor`
caches a session's colour once, forever; the chart could call it before the session's
vendor was known, locking the wrong grey shade in permanently. Fixed on both the JS and Go
side, independently reviewed (Claude Opus 5.5, read-only), full verify pass green.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (repo root, branch `main`): `789a40d` "Fix
  session colour locked grey before its vendor was known, v0.4.0-alpha.6".
- **Files touched** (full paths):
  - `C:\ZND\projects\burnmon\cmd\burnmon-dev\app.go` (version bump only, `0.4.0-alpha.6`)
  - `C:\ZND\projects\burnmon\cmd\burnmon-dev\page.html` (`assignSessionColor`,
    `reserveSessionShades`, `sessionColor`, `agentBySessionMap`, the `renderBurnPanels` call
    site, `renderBurnLegend`'s legend-item markup, `renderBars`' sort comparator)
  - `C:\ZND\projects\burnmon\internal\live\live.go` (new `buildChartAgents`, new
    `Snapshot.ChartAgents` field, wired into `BuildSnapshot`)
  - `C:\ZND\projects\burnmon\internal\live\live_test.go` (new
    `TestBuildSnapshot_ChartAgentsCoversSessionBeyondTurnCap`)
  - `C:\ZND\projects\burnmon\tools\uicheck\check_d15.go` (extended with a two-tick
    fake-mode scenario, a saturation-vs-vendor-strip check, a per-run-unique fake session id)
  - `C:\ZND\projects\burnmon\README.md`, `C:\ZND\projects\burnmon\STATUS.md`,
    `C:\ZND\projects\burnmon\SESSION_LOG.md` (version bump and full narrative)
- **Left out on purpose:** `C:\ZND\projects\burnmon\go.mod` (unrelated modification already
  present before this session, per Wilco's own instruction to leave it out) stays unstaged
  and uncommitted.
- **Untracked, pre-existing, not touched this session:**
  `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-28_ws2_alpha5_pushed_and_tagged.md`
  (a prior session's own brief, left as-is).

## 3. What did NOT happen (and why)
- **Not pushed, not tagged.** Committed locally only, per house rules; push/tag commands
  are printed for Wilco at the end of this session, his to run.
- **Not merged anywhere** (already on `main` directly, no branch involved).
- **No live-store numbers re-measured** this session (this was a targeted bug fix, not a
  performance pass); no CPU/RAM figures collected or claimed.
- **Wilco's own live `burnmon-dev.exe` (PID 35032) was closed mid-session**, at his
  explicit confirmation, so `uicheck` could get exclusive access to the single-instance
  mutex; it was left closed at the end of this session (his to relaunch).
- **The stray `04_assets\hub_agent_update_2026-09-28_ws2_alpha5_pushed_and_tagged.md`
  file was left untouched**, not read for content beyond its filename, not committed by
  this session.

## 4. Findings worth propagating
- **[RESULT]** Root cause confirmed by direct code reading, not just Wilco's own
  hypothesis: `sessionColor`/`assignSessionColor` (`page.html`) cache a session's colour
  into `sessionColorMap` on first call, unconditionally, even when `AGENT_COLOR[agent]`
  misses (agent unknown that tick) and the code fell back to `HARNESS_HUE.other`. That
  grey then never changes, because every caller only checks "already coloured", never
  "coloured with a real vendor".
- **[RESULT]** Found and confirmed why `agentBySessionMap` could lack a session's vendor
  in the first place: its old fallback, `internal/live`'s `Turns`, is capped at
  `turnTickerCap` (50) for the turn ticker's own display purposes, while `buildChart`
  (the chart's own data source) aggregates every turn in the 30-minute window uncapped. A
  busy window (more than 50 turns across all sessions, easy with one chatty session) can
  crowd a quieter session's turns out of that cap entirely. Same class of bug review
  already caught once for `BuildTurns` itself, 2026-09-24 - reappeared here for a second,
  independent consumer of the same capped `Turns` field.
- **[RESULT]** Fix applied and verified both ends: JS side stops caching/reserving a shade
  for an unknown vendor (retries every tick until known); Go side adds an uncapped
  `Snapshot.ChartAgents` map so the frontend always has a real answer for any session the
  chart names. New Go test and an extended live-window UI check (`uicheck d15`, WebView2,
  fake data only) both pass, reproducing the exact gap and its fix.
- **[RESULT]** Independent review (Claude Opus 5.5, fresh, read-only) confirmed the
  diagnosis and fix are correct, found no remaining path that caches a colour before the
  vendor is known, and caught one real Important issue in the new test itself (a fixed
  fake session id would collide across repeat `d15` runs against the same already-running
  window) plus two minor hardenings, all fixed and re-verified this session (`d15 d15`
  back to back against one window, both clean).
- **[RESULT]** Full verify pass green: `go vet ./...`, `go test ./... -count=1` (27
  packages), `.\build.ps1`, `node --check` on `page.html`'s script, `uicheck d0`-`d17`
  (one `d7` "fullscreen likely never engaged" failure on the full sweep, confirmed
  transient by an immediate solo re-run, clean - the same desktop-contention pattern
  earlier alpha sessions already documented, not a code regression).

## 5. Hub-level decision (if any)
Nothing. Project-internal bug fix, no cross-project time/park/pricing/CIPHER-wall call
involved.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` (hub one-pager): bump the referenced BurnMon
  Dev version to `v0.4.0-alpha.6` if it names a specific version anywhere.
- No DEADLINES, roadmap or portfolio change expected: this was an unplanned bug fix, not a
  roadmap item closing.

## 7. Open flags for next session
- Push and tag `v0.4.0-alpha.6` once Wilco is ready (commands handed to him at the end of
  this session, not run yet).
- Wilco's own `burnmon-dev.exe` was closed this session to free the uicheck mutex; he
  should relaunch it himself when he wants the live window back.
- Three findings the independent review flagged as pre-existing (not touched, not new
  regressions, no action needed unless they start mattering in practice):
  `buildChartAgents`' "last write wins" on a cross-vendor session_id collision (an
  already-known edge case elsewhere in this codebase's own tests); `agentBySessionMap`'s
  sessions-walk could in principle overwrite a good agent with an empty one if
  `Session.Agent` were ever empty (never observed); the `sessionColor` fallback path's own
  code comment understates how often it actually runs.

## 8. Related files
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-28_ws2_vendor_colour_families.md` (the
  original colour-family spec this bug's fix still honours).
- `C:\ZND\projects\burnmon\SESSION_LOG.md`, top entry, 2026-09-28 (fullest narrative).
- `C:\ZND\projects\burnmon\STATUS.md`, "BurnMon Dev" section, `v0.4.0-alpha.6` paragraph.
