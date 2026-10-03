# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-10-03 (one Claude Code session on the laptop, Wilco present) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** tells the hub that Step 1 of the Station UI pass is committed locally and verified, and that nothing is pushed or tagged yet.
**Read order:** this file, then `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`, then the top entry of `C:\ZND\50_projects\burnmon\SESSION_LOG.md`.
**Supersedes:** nothing

## 1. Headline
Step 1 (the five Station UI items, plus a second round of site theme tweaks Wilco asked for in the same session) is committed locally as `2908574` on `main`. It is not pushed and not tagged. `burnmon-dev.exe` still reports v0.4.0-alpha.10, so no version moved.

## 2. What changed on disk
- **Committed** in `C:\ZND\50_projects\burnmon`: `2908574` "Station UI pass step 1: room names off props, one rover, uplink chair, seated site worker, site layout tweaks, brighter hologram". 9 files, 311 insertions, 50 deletions. Wilco ran the commit himself.
- **Files touched** (all inside that commit):
  - `C:\ZND\50_projects\burnmon\cmd\burnmon-dev\station\station.js` (room name layout, one rover, uplink chair, seated pose in both views, site layout mapping, `HOLO` brightness table, rug marks removed)
  - `C:\ZND\50_projects\burnmon\cmd\burnmon-dev\station\station_atlas_site.js` (regenerated, now 102 sprites)
  - `C:\ZND\50_projects\burnmon\tools\station_atlas\render_site.py` (new `pose` key for a hand-posed rig)
  - `C:\ZND\50_projects\burnmon\tools\station_atlas\site_manifest.json` (12 seated worker entries, `SE` added to `site_excavator` and `site_frame`)
  - `C:\ZND\50_projects\burnmon\tools\station_atlas\preview\check_themes.js` (the node check, new assertions)
  - `C:\ZND\50_projects\burnmon\tools\uicheck\check_d23.go` (O view in the space theme, 12 s settle wait, repaint wait before each screenshot, re-focus)
  - `C:\ZND\50_projects\burnmon\DEADLINES.md` (the 2026-10-24 Valona line removed, done earlier in the day)
  - `C:\ZND\50_projects\burnmon\SESSION_LOG.md`
  - `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md` (new, the step list)
- **Untracked** (so it is not lost): this brief, `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-10-03_station_ui_step1.md`.
- **Outside the repo, gitignored:** the Blender renders in `C:\ZND\50_projects\burnmon\tools\station_atlas\kits\_renders_site` (12 seated worker PNGs and 2 new direction PNGs). They are the source for the atlas rebuild and live only on this laptop.

## 3. What did NOT happen (and why)
- **Not pushed, not tagged.** Checked live: `git rev-parse` gives `2908574`, `git ls-remote origin refs/heads/main` gives `b8695d4`, no tag points at HEAD. Commits, tags and pushes are Wilco's step.
- **No version bump.** STATUS.md still names alpha.10 as current. Whether this becomes alpha.11 is Wilco's call.
- **STATUS.md was not updated** this session.
- **Steps 2 to 4 of the roadmap file were not started:** the Station CPU climb, `uicheck` d19 and d20 (known failing on the single 1600x1000 screen per STATUS.md, not re-run), the parked list, and the plan-limits panel brief.
- **The decision records Wilco still owes** (listed in the roadmap file) were not done by this session: `C:\ZND\10_holding\03_logs\decisions.md`, the hub one-pager, and striking assumption A1 and the Valona rows in `C:\ZND\50_projects\burnmon\02_roadmap\2026-09-22_burnmon_plan.md`.
- Wilco's own `burnmon-dev.exe` (it blocked `uicheck`) was stopped once, with his explicit yes, and left stopped.

## 4. Findings worth propagating
- [RESULT] Verification on the final code: `go vet ./...` exit 0, `go test ./... -count=1` exit 0, `.\build.ps1` exit 0, `node tools\station_atlas\preview\check_themes.js` all checks passed, `.\scripts\uicheck.ps1 d23 d24` both OK, and d21 and d22 OK in one earlier full run. Source: commands run in this session, output read.
- [RESULT] Room names: one shared `labelLayout` places all 24 names (12 rooms, 2 themes) on the bottom row, off every prop and plant of their room, at one common size (the smallest any room needs, at most 40 percent smaller). Ten rooms are centred, Lounge and Core sit right of centre. Proven from tile data in `check_themes.js`, with name widths measured with Pillow in Segoe UI Black.
- [RESULT] The "small orange box" in the Lounge was a second rover, `rovers[0]` with `area: 'lounge'`. It is removed; one corridor rover or loader remains.
- [RESULT] Item 4 answer: the shapes above the plan table are `holoTable()` in `station.js`, a deliberate hologram (translucent cone plus four rotating task cards). Wilco chose to keep it and make it brighter. The "L" shapes seen in the O view were marks drawn on the blueprint rug, now removed in both themes.
- [RESULT] Seated site worker: re-posed from the same CC0 Quaternius rig in headless Blender 5.2, 12 sprites (3 workers, 4 chair directions), plus a seated pixel worker in the plan view. Confirmed in the four screenshots in `C:\ZND\50_projects\burnmon\testdata\uicheck\out\`.
- [RESULT] Rebuilding the site atlas re-quantizes its palette: the 88 older sprites drift 0.24 of 255 on average (worst 25), with zero coverage changes. Measured against a copy of the previous file.
- [RESULT] Found along the way: agents stand at tile centres (slot plus 0.5); d23 screenshots used to be taken before the repaint, so two "different" theme shots were identical.
- [STATE] d23 uses real key presses and failed for lost window focus in 3 of about 9 launches this session. Passing runs were clean. Treat a single d23 failure as a re-run, not as a code fault.
- [STATE] Cosmetic and left alone: in the space theme the "E" of PLAN TABLE is partly hidden by the Test Chamber's top-row consoles. The old `site_scaffold` sprite stays in the atlas, unused.

## 5. Hub-level decision (if any)
Recorded by Wilco earlier on 2026-10-03 in the roadmap file; it is not new in this session. Paste-ready, if not already in the log:

  **Decision:** BurnMon has no Valona pilot. It is a ZeroNonsense.dev product only; developers use it through the public ZND GitHub.
  **Context:** the plan carried a Valona pilot assumption (A1) and a 2026-10-24 date. Wilco decided against it on 2026-10-03.
  **Alternatives considered:** keep a Valona pilot (rejected by Wilco).
  **Consequences:** the 2026-10-24 line is removed from `C:\ZND\50_projects\burnmon\DEADLINES.md` (done). Still open for Wilco: this entry in `C:\ZND\10_holding\03_logs\decisions.md`, the hub one-pager `C:\ZND\10_holding\01_projects\burnmon.md`, and striking A1 and the Valona rows in the plan file.
  **Links:** `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`

## 6. What the next hub read should update
- `C:\ZND\50_projects\burnmon\STATUS.md`: top entry should say Step 1 of the Station UI pass is committed as `2908574`, unpushed and untagged, version still alpha.10.
- `C:\ZND\10_holding\01_projects\burnmon.md` (hub one-pager): the Valona decision above, and the Station state.
- `C:\ZND\10_holding\03_logs\decisions.md`: the decision block in section 5, once Wilco confirms.
- `C:\ZND\50_projects\burnmon\DEADLINES.md`: already edited, check that no Valona date remains.
- Mission Deck This Week items: none named, because the deck was not read this session.

Tracker rows moved: none identified. `C:\ZND\10_holding\02_roadmap\roadmap.md` section 5 was not read in this session, so a hub reader should check whether Station work maps to a row.

## 7. Open flags for next session
- Wilco to decide: push `2908574`, tag, and whether this is alpha.11.
- Step 2 of the roadmap file: the Station CPU climb (cause open) and d19 and d20.
- The three Valona record edits listed in section 3.
- The clipped "E" in PLAN TABLE (space theme), if it bothers Wilco.
- `check_themes.js` is not part of `go test`; it runs by hand with node. Worth wiring in if it should gate commits.

## 8. Related files
- `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`
- `C:\ZND\50_projects\burnmon\SESSION_LOG.md` (top entry, 2026-10-03)
- `C:\ZND\50_projects\burnmon\STATUS.md`
- `C:\ZND\50_projects\burnmon\testdata\uicheck\out\d23-station-full.png`, `d23-station-space.png`, `d23-station-panel.png`, `d23-station-panel-space.png`
