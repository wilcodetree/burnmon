# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 (v0.2.3 window check patch session) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** Report the v0.2.3 patch: a real Win32+eval harness (`tools\uicheck`) replacing
the proxy checks v0.2.1 and v0.2.2 had to rely on, then eight fixes (W1 to W8) reproduced
and verified against the actual running window.
**Read order:** this file, `C:\ZND\projects\burnmon\STATUS.md`, `C:\ZND\projects\burnmon\SESSION_LOG.md`
**Supersedes:** nothing

## 1. Headline
W0 built the harness first, per the spec's own gate: no item counts as done on a proxy
check. The first approach tried (chromedp over WebView2's remote-debugging env var, then a
hand-rolled `ICoreWebView2EnvironmentOptions` COM object in a vendored `go-webview2` fork)
did not work on this laptop after several genuinely different attempts and was abandoned
per the debugging process rather than pushed further; the harness that shipped instead
drives the window with pure Win32 (`BitBlt` screenshots, `SendInput` clicks/keys) plus a
dev-only TCP channel the app itself opens (`BURNMON_UICHECK=1`, never set outside this
script) for the one thing an external process cannot do on its own: reading and evaluating
the page's own DOM/JS. W1 to W8 all reproduced first against the real window, then fixed,
then re-verified: startup blank/Not Responding, the turn drawer's Close button and overlay
click (both silently broken by an inline `onclick` attribute resolving in the wrong JS
scope), a horizontal scrollbar on long paths, duplicate/generic legend labels, legend
order, repeating axis ticks, skipped X-axis minute labels, and the cost axis not tracking
its series' visibility. `node --check`, `go vet`, `go test ./... -count=1`, `.\build.ps1`
and `scripts\uicheck.ps1` (all checks) all green. This session's own live Claude Code
activity provided the real store data every check ran against; no separate Codex CLI
session happened to be running concurrently. Committed locally, not pushed.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (local commit, not pushed): the new harness,
  all eight fixes, STATUS/roadmap/SESSION_LOG updates, this brief, version bump to `0.2.3`.
- **Files touched** (full paths):
  - `C:\ZND\projects\burnmon\tools\uicheck\*.go` (new): `main.go`/`checks.go` (registry and
    CLI entry), `win32.go` (window discovery incl. the ghost-window check, `BitBlt`
    screenshot, `SendInput` click/key, DPI awareness), `eval.go` (the TCP client to the
    dev eval channel, click-by-selector), `check_w0.go` through `check_w8.go` (one check
    per item).
  - `C:\ZND\projects\burnmon\cmd\burnmon\uicheck_devserver.go` (new): the dev-only
    localhost TCP eval server, `BURNMON_UICHECK=1` gated, bound into `main()`.
  - `C:\ZND\projects\burnmon\cmd\burnmon\main.go` (W1: `scan.DefaultSourcesWithOptions`,
    `codex.NativeSources`, `SeedNativeRoots`, `startLiveWatch`, the Hermes/Copilot CLI
    pollers moved off the pre-`w.Run()` path into the startup goroutine; the
    `startUICheckServer` call added; version to `0.2.3`).
  - `C:\ZND\projects\burnmon\internal\report\template.html` (W2: Close button/overlay
    click moved from inline `onclick` to `addEventListener`; W3: `table-layout:fixed` and
    `overflow-wrap:anywhere` scoped to `#turn_drawer`; W4: the "recent session" fallback
    label replaced with the session id's own first 8 characters; W5: `sessionIds` sorted
    by vendor/surface/model/project before building chart datasets; W6: `tokPrecise`, a
    two-decimal token formatter scoped to the Now chart's own axis/tooltip, `tok()`
    elsewhere unchanged; W7: `tickCallback` no longer skips any minute, rotation computed
    from px-per-minute; W8: the cost scale's `display:'auto'`).
  - `C:\ZND\projects\burnmon\scripts\uicheck.ps1` (new): starts burnmon.exe with the dev
    eval channel, waits for the real first Collect to finish (polls app.log, not a fixed
    sleep), runs the named checks (or all of them), stops the exe.
  - `C:\ZND\projects\burnmon\cmd\burnmon-cli\main.go` (version to `0.2.3`).
  - `C:\ZND\projects\burnmon\.gitignore` (`tools\uicheck\uicheck.exe`, `testdata\uicheck\out\`).
  - `C:\ZND\projects\burnmon\STATUS.md`, `C:\ZND\projects\burnmon\02_roadmap\roadmap.md`,
    `C:\ZND\projects\burnmon\SESSION_LOG.md` (v0.2.3 recorded as roadmap item 6, v0.3
    renumbered to item 7, decision to item 8).
  - `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-23_v0.2.3_window_check_patch.md`
    (this file).
- **Screenshots**: `C:\ZND\projects\burnmon\testdata\uicheck\out\` (git-ignored), one
  before/after pair per item where a genuinely separate broken build was still in hand
  (W1); before/after both against the already-fixed binary for W2 to W8, since each fix
  was isolated by direct DOM/measurement evidence first and a separate broken rebuild per
  item was not worth the round trip once that evidence existed.
- **On a branch, not merged:** none; all work is on `main`, committed locally.
- **Untracked / outside this session's scope:** `04_assets\_grill_state.md` and
  `DEADLINES.md` were already modified, and `02_roadmap\2026-09-23_v0.3_spec.md` already
  untracked, at session start from earlier work; left untouched and not committed by this
  session.

## 3. What did NOT happen (and why)
- Not pushed to `origin/main`. Tag/push commands handed to Wilco at the end of this
  session's own reply, per the session prompt's own instruction.
- The chromedp/CDP route (`WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port`)
  and a hand-rolled `ICoreWebView2EnvironmentOptions` COM object were both tried and both
  abandoned: the env var is silently dropped for `--remote-debugging-port` on this laptop's
  WebView2 runtime, and the COM object kept failing `CreateCoreWebView2EnvironmentWithOptions`
  with `E_INVALIDARG` after several distinct fix attempts (QueryInterface strictness, null
  vs empty-string optional properties), matching the debugging process's own "3+ fixes
  failed, question the approach" signal. Neither is in the shipped code; the vendored fork
  was fully reverted.
- No real screen capture is guaranteed reliable here regardless of code correctness: this
  laptop's elevated terminal (this very session, or a "modelwatch" monitor) sits on top of
  wherever the app window renders, and Windows' UIPI blocks a non-elevated process from
  outranking it in z-order. Screenshots are therefore best-effort (logged, not fatal) in
  every check from W2 onward; the pass/fail signal is always the real DOM/Chart.js state
  read back through the dev eval channel, not the screenshot.
- No separate live Codex CLI session ran during this pass, only this session's own Claude
  Code activity; the Done-when's "a Claude Code and a Codex session running" is half-covered.

## 4. Findings worth propagating
- [RESULT] W2's root cause: the whole Now-page script is one IIFE (opens near line 476,
  closes near line 2271); `closeTurnDrawer` is defined inside it, so the Close button's and
  overlay's inline `onclick="closeTurnDrawer()"` attributes (which resolve in global scope)
  silently threw `ReferenceError` on every real click. Escape already worked because its
  handler is registered via `addEventListener` from inside that same closure. Confirmed by
  dispatching a real `KeyboardEvent` (worked) versus `.click()` on the same button (failed)
  before the fix, both succeeding after.
- [RESULT] W4's root cause is a real window mismatch, not a template bug alone:
  `live.BuildSnapshot` drops a session from `Snapshot.Sessions` once its last turn is older
  than `runningWindow`, but the Now chart's own 30-minute window is longer, so a session
  that just went idle keeps chart bars with no matching `Sessions` entry. Two such sessions
  both hit the same generic fallback label and looked like duplicates.
- [RESULT] `go test ./... -count=1` green across every package; `node --check` on both
  extracted script blocks passed; `.\build.ps1` green; `scripts\uicheck.ps1` green for
  every check, W0 through W8.
- [STATE] Local commit exists on `main`, not pushed.

## 5. Hub-level decision (if any)
None needed this session. Every open item is project-internal, recorded in BurnMon's own
STATUS.md Known gaps, not a hub-level call.

## 6. What the next hub read should update
`10_holding\01_projects\burnmon.md` may want a one-line note that BurnMon now has a real
in-window UI check harness (`tools\uicheck`, `scripts\uicheck.ps1`), reusable by future
sessions instead of falling back to proxy checks; and that a `go-webview2` COM-options
fork was tried and abandoned this session, so a future session should not re-attempt the
same route without new information.

## 7. Open flags for next session
- Push and tag `v0.2.3` to `origin/main`, Wilco's call; commands below.
- A real live-Codex-CLI pass alongside a Claude Code session, since this session only had
  the latter running.
- `scripts\uicheck.ps1`'s Collect-wait is a 90-second poll against app.log; if a future
  session sees it time out, check machine load before assuming a regression (this laptop's
  Collect duration varied from ~2s to 60s+ purely from this session's own process churn).

## 8. Related files
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.2.3_window_check_patch.md` (this
  session's spec/prompt)
- `C:\ZND\projects\burnmon\STATUS.md`, `C:\ZND\projects\burnmon\SESSION_LOG.md`,
  `C:\ZND\projects\burnmon\02_roadmap\roadmap.md`
