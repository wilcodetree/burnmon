# Station, the BurnMon Dev secret screen

Date: 2026-10-01. Target: `v0.4.0-alpha.9`. Decision to unpark for this one feature:
`C:\ZND\10_holding\03_logs\decisions.md`, entry 2026-10-01.

## What it is

An undocumented screen in `burnmon-dev.exe`. Every live session is an astronaut on an
orbital deck. The room it stands in is the stage the session is in now. Nothing in the
UI mentions it.

- **P** opens the full-window isometric Station. **P** or **Esc** closes it.
- **O** swaps the whole burn zone (chart, legend, session cards, vendor strip and turn
  ticker, everything above System; Wilco, 2026-10-01) for a pixel-art plan view of the same deck: flat 3/4
  top-down, in the style of pixel-agents and Pixel Office, drawn at 16 px per tile and
  scaled up with smoothing off. Walls with door gaps, patterned floors per room, furniture
  as pixel sprites (racks, desks with monitors, consoles, cryo pods, scanner arches,
  armchairs, plants), astronauts and aliens in the session colour, subagents as drones, and
  a speech bubble with a stage icon over each agent. **O** closes it and the burn zone comes
  back. **P** while O is open moves to the full view.
- Keys are ignored while focus is in an input, textarea, select or contenteditable, and
  when Ctrl, Alt or Meta is held.
- Drag pans, the wheel zooms, a double-click resets the camera. Hover shows a tooltip.
  A click calls the page's existing `openTurnPopup(sessionID, lastTurn)`.

## Stages and rooms

The deck is a 4 x 3 grid of 7 x 7 rooms with 1-tile corridors. Each room has a door in
the middle of every side that faces a corridor, and agents walk in and out only through doors. Rules live in
`internal/stage` (Go, tested); the first matching rule wins.

| Stage | Room | Rule |
|---|---|---|
| arriving | Airlock | session started less than 15 s ago |
| compacting | Recycler | a compaction finding in the last 60 s |
| dormant | Cryo bay | silent 10 min or more (the K2 idle cutoff) |
| reading | Archive | latest turn called Read, Grep, Glob, LS and similar |
| fetching | Uplink | WebFetch, WebSearch, any `mcp__` tool |
| planning | Plan table | TodoWrite, Task* tools, plan mode, Codex `update_plan` |
| coding | Fabricator | Edit, Write, MultiEdit, Codex `apply_patch`; also any unknown tool |
| running | Test chamber | Bash, PowerShell, Codex `exec` (the name the real store holds), `shell` / `exec_command` |
| delegating | Briefing | Task or Agent tool open; subagents walk as smaller astronauts on a tether |
| thinking | Think pods | the latest tool result is in and the session has been silent 8 s or more |
| waiting | Lounge | the latest turn called no tool, or AskUserQuestion, or a non-shell tool is open 3 min or more (most likely a permission prompt) |

The twelfth room, **Core**, is a reactor whose pulse follows the summed token rate of all
sessions (tok/min, smoothed). No agent is sent there.

Limits, stated plainly: the trails show tool calls and finished turns, not the model's
thought. Thinking is a silence gap, not an observed state. A fast tool shows its room for
about 8 s at most before Thinking takes over. Agents stay at least 3.5 s in a room before
they walk on, so a burst of tool calls shows as a walk, not a flicker.

"Latest turn" for a tool call: Claude keys a call by its turn's own `requestId`, so it is a
key match. Codex keys it by a rollout `turn_id` that never equals its event keys (ordinal or
byte offset) and writes a response's `token_count` only after that response's tool output,
so a call whose turn key matches no turn of the session counts as in the latest turn when it
is newer than the second-newest turn (`internal\live`, `inLatestTurn`).

## Art

Kenney Space Kit 2.0, CC0 (public domain), `C:\ZND\50_projects\burnmon\tools\station_atlas\KENNEY_SPACE_KIT_LICENSE.txt`.
100 sprites packed into one 256-colour atlas, inlined as `station_atlas.js` (132 KB).
Deck, glow, holograms, stars, planet and the plan view are drawn in code.

Camera: elevation 30 as in the kit, but the deck is turned 15 degrees (camera azimuth 30
instead of the kit's 45; Wilco, 2026-10-01: the left corner closer, the right corner
further, the station straighter). The long edges run about 16 degrees from horizontal
instead of 26.6. The kit's isometric PNGs only exist at azimuth 45, so the sprites are
re-rendered from its GLB models in headless Blender 5.2 (`tools\station_atlas\render_iso.py`).
Calibrated against the kit's own PNGs at its own view: yaw sign -1, offset 45 degrees,
anchor nudged 4.5 px, mean silhouette overlap (IoU) 0.906 on 20 samples. Pieces whose
footprint is not centred on their tile (walls, rails, dishes) are placed by a per-sprite
shift measured against the kit (`shifts_iso.py`); with it the walls go from IoU 0.22 to
0.985, every checked sprite within 1 px. `render_iso.py` writes the camera's tile axes on
screen to `proj.json`, `build_atlas.py` copies them into the atlas as `proj`, and
`station.js` projects the floor, fits the camera and depth-sorts from that matrix, so floor
and sprites cannot drift apart. Regenerate, from `tools\station_atlas`, with the kit unzipped:

    blender -b -P render_iso.py -- <kit> <r45> 30                     (kit view, for the shifts)
    python shifts_iso.py <kit> <r45> shifts.json
    set BMS_SHIFTS=shifts.json, BMS_AZIMUTH=30
    blender -b -P render_iso.py -- <kit> <r30> 30
    python build_atlas.py <kit> <r30>

`build_atlas.py <kit>` alone still packs the kit's own PNGs. Pixel-identical across
machines, not byte-identical: the PNG encoding differs by Pillow version (checked
2026-10-01, Pillow 12.2 on Windows against the Cowork build: same map, same pixels).
`calibrate_iso.py` and `make_sheet.py` are the calibration checks.

## What was built first (Cowork, 2026-10-01)

| File | State |
|---|---|
| `internal\stage\stage.go`, `stage_test.go` | built, `go vet` and `go test` green (Linux, Go 1.25.1) |
| `cmd\burnmon-dev\station\station.js` | built, `node --check` green, both views checked in headless Chromium at 1600x900 and 1920x1080 |
| `cmd\burnmon-dev\station\station_atlas.js` | generated |
| `cmd\burnmon-dev\station\wire_reference.js` | the P / O / Esc wiring used by the preview, reference for page.html |
| `tools\station_atlas\` | atlas builder, licence, preview builder and fake-session harness |
| `04_assets\2026-10-01_station_preview.html` | self-contained preview with 7 fake sessions, a guest that docks and leaves, two subagents |

Headless check of the preview: P opens, Esc closes, O replaces the chart, P from O swaps to
full, P closes; 75 s run with arrivals, departures and subagents, zero page errors.

## Steps 1 to 8 (built in v0.4.0-alpha.9, Claude Code on Windows, 2026-10-02)

All eight are built; results, deviations and known gaps are in `STATUS.md`'s alpha.9 entry.
Deviations from the text below, all deliberate: `ApplyStages` is a sibling of
`ApplySessionTotals`, called only while the Station is open (so a closed Station adds no store
read); Codex calls fall back to "newer than the second-newest turn" (their turn ids never match
event keys); O mounts into the whole `.zone.burn`, not `.burnchart.panel` (Wilco); the open
Station costs +8.7 points of one core, not under 6 (accepted by Wilco); d19 and d20 fail on a
single 1600x1000 screen for alpha.8 as well.

1. **Store:** `(*store.Store).LatestToolCalls(keys []SessionKey) (map[SessionKey]schema.ToolCall, error)`,
   newest call per session by `at`, with a test.
2. **Live:** `live.Session` gains `Stage string`, `StageSince string` (RFC3339) and `StageTool string`
   (json `stage`, `stage_since`, `stage_tool`). Fill them where `ApplySessionTotals` already
   walks sessions and subagents: build `stage.Input` from the session's start, last turn
   time, newest turn key, the latest tool call (`InLatestTurn` = call.Turn equals the newest
   turn's key, `Done` = `ResultBytes != nil`) and the newest compaction finding. One store
   call per tick, not one per session.
3. **Embed:** `//go:embed station/station.js station/station_atlas.js` in `main.go`; insert
   both as `<script>` blocks at a `<!--BM_STATION-->` marker in `page.html` before `SetHtml`.
4. **page.html:** port `wire_reference.js`. Full view mounts into a new fixed overlay
   `#stationOverlay` (z-index above `#turnPopupOverlay`). The plan view mounts into
   `.burnchart.panel`, hiding `#burnBars` and `#burnAxis` while open. Feed it from the
   existing tick: `station.update(BMStation.fromSnapshot(snap, sessionColor))`, only while
   mounted. Unmount on `visibilitychange` hidden and remount on visible, so a hidden window
   costs nothing. The existing Esc handler keeps priority for the turn popup.
5. **Click:** `onAgentClick(sid)` opens `openTurnPopup(sid, turn_count)`.
6. **uicheck:** new `d23`: press P, assert the overlay canvas exists and draws; Esc closes;
   O hides `#burnBars`; O restores it. Screenshot both views.
7. **Measure:** 10 minutes with the Station open, 10 with it closed. Closed must match
   alpha.8. Open target: under 6 percent of one core and under 60 MB extra JS heap.
8. Version `0.4.0-alpha.9`, `STATUS.md`, `SESSION_LOG.md`, a hub brief. No README mention;
   it is a secret screen.

## Done when

- P, O and Esc behave as above in the real window, and `uicheck d0` to `d23` pass.
- With real Claude Code and Codex sessions, each session's room matches its latest tool
  within one tick plus the 3.5 s dwell.
- Closed Station costs nothing measurable; open Station is inside the step 7 target.
