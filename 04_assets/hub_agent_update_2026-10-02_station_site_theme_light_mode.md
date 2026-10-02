# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-10-02 (Claude Code on Windows, one session) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** BurnMon Dev `v0.4.0-alpha.10` is built and verified in the working tree, not committed: the Station's construction site theme plus a public light theme.
**Read order:** this file, `C:\ZND\50_projects\burnmon\STATUS.md` (alpha.10 entry), `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-01_station_secret_screen.md`
**Supersedes:** nothing

## 1. Headline
`v0.4.0-alpha.10` is code-complete and verified on `main`'s working tree, uncommitted. It adds a
construction site theme to the Station (now the default, T toggles it) and an L key that toggles
BurnMon Dev between dark and light. Wilco commits, pushes and tags.

## 2. What changed on disk
- **Committed:** nothing this session (`HEAD` is still `a15cb71`, alpha.9 docs).
- **Files touched** (all under `C:\ZND\50_projects\burnmon\`): `cmd\burnmon-dev\station\station.js`,
  `cmd\burnmon-dev\station\wire_reference.js`, `cmd\burnmon-dev\page.html`, `cmd\burnmon-dev\main.go`,
  `cmd\burnmon-dev\app.go`, `cmd\burnmon-dev\station_embed_test.go`, `winres\burnmon-dev.json`,
  `tools\uicheck\check_d23.go`, `tools\station_atlas\build_atlas.py`, `tools\station_atlas\preview\*`,
  `04_assets\2026-10-01_station_preview.html`, `README.md`, `STATUS.md`, `SESSION_LOG.md`, `.gitignore`,
  `02_roadmap\2026-10-01_station_secret_screen.md`.
- **New files:** `cmd\burnmon-dev\station\station_atlas_site.js`, `cmd\burnmon-dev\viewstate.go`,
  `cmd\burnmon-dev\viewstate_test.go`, `cmd\burnmon-dev\app_test.go`, `tools\uicheck\check_d24.go`,
  `tools\station_atlas\render_site.py`, `site_manifest.json`, `site_proc.py`, `site_sheet.py`,
  `CREDITS.txt`, four Kenney licence files, `tools\station_atlas\preview\check_themes.js`, and
  screenshots `04_assets\2026-10-02_station_*.png`.
- **Untracked, outside git on purpose:** the downloaded kits in `C:\ZND\50_projects\burnmon\tools\station_atlas\kits\` (gitignored, re-downloadable).
- **Stray:** `C:\ZND\50_projects\burnmon\burnmon-dev.exe~` (left by `.\build.ps1` while an old instance ran; not ignored, not to be committed).

## 3. What did NOT happen (and why)
- Not committed, not pushed, not tagged: house rule, Wilco runs git on `C:\ZND`.
- `burnmon.exe` not touched; no shared ingest code changed.
- The cause of the open-Station CPU climb (section 4) is not found; Wilco chose to ship and investigate later.
- uicheck d19 and d20 not fixed: they fail on this laptop's single 1600x1000 screen, as for alpha.9.
- Codex Running was not observed in the live check (its calls fell in its Arriving window).
- No secrets handled.

## 4. Findings worth propagating
- [RESULT] Verification: `go vet ./...`, `go test ./... -count=1`, `.\build.ps1` green; uicheck d0 to d18 and d21 to d24 pass (d24 new, d23 extended with T).
- [RESULT] Space theme pixel-identical to alpha.9 (headless diff, frozen clock), apart from the "on board" wording already in the tree.
- [RESULT] Site art: 88 sprites, 152,614 bytes, all CC0 or procedural, same calibrated Blender camera as the space atlas (`proj` identical). No CC-BY item, so no public credit line needed. Spec correction: the excavator's author is Iacox (2022), not "noway".
- [RESULT] L persistence: saved to `burnmon-dev-view.json`, survives a restart (checked live).
- [RESULT] Cost, 10 + 10 minutes: closed Go 5.70 and WebView2 4.26 percent of one core; open (site) Go 10.98 and WebView2 16.32.
- [RESULT] New bug, also in alpha.9: with the Station open, CPU climbs with uptime (alpha.9 14.8, 25.7, 37.1; alpha.10 24.1, 44.6, 63.9 percent of one core per 5-minute window). Closed does not climb. alpha.9's accepted "+8.7 open" was a single 10-minute sample and missed it.
- [STATE] Next step for the climb (proposed, not started): split the WebView2 total per process (browser, GPU, renderer) over 30 minutes, then the same with drawing disabled but the canvas mounted, then a WPR trace of whichever process climbs.

## 5. Hub-level decision (if any)
Paste-ready, because the unpark of 2026-10-01 covered "one feature only":

  **Decision:** The BurnMon Station unpark (2026-10-01) also covers the Station's site theme (`v0.4.0-alpha.10`, part of the same spec) and one unrelated public addition, a dark and light toggle in BurnMon Dev (key L), which Wilco asked for during the session on 2026-10-02. The park otherwise stands until 1 Nov.
  **Context:** alpha.9 had already shipped, so the session moved to the spec's next item; Wilco added the light toggle mid-session.
  **Alternatives considered:** stop after alpha.9 (no work this session); build only the theme and leave the light toggle for 1 Nov.
  **Consequences:** One more Claude Code session on BurnMon during the Siteoffice go-live fortnight. The open-Station CPU climb follow-up is not scheduled; it waits for 1 Nov unless Wilco says otherwise.
  **Links:** `C:\ZND\50_projects\burnmon\STATUS.md` (alpha.10 entry), `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-01_station_secret_screen.md`.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` and `roadmap.md` section 1: latest BurnMon Dev is alpha.10 (once Wilco commits and tags), plus the open CPU climb.
- `C:\ZND\10_holding\03_logs\decisions.md`: the section 5 block, if Wilco agrees.
- `wiki\hot.md`, `wiki\log.md`.
- DEADLINES: nothing.

Tracker rows moved: none (BurnMon rows BM2 and BM3 stay parked to 1 Nov; this session is not a planned row).

## 7. Open flags for next session
- Commit, push, tag `v0.4.0-alpha.10` (Wilco).
- Open-Station CPU climb, cause unknown, next step in section 4.
- d19 and d20 need a larger screen to pass.
- `C:\ZND\50_projects\burnmon\SESSION_LOG.md`'s alpha.8 entry carries a mangled path (`02_roadmap6-09-29_...`, a lost backslash from an old heredoc).

## 8. Related files
`C:\ZND\50_projects\burnmon\02_roadmap\2026-10-01_station_secret_screen.md`,
`C:\ZND\50_projects\burnmon\tools\station_atlas\CREDITS.txt`,
`C:\ZND\50_projects\burnmon\04_assets\2026-10-02_station_site_sheet.png`,
`C:\ZND\50_projects\burnmon\04_assets\2026-10-01_station_preview.html`,
decisions `C:\ZND\10_holding\03_logs\decisions.md` 2026-10-01 and 2026-09-29.
