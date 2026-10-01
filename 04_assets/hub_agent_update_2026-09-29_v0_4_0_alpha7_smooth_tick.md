# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-29 - **Owner:** Wilco de Tree
**Project:** BurnMon (BurnMon Dev, WS2)
**Purpose:** BurnMon Dev v0.4.0-alpha.7 is code-complete and verified locally: the page freeze has two named causes, both fixed, and the System chart gap cause is narrowed and moved off the sample path.
**Read order:** this file, `C:\ZND\50_projects\burnmon\STATUS.md` (the `v0.4.0-alpha.7` entry), `C:\ZND\50_projects\burnmon\02_roadmap\2026-09-29_ws2_smooth_tick_and_system_layout.md`
**Supersedes:** nothing (follows `hub_agent_update_2026-09-28_v0.4.0_alpha6_pushed_and_tagged.md`)

## 1. Headline
v0.4.0-alpha.7 (spec items 1 to 5) is built and tested on `main` in the working tree, not committed. Before and after runs on the real store meet all three spec proof targets in the final 92 minute verification run; the first 90 minute after run missed two of them and led to a second freeze fix.

## 2. What changed on disk
- **Committed:** nothing. Wilco commits by hand (no git writes on `C:\ZND`).
- **Modified:** `C:\ZND\50_projects\burnmon\cmd\burnmon-dev\main.go`, `app.go`, `page.html`; `C:\ZND\50_projects\burnmon\cmd\burnmon\main.go` (one line, WebView2 folder); `C:\ZND\50_projects\burnmon\internal\sysmon\sample_windows.go`, `sample_stub.go`, `types.go`; `C:\ZND\50_projects\burnmon\internal\watch\watch_windows.go`; `C:\ZND\50_projects\burnmon\README.md`; `C:\ZND\50_projects\burnmon\STATUS.md`.
- **New:** `C:\ZND\50_projects\burnmon\cmd\burnmon-dev\history_delta.go`, `history_delta_test.go`, `perf.go`; `C:\ZND\50_projects\burnmon\tools\uicheck\check_d18.go` to `check_d22.go`; screenshots in `C:\ZND\50_projects\burnmon\04_assets\reference\2026-09-29_smooth_tick\`; this brief.
- **Untracked, not mine:** `C:\ZND\50_projects\burnmon\02_roadmap\2026-09-29_ws2_smooth_tick_and_system_layout.md` (the spec) and Wilco's `2026-09-29_system_boxes_mockup.png`.

## 3. What did NOT happen (and why)
- Not committed, not pushed, not tagged: Wilco's manual step.
- The final exe (with the review fixes, the `WEBVIEW2_USER_DATA_FOLDER` line and item 5) has no long real-store run of its own. Its proof is `go vet ./...`, `go test ./... -count=1` and uicheck `d0` to `d22`, all green. The 92 minute verification run used the build just before those changes.
- The 2026-09-29 morning System stall (up to 2 min 56 s) did not recur in 182 minutes of instrumented runs, so its exact blocking call is inferred, not observed.
- Six garbled WebView2 profile folders in the repo root were moved to the Recycle Bin, not hard-deleted. Their names held prompt and path fragments from process memory. No secret was seen or written.

## 4. Findings worth propagating
- [RESULT] Freeze cause 1, payload growth. Before (62 min, pre-alpha.7 build): snapshot JSON grew to 4.38 MB/tick, round trip p50 148 ms (worst minute p95 352, max 466), WebView2 renderer 2,970 MB and JS heap 2,356 MB at minute 62. After (deltas): JSON 36 to 58 KB/tick, flat. Snapshot build 1 to 10 ms throughout, so no burn cache.
- [RESULT] Freeze cause 2, lost answer. A Win32 modal loop drops go-webview2's `PostThreadMessage` wakeup, and one answer waits for the next `Dispatch`. Measured: 67 s once in the first 90 min after run, 36 s on a forced modal move loop. Before alpha.7 nothing else dispatched, so it froze for good, matching Wilco's 2026-09-28 freeze. With the 1 s heartbeat plus watchdog, the same forced loop gives a 2-tick streak.
- [RESULT] Final verification run (92 min): p95 round trip worst 23 ms (17 ms outside the forced repro), no streak over 2 ticks, renderer 223 MB at minute 62 and 223 MB at minute 92 (range 223 to 230), JS heap at most 24.8 MB, 0 slow sample passes, 0 gaps over 25 s.
- [RESULT] System gaps: the process was awake, the sample loop was stalled (persisted rows prove it); the logged slow part was `netsh wlan show interfaces` (2.1 s, 2.65 s). The drive and wifi refresh now runs on its own goroutine.
- [RESULT] Layout: chart 70% with To Do on (0.700 to 0.701 at three sizes), To Do fills the bottom; harness labels in full; System boxes on one row grid matching the mock-up.
- [STATE] Shared-code fixes worth knowing hub-wide: an `internal\watch` Close race (a panic at shutdown, intermittent) and a go-webview2 use-after-free on `DataPath`. Both are worked around in this repo; neither is reported upstream.

## 5. Hub-level decision (if any)
Nothing. All calls here are project-internal.

## 6. What the next hub read should update
`C:\ZND\10_holding\01_projects\burnmon.md` (current version and state), the hub STATUS or portfolio line for BurnMon, `C:\ZND\wiki\hot.md`. No Mission Deck This Week item is named in the spec; none is claimed here.

## 7. Open flags for next session
- A long real-store run of the final alpha.7 exe, to confirm nothing regressed.
- `C:\ZND\50_projects\burnmon\STATUS.md` still opens with "What is true at this commit (2026-09-26): v0.3.2"; that header is stale against the BurnMon Dev section.
- uicheck overwrites `%LOCALAPPDATA%\burnmon\burnmon-dev-window.json` (it restores at (100,100) after a run): pre-existing, not fixed.
- The d19 and d20 screenshots show Wilco's real Wi-Fi SSID in the Network box.
- Consider reporting the `PostThreadMessage` and `DataPath` issues to go-webview2 upstream.

## 8. Related files
- `C:\ZND\50_projects\burnmon\02_roadmap\2026-09-29_ws2_smooth_tick_and_system_layout.md`
- `C:\ZND\50_projects\burnmon\04_assets\reference\2026-09-29_smooth_tick\` (incl. `2026-09-29_d22-system-boxes_vs_mockup.png`)
- `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-09-28_v0.4.0_alpha6_pushed_and_tagged.md`
