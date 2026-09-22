# BurnMon, status

What is true at the v0.1.2 tag, 2026-09-22.

## What it is

BurnMon is the successor of claudecost, a vendor-agnostic token and cost monitor for
Wilco's own Claude Code and Codex usage: a portable Windows app (`burnmon.exe`) plus a
single-file CLI (`burnmon-cli.exe`), reading local session transcripts directly, no
account or API key.

## Adapters

- **Claude** (`internal\adapter\claude`): Claude Code CLI, Cowork/desktop agent-mode
  sessions, `claude-code-sessions`.
- **Codex** (`internal\adapter\codex`): Codex CLI/Desktop rollout transcripts, native and
  WSL roots.

## Pages

Tab bar: **Now**, Overview, Months, Weeks, Days, Sessions, Tools, How, About.

- **Now** (v0.1 Step 3): every running Claude Code/Codex session, polled every 2 seconds,
  a live 30-minute/10-second-bucket burn chart, session cards with a context gauge, a
  forecast panel.
- The rest is claudecost's original historical dashboard: current/previous month, by
  month, week, day and session, in euros, plus Tools.

## Known gaps at this tag

- README is still mostly claudecost's original text (says "claudecost",
  `claudecost.json`); a full pass is scoped for v0.2.
- Now page is English only, single-vendor totals only (no per-vendor split yet).
- No forecast line on the live chart yet (only the day-ahead table).
- `_board` still shows claudecost-era rows; renamed this tag.

## v0.1.2 patch, this tag

F7: a new Codex session could sit invisible until "Refresh now" forced a full rescan.
Root cause confirmed live and by test (`TestWatcher_NewNestedDayFolderRace`,
`internal\watch\watch_test.go`): Codex creates its whole `YYYY/MM/DD` path and the
rollout file back to back, and the file's own Create event could fire, and be dropped by
Windows `ReadDirectoryChanges`, before the watch on that brand-new leaf directory was
registered. `addTree` now re-lists a freshly-created directory for files that raced past
it. F8: the Now chart no longer destroys and rebuilds itself every 2-second poll; one
persistent Chart.js instance, per-session datasets updated in place, both axis maxima
smoothed instead of snapped to the raw window peak. F9: this file, version strings, README
first section, board rows.

## Next release

v0.2, one release tagged `v0.2.0`, due 2026-11-14, two BurnMon sessions a week starting
after this tag. Plan: `02_roadmap\2026-09-22_v0.2_spec.md`.
