# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-25 - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** report the UI review patch pass over BurnMon Dev (WS2): 7 commits on `burnmon-dev`, not pushed, plus the exact push commands for Wilco to run.
**Read order:** this file, then `C:\ZND\projects\burnmon\02_roadmap\2026-09-25_ws2_ui_review_patch.md`, then `C:\ZND\projects\burnmon\04_assets\2026-09-24_burnmon_dev_design.md` section 11.
**Supersedes:** nothing (first brief for this patch pass)

## 1. Headline

All 11 sections of Wilco de Tree's 2026-09-25 UI review patch are built, reviewed and committed on branch `burnmon-dev` in `C:\ZND\projects\burnmon` (code complete on branch, not merged, not pushed). Startup time dropped sharply as part of section 11: window creation from 18.67s to 0.80s, first full render from 19.08s to 1.03s, measured against the real local store both times.

## 2. What changed on disk

Committed in `C:\ZND\projects\burnmon` (worktree `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev`), branch `burnmon-dev`, 7 commits on top of the already shipped v0.4 WS2 phase 0-4 work:

- `7194f2f` - sections 1-8: top bar, burn chart (per-session stacked bars, CPU line overlay removed), vendor strip totals row, activity plus harness-heatmap split row, turn ticker with a click-through popup, a perfadvisor-style system panel, process-groups column tweaks, the "Today's Read" advisor panel deleted (its UI binding only; the advisor rule engine itself is unchanged and still runs inside the export bundle). Also includes section 11 step 1's startup-timing instrumentation, bundled in since it touched the same files.
- `22c9e1a` - section 10: window size, position, maximized state and monitor now persist across launches; F11 toggles fullscreen and Esc leaves it (hand-rolled Win32, since go-webview2 exposes no placement API and WebView2/Chromium reserves F11 as a browser accelerator key); responsive CSS breakpoints at 900px and 1600px replace the never-finished two-viewport/tabs plan from the original design.
- `b3be343` - section 9: an optional Microsoft To Do panel, off by default, ported from `C:\ZND\projects\perfadvisor\internal\todo\todo.go` with its own token cache and a StartLogin/FinishLogin split so sign-in never blocks the UI thread.
- `7b63593` - a design-note-only commit: a section 11 addendum to `C:\ZND\projects\burnmon\04_assets\2026-09-24_burnmon_dev_design.md` covering the whole patch and correcting the now-superseded phase-1 viewport plan.
- `e5cf3f7` - section 11 (rest): the native-root filesystem scan and the startup backfill moved from blocking window creation to a background goroutine started after the window and its own loading screen already exist. This is the change behind the startup numbers in section 1 above.
- `1d7d3a6` - the matching `SESSION_LOG.md` entry.
- `e9a7324` - removed one stray em dash a review missed in a code comment (`tools\uicheck\check_d7.go`).

Files touched span `cmd\burnmon-dev\*.go`, `cmd\burnmon-dev\page.html`, `internal\sysmon\*.go` (new `Sample.SwapUsedMB`, `Disks`, `Wifi` fields, `BaseClockGHz()`, ported from perfadvisor source commit `main 2ed8046`), `internal\live\live.go` (new `TurnDetail.Cost` field), `internal\todo\todo.go` (new package), `internal\advisor\advisor.go` (comment only), `tools\uicheck\*.go` (new checks d4 through d7, `check_d0.go` rewritten, `win32.go`'s `bringToFront` hardened), and `C:\ZND\projects\burnmon\04_assets\2026-09-24_burnmon_dev_design.md`.

Untracked, not committed: `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev\04_assets\reference\2026-09-25_ui_review\` (four reference screenshots Wilco copied into the worktree for this task). Left for Wilco to decide whether to commit.

## 3. What did NOT happen (and why)

- Not pushed, not tagged, not merged. All 7 commits sit on local `burnmon-dev` only. Exact push command is at the end of this brief.
- Section 9's Microsoft To Do panel was not exercised end to end with a real sign-in. Verified the sign-in button, the device-code display, and the stuck-panel-after-failure fix (simulated a failed login via the dev eval channel), but deliberately did not complete a real OAuth flow against Wilco's own Microsoft account without him asking for it. The task-list rendering itself is therefore unverified against real data.
- `uicheck` check `d7` (the real F11 keypress test) did not run in the final consolidated verification pass. It was independently verified working, with real screenshots at genuine fullscreen, earlier in the same session, but the workstation locked partway through the session and Windows correctly refused synthetic input (`SendInput` returned access denied) for the rest of it. Checks `d0` through `d6` did all pass together in that final pass.
- This session's own screen resolution was not stable. It measured 3440x1440 at some points and roughly 1600x1000 at others during the same session (most likely tied to Wilco's own remote-viewer window, not something this session controlled). Every viewport check logs plainly when it could not grant the exact requested size rather than silently asserting a fit it did not really test; the five required sizes (1280x860, 1024x1152, 1152x2048, 1920x1080, 2560x1440) were each confirmed fitting with no scroll at least once this session, at whichever real size was actually available at that moment.
- No shared or ingest code on `main` was touched. The known shared-store memory and handle growth bug (design doc, "a BurnMon bug on main") is explicitly out of this patch's scope and was left alone.
- No secret was pasted or persisted. No OAuth code, token or consent URL from the (unattempted) Microsoft To Do sign-in exists anywhere in this session's output.

## 4. Findings worth propagating

- [RESULT] Startup time, measured against the real local store, before this session touched anything vs. after all fixes: window created 18.67s to 0.80s; first full render 19.08s to 1.03s. Watcher and pollers started, and backfill done, kept roughly the same absolute duration (about 14s and 23s) but now run in the background instead of blocking the window.
- [RESULT] Four fresh read-only Opus code reviews this session (one per major commit) caught real bugs, not style nits, all fixed before the matching commit: a `WINDOWPLACEMENT` struct with an extra field only valid on the old Mac Win32 port (silently failed every window-state save and restore); `MonitorFromPoint` called with two arguments instead of one packed `POINT` (silently found no monitor, ever); an F11 hotkey that would have hijacked F11 from every other running app system-wide; a stale-response race in the new turn-detail popup; `disk.Partitions`/`disk.Usage` and `netsh` running on every 2-second sample tick instead of a slower cadence (risk of blocking a whole tick against an unreachable mapped network drive); a Microsoft To Do sign-in that could get permanently stuck with no way to retry after one failed attempt; a token-cache file race between goroutines; and a per-file backfill progress callback that could have pushed thousands of UI-thread round trips against a large trail.
- [STATE] Section 9 (Microsoft To Do) is code complete but its live task-rendering path has not been exercised against real Microsoft Graph data. Pending a real sign-in, which Wilco needs to do himself (see section 3).
- [STATE] `uicheck` check `d7` needs one more clean run (workstation unlocked, stable screen resolution) to re-confirm the real-F11 path in the same pass as `d0` through `d6`, though it was already proven correct in an earlier pass this same session.

## 5. Hub-level decision

Nothing. This is a project-internal UI/UX and startup-performance pass; no cross-project time, park or unpark, consultancy, positioning, pricing, voice or CIPHER-anonymity call was made.

## 6. What the next hub read should update

- `C:\ZND\10_holding\01_projects\burnmon.md` (the hub one-pager): note the UI review patch is code complete on `burnmon-dev`, awaiting push and Wilco's own review and merge decision.
- `C:\ZND\10_holding\02_roadmap\roadmap.md` and `C:\ZND\10_holding\01_projects\portfolio.md` if BurnMon's own milestone language there still describes the pre-patch v0.4 WS2 state.
- No `DEADLINES.md` entry implied by this brief; nothing here is time-boxed.

## 7. Open flags for next session

- Push `burnmon-dev` to `origin` (never the `mirror` remote, per this project's own remote policy) once Wilco has reviewed the diff. Exact command at the end of this brief.
- Complete a real Microsoft To Do sign-in to verify section 9's task-list rendering against live data.
- Re-run `uicheck` check `d7` once the workstation is unlocked and the screen resolution is stable, to close out the one check that could not complete in the final pass.
- Decide whether to commit `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev\04_assets\reference\2026-09-25_ui_review\` (currently untracked).
- Consider tightening `scripts\uicheck-dev.ps1`'s own wait budgets (currently up to 120s for the eval port, 90s for backfill): both are now far more generous than the app's own actual startup time warrants, though neither is incorrect.

## 8. Related files

- `C:\ZND\projects\burnmon\02_roadmap\2026-09-25_ws2_ui_review_patch.md` (the patch spec this whole session followed).
- `C:\ZND\projects\burnmon\04_assets\2026-09-24_burnmon_dev_design.md` (design note, section 11 addendum added this session).
- `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry, 2026-09-25).
- `C:\ZND\projects\burnmon\04_assets\reference\2026-09-25_ui_review\` (Wilco's own reference screenshots for this patch; untracked).
- Screenshots from this session's own verification, local only, gitignored, not committed: `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev\testdata\uicheck\out\`.

---

## Push command (not run this session, Wilco's own call)

PowerShell, working directory `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev`:

    cd C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev
    git push -u origin burnmon-dev

Pushes to `origin` (`https://github.com/wilcodetree/burnmon.git`) only. Do not push or tag to the `mirror` remote (`ssh://cipher/~/znd-mirrors/burnmon.git`), per this project's own remote policy. Not merged into `main`; that is a separate decision once Wilco has reviewed the diff.
