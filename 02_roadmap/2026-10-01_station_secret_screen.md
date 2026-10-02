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

## Themes: construction site (default) and space station

Decided 2026-10-02 with Wilco. Two themes share one engine, one deck grid and one set of stage
rules; a theme only swaps room names, props, floors, sprites and the HUD wording.

- **T** toggles the theme while the Station is open, in either view. It lasts until the app
  restarts.
- `burnmon-dev.json` sets the startup theme: `"station_theme": "site"` (default when the key
  is absent or unknown) or `"space"`. Moved from `burnmon.json` (Wilco, 2026-10-02): that file
  is shared with `burnmon.exe`, and the Station only exists in `burnmon-dev.exe`. Not
  documented anywhere public, so the screen stays secret.
- HUD title and count: site theme `SITE` and "N agents on site"; space theme `STATION` and
  "N agents on board" (the space wording is already live in `station.js`).

| Stage | Space station | Construction site |
|---|---|---|
| reading | Archive | Drawing office (plans on the wall) |
| fetching | Uplink | Delivery gate |
| planning | Plan table | Site office meeting table |
| coding | Fabricator | Build zone (half-built walls, scaffolding) |
| running | Test chamber | Inspection and QA |
| thinking | Think pods | Foreman's hut |
| delegating | Briefing | Toolbox meeting |
| waiting | Lounge | Site canteen |
| compacting | Recycler | Skip container |
| dormant | Cryo bay | Parking area |
| arriving | Airlock | Site entrance with barrier |
| (core) | Reactor | Tower crane whose swing speed follows tok/min |

### Site art sources (licences checked 2026-10-02)

| Need | Source | Licence |
|---|---|---|
| Site office (containers), tanks | Kenney City Kit (Industrial) 2.0 | CC0 |
| Crane, cones, crates, warning signs, catwalks, screens | Kenney Factory Kit 3.0 | CC0 |
| Half-built walls, columns, stairs, barricades | Kenney Building Kit | CC0 |
| Wheel loader, trucks, van, cones | Kenney Car Kit | CC0 |
| Workers in hard hat and hi-vis vest (rigged, 24 animations) | Quaternius Ultimate Modular Men Pack, "Worker", via poly.pizza | CC0 |
| Excavator | "noway" on OpenGameArt, `excavator.blend` (2014, untextured, recolour) | CC0 |
| Scaffolding tower | "Scaffolding" by Marisha, poly.pizza `1_PM9UWLgAb` | CC-BY 3.0, credit required |
| Bulldozer, dump truck (optional) | Poly by Google via poly.pizza `ddxtaegI3HQ`, `1BpGYg14QGD` | CC-BY 3.0, credit required |

CC-BY items need a credit line in `tools\station_atlas\CREDITS.txt` and the About page;
everything else is CC0. Rejected: Jayclock's itch.io excavator (no formal licence, only "feel
free to use"), Meshy AI models (generated, style mismatch), Sketchfab and CGTrader paid or
CC-BY-NC models.

Pipeline: none of these kits ship isometric renders. Changed 2026-10-02 (alpha.10 build):
`tools\station_atlas\render_site.py` renders them in headless Blender 5.2 with the exact camera
`render_iso.py` already uses for the space sprites (elevation 30, azimuth 30, 512 x 512, 130 px
per tile), driven by a manifest `site_manifest.json`, instead of a three.js re-implementation
of that camera, so both atlases share one calibrated projection (`proj` must match). The
existing `build_atlas.py` packs them into `station_atlas_site.js` (`window.BM_STATION_ATLAS_SITE`).
Kits are downloaded into `tools\station_atlas\kits\` (gitignored). CC0 first: a CC-BY item is
used only where no CC0 piece does the job, and every one used is listed in `CREDITS.txt`.
The worker gets 8 directions times 2 walk frames from its walk animation, in three vest
colours (one per vendor family, as astronautA, astronautB and alien are in space). The O view
stays code-drawn pixel art, with new pixel sprites for containers, crane, excavator,
scaffolding, cones and workers.

### Theme engine contract (alpha.10)

- Site sprite names carry a `site_` prefix (workers `workerA|B|C_<dir8>_<0|1>`), so the
  pixel view's name patterns never match a space sprite by accident.
- Site rooms keep the space rooms' prop footprint tile for tile (same blocking tiles, same
  slots), so paths and slots are identical in both themes and a theme swap moves nobody.
- `BMStation.create({atlases: {space, site}, theme, onAgentClick})`, `st.setTheme(name)`,
  `st.theme()`. The page passes `window.BM_STATION_THEME` (spliced in by Go from
  `burnmon-dev.json`) as the startup theme; T calls `setTheme` while the Station is mounted.
- Tower crane (core): the mast is a sprite, the jib is drawn in code and swings at a speed
  that follows the smoothed tok/min, as the reactor's pulse does in space.

Built in `v0.4.0-alpha.10` (Claude Code on Windows, 2026-10-02); results, the excavator author
correction (Iacox 2022, CC0) and the open CPU climb are in `STATUS.md`'s alpha.10 entry.

## Light and dark (alpha.10, public)

Not part of the secret screen; documented in README. **L** toggles BurnMon Dev between its
dark theme (default) and a light theme, ignored in inputs and with Ctrl, Alt or Meta held,
like P and O. The choice is remembered across restarts in `burnmon-dev-view.json` next to
`burnmon-dev.db`, written by the app through the `bdevSetUITheme` binding (a per-machine view
preference, no config key; `burnmon-dev.json` stays the user's own file and is never
rewritten). Not localStorage: the page is loaded with NavigateToString, whose opaque origin
blocks it. At startup Go puts `data-theme="light"` into the `<html>` tag, so there is no
dark flash and no script. Light swaps the `:root` tokens and every
hard-coded colour the page and its canvases draw with; session colours stay as they are,
darkened only where they fail contrast on white. The Station keeps its own scene colours.

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
