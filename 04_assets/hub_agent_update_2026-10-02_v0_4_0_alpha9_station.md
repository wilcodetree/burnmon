# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-10-02 (session started 2026-10-01 evening, Claude Code on Windows) - **Owner:** Wilco de Tree
**Project:** BurnMon (BurnMon Dev)
**Purpose:** BurnMon Dev v0.4.0-alpha.9, the Station secret screen, is built and verified locally, with one accepted miss on the CPU target.
**Read order:** this file, `C:\ZND\50_projects\burnmon\STATUS.md` (the `v0.4.0-alpha.9` entry), `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-01_station_secret_screen.md`
**Supersedes:** nothing (follows `hub_agent_update_2026-09-29_v0_4_0_alpha8_codex_live_tail.md`)

## 1. Headline
v0.4.0-alpha.9 (the Station, spec steps 1 to 8) is built and tested on `main` in the working tree, not committed. The open Station costs +8.7 percent of one core against a target of 6; Wilco accepted that for alpha.9.

## 2. What changed on disk
- **Committed:** nothing. Wilco commits by hand (no git writes on `C:\ZND`).
- **Modified:** `C:\ZND\50_projects\burnmon\internal\store\store.go`, `store_test.go`; `C:\ZND\50_projects\burnmon\internal\live\live.go`; `C:\ZND\50_projects\burnmon\internal\schema\event.go`; `C:\ZND\50_projects\burnmon\internal\adapter\claude\claude.go`; `C:\ZND\50_projects\burnmon\internal\adapter\codex\codex.go`; `C:\ZND\50_projects\burnmon\cmd\burnmon-dev\main.go`, `history_delta.go`, `page.html`, `app.go` (version); `C:\ZND\50_projects\burnmon\STATUS.md`, `SESSION_LOG.md`.
- **New (untracked):** `C:\ZND\50_projects\burnmon\internal\live\stage_test.go`; `C:\ZND\50_projects\burnmon\cmd\burnmon-dev\station_embed_test.go`; `C:\ZND\50_projects\burnmon\internal\adapter\claude\lateresult_test.go`; `C:\ZND\50_projects\burnmon\internal\adapter\codex\lateresult_test.go`; `C:\ZND\50_projects\burnmon\tools\uicheck\check_d23.go`; `C:\ZND\50_projects\burnmon\tools\station_atlas\render_iso.py`, `shifts_iso.py`, `calibrate_iso.py`, `make_sheet.py`; this brief.
- **Untracked from the Cowork session, changed here:** `C:\ZND\50_projects\burnmon\internal\stage\stage.go` (one word: `exec`) and `stage_test.go`; `C:\ZND\50_projects\burnmon\cmd\burnmon-dev\station\station.js` (projection from the atlas, baked props, paced frames, a duplicate-loop fix) and `station_atlas.js` (regenerated); `C:\ZND\50_projects\burnmon\tools\station_atlas\build_atlas.py`; `C:\ZND\50_projects\burnmon\04_assets\2026-10-01_station_preview.html` (regenerated); the spec.
- **Outside the repo:** Blender 5.2.1 installed on the laptop with winget (`C:\Program Files\Blender Foundation\Blender 5.2`), needed to re-render the sprites. The Kenney kit zip and all renders live only in the session scratchpad; they regenerate from the spec's Art section.

## 3. What did NOT happen (and why)
- Not committed, not pushed, not tagged: Wilco's manual step.
- The CPU target (open Station under 6 percent of one core) is NOT met: +8.7. Wilco chose to accept it over a lower frame rate or a softer canvas.
- uicheck d19 and d20 do not pass on this laptop today: its only screen is 1600x1000 at 200 percent, and the alpha.8 exe fails them the same way. Not re-run on Wilco's usual monitors.
- `burnmon.exe` stays `0.3.2` although it shares the late-tool-result ingest fix and was rebuilt. No release note for it.
- No README mention: the Station is a secret screen by design.
- No secret was handled.

## 4. Findings worth propagating
- [RESULT] Final 10-minute samples, same exe, real key presses counted (none): Station closed, Go 3.18 and WebView2 4.84 percent of one core, JS heap 9.5 MB; open, Go 3.28 and WebView2 13.49, JS heap 9.5 to 28.0 MB. Open costs +8.7 points CPU (target 6, missed, accepted) and +18.5 MB heap (target 60, met).
- [RESULT] The first open measurement cost about +44 points. Profiling showed JavaScript at about 4 percent; the cost tracked how often an animation frame was requested (62 a second for a 30 fps loop). Timer-paced frames (15 fps moving, 8 idle), baked static props and a fixed duplicate-loop bug brought it to +8.7.
- [RESULT] Live rooms check, a stage sample every 2.5 s against the store's newest call: this Claude Code session went Read and Grep to Reading, Bash to Running, Thinking 8 s after the result, Write to Coding, an MCP search to Fetching, then Waiting; a real `codex exec` task went Arriving, Running, Thinking, Waiting. Every change landed within one sample.
- [RESULT] Two real ingest bugs found from the store and fixed with tests: Codex shell calls are stored as `exec` and showed as Coding; tool results read in a later pass were dropped (1,812 of 3,805 Bash calls had none), now applied as result-only updates.
- [RESULT] `go vet ./...` and `go test ./... -count=1` green; `node --check` green on all three page scripts; `.\build.ps1` green; uicheck d0 to d18 and d21 to d23 pass (d23 new).
- [STATE] Closed against alpha.8: alpha.8 measured 22.4 percent (Go plus WebView2) in one 10-minute run, alpha.9 closed 13.1 and 8.0 in two later runs, the same 9.5 MB heap; run-to-run spread from live sessions exceeds any difference, so the claim is "no worse", not "faster".
- [STATE] Wilco changed the spec twice in-session: O replaces the whole burn zone, and the full view turns the deck 15 degrees (azimuth 30, elevation 30). Both are in the spec now.

## 5. Hub-level decision (if any)
Nothing new. The unpark for this one feature is already `C:\ZND\10_holding\03_logs\decisions.md` 2026-10-01; accepting +8.7 percent CPU is a project-internal call, recorded in `C:\ZND\50_projects\burnmon\STATUS.md`.

## 6. What the next hub read should update
`C:\ZND\10_holding\01_projects\burnmon.md` (version alpha.9, Station built, not committed), the BurnMon line in `C:\ZND\10_holding\02_roadmap\roadmap.md` section 1 (still parked to 1 Nov apart from this feature), `C:\ZND\wiki\hot.md`. No Mission Deck This Week item covers the Station.

Tracker rows moved: none (the Station has no row in `C:\ZND\10_holding\02_roadmap\2026-09-29_golive_tracker.md`).

## 7. Open flags for next session
- Commit, push and tag alpha.9 (Wilco).
- Re-run uicheck d19 and d20 on the usual monitors.
- If the CPU miss ever matters: 12 and 6 fps, or a 1.5x canvas cap, are the next levers (estimates, unmeasured).
- Known gaps listed in STATUS: an `ApplyStages` error only logs; a one-tick lounge flicker is possible; a Codex `exec` awaiting approval reads Running.
- Pre-existing, not touched: `C:\ZND\50_projects\burnmon\SESSION_LOG.md`'s alpha.8 entry has a mangled path (`02_roadmap6-09-29...`), and the v0.3.2 header at the top of `STATUS.md` is still stale.

## 8. Related files
- `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-01_station_secret_screen.md`
- `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-01_station_session_prompt.md`
- `C:\ZND\50_projects\burnmon\04_assets\2026-10-01_station_preview.html`
- `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-09-29_v0_4_0_alpha8_codex_live_tail.md`
- `C:\ZND\10_holding\03_logs\decisions.md` (2026-10-01, BurnMon unparked for the Station)
