# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-10-05 (second BurnMon session of the day) - **Owner:** Wilco de Tree
**Project:** BurnMon (Station, `burnmon-dev`)
**Purpose:** Tells the hub that the second Copilot in VS Code Station symptom ("Waiting on you" while working, tiny context) is proven on the real store and fixed in code, and what is still unverified by eye.
**Read order:** `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-10-05_copilot_compaction_fix.md`, then this file, then `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md` (last section).
**Supersedes:** nothing. It is the sequel to the compaction-fix brief above, which stays valid.

## 1. Headline
Fix committed and pushed as `0760623`: a Copilot in VS Code session now reads Thinking (not Waiting) for up to 3 minutes after its newest request, and its context and context window come from the newest conversation-sized request, not a helper's. Proven by replay on a copy of the real store; not yet looked at in the live Station.

## 2. What changed on disk
- **Committed and on origin/main** in `C:\ZND\50_projects\burnmon`: `0760623` "station: Copilot VS Code shows Thinking between requests, context ignores helper requests". Checked live on 2026-10-05: `git rev-parse` HEAD and `git ls-remote origin refs/heads/main` both return `e3c2506`, the docs commit on top of `0760623`. No tag was made.
- **Files touched:**
  - `C:\ZND\50_projects\burnmon\internal\stage\stage.go` (new `Input.NoToolTrail`, Thinking until `StuckToolAfter`, then Waiting)
  - `C:\ZND\50_projects\burnmon\internal\stage\stage_test.go` (`TestNoToolTrail`)
  - `C:\ZND\50_projects\burnmon\internal\insight\insight.go` (`ContextTurn`, agent constant exported as `CopilotVSCodeAgent`)
  - `C:\ZND\50_projects\burnmon\internal\live\live.go` (context and window from `ContextTurn`, `NoToolTrail` set for `copilot-vscode` only)
  - `C:\ZND\50_projects\burnmon\internal\live\copilot_test.go` (new, synthetic sequences plus Claude Code and copilot-cli controls)
- **Untracked, outside the repo:** `C:\ZND\50_projects\burnmon\burnmon-dev.exe~` (editor leftover, not mine). A copy of the real `burnmon.db` sits in the Claude scratchpad temp folder; it holds real data and is not part of the repo.

## 3. What did NOT happen (and why)
- Not looked at in the running Station. `.\build.ps1` built the new `burnmon-dev.exe`, but nobody has restarted it and watched a live Copilot session.
- `.\scripts\uicheck.ps1` not run: no UI file changed.
- No tag, no release.
- Not changed: `Session.Model` (still shows the helper model), copilot-cli stage (it also records no tool calls, but its 25 sessions have one event each, too thin to measure), the context-runway finding for Copilot (not examined).
- No prompt text, project name, file path or client name was printed, copied into a test or written into any file. The tests use synthetic sequences only.

## 4. Findings worth propagating
- [RESULT] Replay of the real Copilot in VS Code sessions through the real `BuildSnapshot` and `ApplyStages`, taken 3 s, 30 s and 200 s after each request: "waiting" in 109 of 109 samples at every lag (plus 6 "arriving" at 3 s), including right after requests with a 0 to 2 s gap. The store holds 520 `copilot-vscode` rows in 7 sessions and zero tool-call rows for them.
- [RESULT] 435 of 520 requests have under 10,000 input tokens and 6 of 7 sessions end on one. Before the fix the Station showed 253 to 5,523 tokens for sessions whose main requests run 60k to 725k, against the helper model's window.
- [RESULT] After the fix, same replay: Waiting at 3 s and 30 s in 0 samples, Waiting at 200 s in 109 of 109, samples showing a context under 10,000 at 3 s down from 88 to 4 (sessions with only helper requests keep the newest turn).
- [RESULT] Other agents unchanged: 3,294 replayed samples across claude-code, cowork, codex, copilot-cli and hermes, before (HEAD exported read-only) versus after, zero differences in stage, tool, context or window.
- [RESULT] Gaps between consecutive Copilot requests: median 9 s, 93.2 percent at 180 s or less (513 gaps).
- [STATE] The 3-minute Thinking window is an inference. It reuses `StuckToolAfter`, supported by the 93.2 percent figure, but a real hand-back and a long pause look identical in this trail.
- [STATE] The mechanism (Classify falling through to Waiting for a turn with no tool call) matches the 109 of 109 result but the branch was not instrumented.
- [RESULT] Verification on the committed tree: `go vet ./...` exit 0, `go test ./... -count=1` all packages ok, `.\build.ps1` exit 0.

## 5. Hub-level decision (if any)
Nothing. Project-internal.

## 6. What the next hub read should update
- `C:\ZND\50_projects\burnmon\STATUS.md` and `C:\ZND\50_projects\burnmon\SESSION_LOG.md`: fix `0760623` shipped, live Station check still pending.
- `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`: the Copilot section is now fix-shipped, not open.
- `C:\ZND\10_holding\01_projects\burnmon.md`: only if the one-pager lists the Copilot Station bugs.

Mission Deck This Week item: none named.
Tracker rows moved: none.

## 7. Open flags for next session
- Restart `burnmon-dev.exe`, watch a live Copilot session, confirm Thinking between requests and a sensible context bar. Close the Copilot item only after that.
- `Session.Model` shows the helper model for Copilot sessions (the drawer says gpt-4o-mini while the conversation runs on another model).
- Model gpt-6.1-sol has no context window entry, so its window reads 0.
- copilot-cli has no tool rows either; decide whether it needs the same rule once it has sessions with more than one event.
- Context-runway finding for Copilot interleaved requests: not checked.

## 8. Related files
- `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-10-05_copilot_compaction_fix.md`
- `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`
- `C:\ZND\50_projects\burnmon\internal\insight\insight_copilot_test.go`
