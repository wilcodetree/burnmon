# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-22 - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** the v0.1.2 patch (F7, F8, F9) is code-complete, tested, built green, committed and tagged locally; not yet pushed.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md` (top three entries, F9/F8/F7 in that order), then `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1.2_patch_spec.md` for the source requirements.
**Supersedes:** nothing (this project's fourth hub brief; companion to the three v0.1 step briefs already sitting untracked in this same folder).

## 1. Headline
BurnMon v0.1.2 (three defects: a missed live Codex session, a jumping Now chart, stale product identity strings) is fixed, tested, committed at `e99ecf9`, and tagged `v0.1.2` on Wilco's own machine. Not pushed to `origin` yet.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (GitHub: `wilcodetree/burnmon`): `e99ecf9` "fix: F7-F9, live Codex race, Now chart jump, v0.1.2 identity".
- **Tagged, not pushed:** `v0.1.2` on `e99ecf9`, local only. `git status -sb` shows `main...origin/main [ahead 1]`; neither the commit nor the tag has been pushed this session.
- **Files touched** (full paths): `C:\ZND\projects\burnmon\internal\watch\watch.go` and `watch_test.go` (F7 fix plus new regression test `TestWatcher_NewNestedDayFolderRace`), `C:\ZND\projects\burnmon\internal\report\template.html` (F8, `drawNowChart` rewritten), `C:\ZND\projects\burnmon\cmd\burnmon\main.go` and `C:\ZND\projects\burnmon\cmd\burnmon-cli\main.go` (F9, version `0.8.1` to `0.1.2`), `C:\ZND\projects\burnmon\README.md` (F9, first section rewritten), `C:\ZND\projects\burnmon\STATUS.md` (F9, filled in from the empty 2026-09-08 stub), `C:\ZND\projects\burnmon\_board\board.json` and `board.html` (F9, claudecost to burnmon), `C:\ZND\projects\burnmon\SESSION_LOG.md` (three new entries, one per defect).
- **Untracked, still outside the repo (so it is not lost):** this file and the three v0.1 step briefs sit in `C:\ZND\projects\burnmon\04_assets\`, none yet ingested by a hub read. A stray `burnmon.exe~` from a rebuild-while-locked byproduct also sits untracked; harmless, safe to delete, not part of this patch.

## 3. What did NOT happen (and why)
- Not pushed. The commit and tag are local only; Wilco ran `git commit` and `git tag v0.1.2` himself but has not run `git push`.
- No hub file was touched by this session, no propagation into STATUS/DEADLINES/portfolio/decisions/wiki. That is this brief's job to flag, not to do.
- F7's root cause was confirmed live (a diagnostic build run alongside Wilco's real Codex CLI sessions) for the "day folder already exists" case, which worked correctly and ruled out two of the three hypotheses; the exact race (a brand-new nested day folder) could not be forced live since today's folder already existed at burnmon startup, so it was reproduced instead by a new automated test (`TestWatcher_NewNestedDayFolderRace`), which failed 4 of 5 runs before the fix and passed 10 of 10 after. This is a test-confirmed root cause, not a live-observed one for the exact failure mode.
- F8's "5 minutes of watching, no visible jump" check was done live by Wilco on the rebuilt binary (1 Claude CLI, 4 Codex CLI sessions running) and he confirmed it, but this is a visual call, not a number; no chart-jump metric exists to report.
- F9's `_board` rename was hand-edited directly in the generated `board.json`/`board.html`: no `siteoffice.json` source or generator script was found under `C:\ZND\projects\burnmon` to regenerate from instead (the file's own header still points at the old `C:\ZND\projects\claudecost\siteoffice.json`, which does not exist). This is flagged as a gap, not fixed, since restoring a real Siteoffice source is outside this patch's scope.
- v0.2 has not started. A spec already exists (`02_roadmap\2026-09-22_v0.2_spec.md`, decided in the same day's grill), but no v0.2 build session has run.

## 4. Findings worth propagating
- [RESULT] `go build ./...`, `go test ./... -count=1`, and `.\build.ps1` are all green at `e99ecf9`, including the new `internal\watch` regression test for F7.
- [RESULT] F7's root cause is hypothesis 3 from the spec: Windows `ReadDirectoryChanges` is per-directory, not recursive, so a rollout file's own Create event can fire and be silently dropped while `fsw.Add` on its brand-new parent directory is still in flight. Fixed by re-listing a freshly-created directory for any file that raced past the watch, in `internal\watch\watch.go`'s `addTree`.
- [RESULT] F8's Now chart (`internal\report\template.html`, `drawNowChart`) now holds one Chart.js instance for the life of the page instead of destroying and rebuilding it every 2-second poll; axis maxima are smoothed (grow instantly to a new peak, decay 5% per poll) instead of snapped to the raw window peak each time.
- [STATE] Not pushed yet; the tag `v0.1.2` exists only on Wilco's own machine.
- [STATE] v0.2 (Copilot CLI, Hermes, forecast line, re-prefill/compaction insight, five-tab page set) is spec'd but not started, due 2026-11-14 at two BurnMon sessions a week from this tag.

## 5. Hub-level decision (if any)
Nothing. This is a project-internal bugfix patch, no cross-project time, positioning, pricing or CIPHER-wall question involved.

## 6. What the next hub read should update
- `C:\ZND\10_holding\02_roadmap\roadmap.md`: v0.1.2 patch complete 2026-09-22, pending push; v0.2 next, due 2026-11-14.
- `C:\ZND\projects\burnmon\DEADLINES.md`: confirm v0.1.2 patch entry closed once pushed.
- `C:\ZND\10_holding\01_projects\portfolio.md` and `C:\ZND\projects\burnmon\04_assets\`: fold in this brief plus the three untracked v0.1 step briefs still sitting in the same folder, all four still pending a hub-update ingest pass.
- `C:\ZND\10_holding\01_projects\burnmon.md` (hub one-pager): reflect v0.1.2 patch shipped (pending push), v0.2 spec'd, not started.
- Wiki `hot.md`: worth a line once pushed.

## 7. Open flags for next session
- Push `main` and the `v0.1.2` tag to `origin` (Wilco's manual step, per house rule: no git writes performed on his behalf beyond the commit/tag commands he ran himself).
- Four hub briefs (three v0.1 steps plus this one) are still untracked in `C:\ZND\projects\burnmon\04_assets\` and need a hub-update ingest pass.
- `_board`'s generator source (`siteoffice.json`) does not exist for this project; a future Siteoffice pass should restore one instead of leaving `board.json`/`board.html` hand-edited.
- Start the v0.2 build (two sessions a week, plan already written) once pushed.

## 8. Related files
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1.2_patch_spec.md` (the spec this patch implements).
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` (next release, not yet started).
- `C:\ZND\projects\burnmon\SESSION_LOG.md` (top three entries: F9, F8, F7, this session's full technical account).
- `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.1_step1_schema_store_adapter.md`, `hub_agent_update_2026-09-22_v0.1_step2_codex_adapter.md`, `hub_agent_update_2026-09-22_v0.1_step3_now_page.md` (prior briefs, all still pending ingest).
