# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-10-05 (Claude Code session on the laptop) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** Tells the hub that Station backlog item F (hover card too wide and jumping left) is built, verified and, by Wilco's own step, committed and pushed as `753af34`.
**Read order:** this file, then `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md` (item F at the end), then `C:\ZND\50_projects\burnmon\SESSION_LOG.md`.
**Supersedes:** nothing. Follows `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-10-05_station_glow_and_clouds.md`, whose "last lettered item is E" line is now out of date (F exists).

## 1. Headline
Station item F is shipped to `origin/main` in `753af34`. The hover card has a 320 px maximum width, wraps long values, and its width no longer depends on the pointer position. No version bump, no tag.

## 2. What changed on disk
- **Committed** in `C:\ZND\50_projects\burnmon`: `753af34` station: hover card max 320 px, long values wrap, width no longer follows the pointer. Made by Wilco after the work, not by this session. Checked live: `git rev-parse --short HEAD` gives `753af34` and `git ls-remote --heads origin main` gives `753af348814628b18f941e92f33014662ceb41be`, so it is pushed. No new tag; the newest tag on origin by version sort is `v0.4.0-alpha.10`.
- **Files in that commit** (the code part checked live with `git ls-files`; the full file list was not re-listed):
  - `C:\ZND\50_projects\burnmon\cmd\burnmon-dev\station\station.js`: `showTip` sets left to 0 before it measures the width; `.bms-tip` CSS gets `max-width:320px`, `box-sizing:border-box`, `overflow-wrap:anywhere`.
  - `C:\ZND\50_projects\burnmon\tools\uicheck\check_d28.go`: new check.
- **Untracked at the time of writing:** this brief.
- **Gitignored scratch:** screenshots `d28-<view>-<theme>.png` in `C:\ZND\50_projects\burnmon\testdata\uicheck\out\`.

## 3. What did NOT happen (and why)
- **No tag, no release, no version bump.** The exes were rebuilt locally only.
- **This session did not commit or push.** That was Wilco's step.
- **No `SESSION_LOG.md` paragraph and no status line for F in the roadmap file.** Section 6 asks the hub to record it.
- **No look at Wilco's own screenshots 17 to 20.** The failure was reproduced in a test window with fake sessions, not on his Cowork data.
- **Light page theme not covered by d28.** Only the site and space Station themes, as asked. The click card check d25 covers the light page.
- **No hanging indent for wrapped values.** Wrapped lines run back under the label; cosmetic, left alone.
- **`04_assets\2026-10-01_station_preview.html` still holds the old `showTip` and CSS.** It is an old preview copy and was not touched.

## 4. Findings worth propagating
- [RESULT] New check `d28`, run against the original code first, failed with 126 problems. P: card a constant 1243 px, over the limit. O: width grew 856 to 888 px (site) and 1036 to 1068 px (space) over 9 tiny pointer moves. I: 824 to 848 px (site) and 976 to 1008 px (space). In O and I the card also went left of the window, to x = -284 at worst. Source: `.\scripts\uicheck.ps1 d28` against `burnmon-dev.exe` built from the old code, 1600x900 window, fake sessions with a 200 character project, long model and long tool, no break opportunity.
- [RESULT] After the fix `d28` passes: every pointer step in P, O and I on the site and space themes reads exactly 320.0 px, nothing clipped, nothing outside the window or its Station panel.
- [RESULT] Verification: `go vet ./...` exit 0; `go test ./... -count=1` ok in every package; `.\build.ps1` ok; `.\scripts\uicheck.ps1 d28 d23 d25` all passed. `tools\uicheck` was vetted in its own module and was clean.
- [RESULT] Direction differs from the backlog's reading. The backlog said the width follows the left the card had last time. Measured: the width grew by 4 px for each 1 px the pointer moved right in O and I, and stayed constant in P. [STATE] Which of the two changes (left reset, max-width) matters most was not isolated; the test proves them together.
- [STATE] The d28 pointer is a synthetic `mousemove` on the canvas, not the real OS cursor. It reaches the same `onMove` handler, but a real mouse was not used. Wiggle range is limited to each agent's hit area (18x34 px at the smallest, in I), so it is a few px, not tens.

## 5. Hub-level decision (if any)
Nothing for `C:\ZND\10_holding\03_logs\decisions.md`. A project-internal, reversible UI fix.

## 6. What the next hub read should update
- `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`: add a status line that F is done and pushed in `753af34`.
- `C:\ZND\50_projects\burnmon\STATUS.md`: Station backlog F done; origin/main at `753af34`.
- `C:\ZND\50_projects\burnmon\SESSION_LOG.md`: top paragraph for this session.
- `C:\ZND\10_holding\01_projects\burnmon.md`: one line for the hub view.
- `C:\ZND\10_holding\02_roadmap\roadmap.md` and the week plan: no change expected.

Tracker rows moved: none.

## 7. Open flags for next session
- Wilco to hover a long-path Cowork agent in his own window and confirm the card looks right (320 px, wrapped).
- F is the last lettered item in the roadmap file; ask Wilco what comes next.
- The d28 run takes about 2 minutes (12 s wait per view for the fake agents to settle). `d23`, `d25` and `d28` need the window visible; a running `burnmon-dev.exe` blocks `uicheck-dev.ps1`.

## 8. Related files
- `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`
- `C:\ZND\50_projects\burnmon\tools\uicheck\check_d28.go`
- `C:\ZND\50_projects\burnmon\cmd\burnmon-dev\station\station.js`
- `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-10-05_station_glow_and_clouds.md`
