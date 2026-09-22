# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-22 (session 40A) - **Owner:** Wilco de Tree
**Project:** BurnRate/BurnMon (`C:\ZND\projects\burnmon`)
**Purpose:** v0.2 session 40A shipped: the tool_calls table, both-adapter extraction, burnmon-cli tools --json, and the windowed burnmon-cli live fix (spec S2, S3), committed and pushed to main.
**Read order:** this file, `C:\ZND\projects\burnmon\SESSION_LOG.md` top entry, `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` checklist.
**Supersedes:** nothing.

## 1. Headline
Session 40A (spec S2, S3) is code-complete, tested, and merged to `main`: a `tool_calls` table (migration 3), extraction from both the Claude and Codex adapters, `burnmon-cli tools --since --json`, and `burnmon-cli live` fixed to use the windowed store query instead of loading every event. Pushed to `origin/main`.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (branch `main`): `e469457` "feat: tool_calls table, both-adapter extraction, tools CLI, windowed live (S2, S3)". Pushed; `main` matches `origin/main` at this commit.
- **On a branch, not merged:** nothing; work went straight to `main` per this project's existing convention.
- **Files touched** (full paths): `C:\ZND\projects\burnmon\internal\schema\event.go` (new `ToolCall` type), `C:\ZND\projects\burnmon\internal\store\migrations\migrations.go` (migration 3), `C:\ZND\projects\burnmon\internal\store\store.go` (`UpsertToolCalls`, `ToolCallTotals`), `C:\ZND\projects\burnmon\internal\adapter\adapter.go` (interface signature change), `C:\ZND\projects\burnmon\internal\adapter\claude\claude.go`, `C:\ZND\projects\burnmon\internal\adapter\codex\codex.go`, `C:\ZND\projects\burnmon\internal\dataset\dataset.go` (ingest wiring, one-time backfill flag), `C:\ZND\projects\burnmon\cmd\burnmon-cli\main.go` (`tools` command, `live` fix), plus test files and two new fixtures (`C:\ZND\projects\burnmon\internal\adapter\claude\testdata\toolcalls.jsonl`, `C:\ZND\projects\burnmon\testdata\codex\tool-calls.jsonl`).
- **Untracked / outside a repo:** none from this session; two pre-existing untracked/modified files in the tree (`02_roadmap\2026-09-22_v0.1.2_patch_spec.md` modified, `04_assets\hub_agent_update_2026-09-22_v0.2_39B_migrations_owner.md` untracked) predate this session and were deliberately left out of this commit.

## 3. What did NOT happen (and why)
No Tools tab UI: P1 (the five-tab page shell) has not landed yet (that is session 40B, next), so there is nothing to wire a Tools tab into. `burnmon-cli tools` and `store.ToolCallTotals` exist so 40B/41A can call the same query once the tab exists. No cross-project or hub-level decision was made this session. The Codex tool-call scope question (below) was resolved with Wilco in-session via a direct question, not deferred.

## 4. Findings worth propagating
- [RESULT] The v0.2 spec's S2 wording ("Codex: `function_call` items") does not match real data: a sampled live rollout on this laptop had 54 `custom_tool_call`/`custom_tool_call_output` pairs (name `exec`, real shell/tool execution) against only 10 `function_call`/`function_call_output` pairs. Implementing only `function_call` would have captured a small minority of real Codex tool calls. Wilco chose to implement both families; the spec text itself was not edited, only the code and SESSION_LOG.md.
- [RESULT] `go test ./... -count=1` and `.\build.ps1` both green after the change.
- [RESULT] Verified end-to-end against the real ~33 MB store on this laptop, not only fixtures: the one-time backfill (gated by a new `tool_calls_backfilled_v1` meta flag) re-read every real transcript once; a follow-up `tools --json` run completed in about 4 seconds; `live -json` correctly showed this very session running with the right turn count and cache-hit ratio.
- [STATE] Session 40B (five tabs, Now default, English only, tag `v0.2.0-alpha.1`) is next per the checklist in `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md`.

## 5. Hub-level decision (if any)
Nothing. The Codex tool-call scope call was project-internal (which payload families a parser reads), not a cross-project, pricing, positioning, or CIPHER-wall decision.

## 6. What the next hub read should update
`C:\ZND\projects\burnmon\STATUS.md` (not yet touched this session; still describes v0.1.2) and `C:\ZND\10_holding\01_projects\burnmon.md` (the hub one-pager) could both note S2/S3 done, 40A ticked, 40B next. Roadmap/DEADLINES untouched, no date changed.

## 7. Open flags for next session
Session 40B (five tabs including a real Tools tab UI) is next in the checklist; it should read `store.ToolCallTotals` rather than re-deriving a query. The v0.2 spec's own S2 prose ("function_call items") is now stale against what was actually built and could use a one-line correction in a future spec pass, though nothing is blocking on it.

## 8. Related files
`C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` (section 2.1, S2/S3), `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (checklist, 40A now ticked), `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry, this session's full paragraph).
