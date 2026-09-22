# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-22 - **Owner:** Wilco de Tree
**Project:** BurnMon (formerly claudecost)
**Purpose:** 43A (I3) shipped and committed: markers, turn ticker and the "explain this spike" drawer on the Now page, plus a schema addition (tool_calls.path) decided mid-session.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry), then `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` section 2.3
**Supersedes:** `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.2_42B_runway_expensive_turn.md` (extends it, does not replace it)

## 1. Headline
43A is committed to `main`: the Now page's live chart now carries one marker per
insight finding (shaped by kind), a turn ticker under the chart, and a click-through
drawer with the full turn detail. Committed, not tagged: no `v0.2.0-alpha.N` tag was cut
this session, and nothing was pushed to a remote.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (branch `main`): `e381f80` "feat: markers,
  ticker and spike drawer on Now, tool_calls.path (I3, 43A)", 15 files changed, 828
  insertions, 30 deletions.
- **Not tagged.** The build order table has 43A tagging `v0.2.0-alpha.4` alongside 43B
  (Copilot CLI, OTel check); that tag was not cut, since 43B has not run yet.
- **Not pushed.** `main` is 2 commits ahead of `origin/main`; Wilco was handed the push
  command and had not confirmed running it as of this brief.
- **Files touched** (full paths, all in the commit above):
  `C:\ZND\projects\burnmon\internal\live\live.go`,
  `C:\ZND\projects\burnmon\internal\live\live_test.go`,
  `C:\ZND\projects\burnmon\internal\live\turndetail_test.go` (new),
  `C:\ZND\projects\burnmon\internal\report\template.html`,
  `C:\ZND\projects\burnmon\internal\schema\event.go`,
  `C:\ZND\projects\burnmon\internal\store\migrations\migrations.go`,
  `C:\ZND\projects\burnmon\internal\store\store.go`,
  `C:\ZND\projects\burnmon\internal\store\store_test.go`,
  `C:\ZND\projects\burnmon\internal\adapter\claude\claude.go`,
  `C:\ZND\projects\burnmon\internal\adapter\claude\claude_test.go`,
  `C:\ZND\projects\burnmon\internal\adapter\codex\codex.go`,
  `C:\ZND\projects\burnmon\internal\adapter\codex\codex_test.go`,
  `C:\ZND\projects\burnmon\cmd\burnmon\main.go`,
  `C:\ZND\projects\burnmon\SESSION_LOG.md`,
  `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (43A ticked).
- **Untracked, not this session's work, left alone:**
  `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1.2_patch_spec.md` (modified before
  this session started) and this brief's own predecessor,
  `hub_agent_update_2026-09-22_v0.2_42B_runway_expensive_turn.md` (already untracked at
  session start, per that brief's own note it was meant to fold into a later commit; it
  was not folded into `e381f80` either, still sitting untracked).

## 3. What did NOT happen (and why)
Not pushed to any remote (`origin` or the `mirror` remote at `ssh://cipher/...`): Wilco
was given the push command, decided pushing is his own call, and had not run it as of
this brief. Not tagged: 43B has not run. The spec's own required check, "watch live for
5 minutes that markers appear within 2 seconds of a turn and the chart still drifts
without jumping," was not performed visually: this agent has no way to see or click a
native WebView2 window. It substituted a headless proxy instead (see section 4) and
said so explicitly in `SESSION_LOG.md` rather than claiming the visual check happened.
Sessions-tab findings display (the spec's I3 "Sessions tab" clause) did not ship this
session either: the build order table places that under 44A, not 43A, and the session
prompt for 43A itself scoped to "markers, ticker, spike drawer" only.

Two Codex-side gaps are logged, not silently accepted as working: file-path extraction
for a Codex "custom_tool_call" (most of which carry a shell command string, not JSON)
will resolve to no path in the common case, by design, not a bug; and the join between
a Codex tool call and its owning turn (tool_calls.turn, a turn_id from the rollout's own
metadata) has not been verified to actually coincide with the turn's Event.RequestID,
the key BuildTurnDetail joins on. No fixture exists to prove or disprove this either
way. A Codex turn whose tool calls do not join this way simply shows none in the
drawer, not an error.

## 4. Findings worth propagating
- [RESULT] `go test ./... -count=1`, `.\build.ps1`, and `node --check` (on the extracted
  inline script, `internal/report/template.html` lines 462-1947) all green after the
  change, run from `C:\ZND\projects\burnmon`.
- [RESULT] Open choice resolved with Wilco directly this session: `tool_calls` (S2,
  added in an earlier session) had no file-path column at all, so the drawer's "files
  read" line had nowhere to read from. Wilco chose to add the column now (migration4,
  additive) rather than ship the drawer without that line. Both adapters populate it
  best-effort: confirmed working for Claude (a real fixture's file_path value round-
  trips through ToolCall.Path), unverified for Codex's shell-heavy tool calls (see
  section 3).
- [RESULT] Verified end to end against Wilco's real local BurnMon store, not a
  synthetic fixture: a full `burnmon-cli.exe` ingest pass migrated the live store from
  schema version 3 to 4 with no error; `burnmon-cli.exe live -json` showed this very
  Claude Code session's own turns and a real re-prefill finding on turn 8; a throwaway
  `go run` of live.BuildTurnDetail against the same store returned the matching tool
  call (joined by RequestID) and the correct gap in seconds, then was deleted. Full
  detail in `C:\ZND\projects\burnmon\SESSION_LOG.md`'s 43A entry.
- [RESULT] Built and started the freshly built `burnmon.exe`, replacing a stale
  instance from an earlier build that still held the single-instance mutex;
  `burnmon-app.log` polled cleanly (poll 30, 60, 90, no panic, no "could not bind") for
  several minutes under real load. Not the spec's own 5-minute visual check (see
  section 3), the strongest available substitute.
- [STATE] 43B (Copilot CLI adapter, OTel check, tag `v0.2.0-alpha.4`) is next per the
  build order table, week 43, not started.
- [STATE] `main` is 2 commits ahead of `origin/main`, not pushed as of this brief.

## 5. Hub-level decision (if any)
Nothing. The tool_calls.path schema addition is project-internal (BurnMon's own store
schema, decided with Wilco as an open choice within the session), not a cross-project,
pricing, positioning, or CIPHER-wall call.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` (hub one-pager): 43A done, not tagged;
  note the unpushed `main`.
- `C:\ZND\10_holding\02_roadmap\roadmap.md` if it tracks BurnMon by build-order week:
  week 43 session A (43A) closed, session B (43B) still open.
- No DEADLINES.md change expected: the v0.2 release date (2026-11-14) and the
  two-session-a-week cadence are unaffected.

## 7. Open flags for next session
Push `main` to `origin` (and the `mirror` remote, if Wilco keeps both current) before or
during 43B, so the two do not diverge further. 43B needs a Claude Code and a Codex
session running live before starting (per the session prompts file's own note). The
Codex tool-call turn-join and file-path extraction gaps (section 3) are unverified, not
known broken; worth a real Codex fixture check whenever a Codex session's drawer is
actually opened and compared against its own transcript. This brief's own predecessor
(42B's) is still untracked in `04_assets`; worth folding into whichever commit next
touches this project, per that brief's own note.

## 8. Related files
`C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` (section 2.3, I3),
`C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (43A prompt,
line 79; checklist line 148),
`C:\ZND\projects\burnmon\SESSION_LOG.md` (43A entry, top of file),
`C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.2_42B_runway_expensive_turn.md`
(the 42B brief this one extends).
