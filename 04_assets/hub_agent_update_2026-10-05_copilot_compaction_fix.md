# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-10-05 (Claude Code session on the laptop) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** Tells the hub that the Copilot in VS Code "always Compacting" bug is fixed and pushed, what the measured cause was, and that the roadmap's inferred cause and proposed fix were both corrected.
**Read order:** this file, then `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md` (last section), then `C:\ZND\50_projects\burnmon\internal\insight\insight.go`
**Supersedes:** nothing

## 1. Headline
The Station showed Copilot in VS Code sessions as Compacting almost whenever they were active. The compaction rule in `insight.go` is now scoped for that agent. Committed as `1a22fb9` and on `origin/main` (live-checked). Not tagged, not released.

## 2. What changed on disk
- **Committed** in `C:\ZND\50_projects\burnmon`: `1a22fb9` "insight: Copilot VS Code compaction compares within one model, ignores helper-sized contexts" (Wilco, 2026-10-05 21:33 +0200). `git ls-remote origin refs/heads/main` returns the same SHA.
- **On a branch, not merged:** nothing.
- **Files touched** (the commit stat shows exactly these two):
  - `C:\ZND\50_projects\burnmon\internal\insight\insight.go` (new `compactionBaseline`, constants `copilotVSCodeAgent` and `copilotCompactionFloor` = 10,000)
  - `C:\ZND\50_projects\burnmon\internal\insight\insight_copilot_test.go` (three tests)
- **Untracked / outside a repo:**
  - This brief.
  - `C:\ZND\50_projects\burnmon\burnmon-dev.exe~`, 12.8 MB, dated 2026-10-03, not from this session.
  - Copies of `burnmon.db` and `burnmon-dev.db` plus WAL files in the session scratchpad under `C:\Users\WILCOD~1\AppData\Local\Temp\claude\C--ZND-50-projects-burnmon\...\scratchpad\`. Not durable and not in the repo, but they hold session titles and project paths.

## 3. What did NOT happen (and why)
- No tag, no release, no new alpha. The fix sits after `v0.4.0-alpha.10` (`dd5125c`), the newest tag on origin. Release is Wilco's call.
- `.\scripts\uicheck.ps1` was not run: no UI file changed.
- Nobody looked at the Station screen after the fix. That Compacting clears is read from `live.go:753` and `stage.go:88`, not observed.
- The old Go rule was not re-run. Its "before" counts come from a Python mirror of the rule.
- The CLI (`burnmon-cli insight`) was not used: it does a full disk scan and writes the live store.
- Not touched: `live.go:278` (context gauge) and the context-runway fit, see section 7.
- The backlog entry, `STATUS.md` and `SESSION_LOG.md` were not updated by this session.
- `02_roadmap\2026-10-03_station_ui_and_backlog.md` was already modified at session start and was not edited by this session.
- No hub file was propagated into.

## 4. Findings worth propagating
- [RESULT] Real store, a copy of `burnmon.db` taken 2026-10-05 evening: Copilot VS Code has 508 events in 7 sessions, every cache field null. Each session interleaves two lanes: the main model (claude-sonnet-5.5, context 60k to 725k, growing) and a helper model (`gpt-4o-mini-2024-07-18`, 253 to 3,826 tokens, a fresh prompt each time).
- [RESULT] Old rule on those 7 sessions: 101 compaction findings (Python mirror). After the fix, real `Analyze` over the same copy: 1. Claude Code 46, Cowork 31 and Codex 13 are unchanged from the old-rule counts.
- [RESULT] The roadmap's inferred cause was half right. The cause is the two-lane interleaving, not empty cache fields. "Same model as the previous turn" alone would not have worked: 20 of the 101 findings were same-model helper drops, including the trigger in Wilco's session `4be14c72` (turn 59, 1,151 to 253, helper to helper). The rule fired on 17 of 60 turns in that session, not almost every turn.
- [RESULT] The one real Copilot compaction in the store, session `20310c89`, 2026-10-01 20:12:02, 725,556 to 97,476 on the main model, still fires. It is the single remaining finding.
- [RESULT] `go vet ./...`, `go test ./... -count=1` and `.\build.ps1` all exit 0. Run on the tree before Wilco's commit. Not re-run on `1a22fb9`, which adds only the two files above.
- [STATE] The 10,000-token floor is a judgment value. In the 7 sessions the helper lane tops out at 3,826 and the main lane starts near 60,000. A real compaction from a context under 10,000 would be missed.
- [STATE] The rule is scoped by `Agent == "copilot-vscode"` and compares within one model. A global per-model rule was rejected: on real data it would have dropped 7 Cowork findings (model switches) and added 3 Codex findings (rows with empty model).

## 5. Hub-level decision (if any)
Nothing. A project-internal rule change, no cross-project effect, so no `decisions.md` entry.

## 6. What the next hub read should update
- `C:\ZND\50_projects\burnmon\STATUS.md`, top: the fix is on `main` at `1a22fb9`, untagged.
- `C:\ZND\50_projects\burnmon\SESSION_LOG.md`: one paragraph for this session, not yet written.
- `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`, last section: mark fixed, replace the inferred cause with the measured one.
- `C:\ZND\10_holding\01_projects\burnmon.md`: one line, only if the hub tracks fixes during the park.

Tracker rows moved: none identified. Section 5 ("Current plan", line 238 of `C:\ZND\10_holding\02_roadmap\roadmap.md`) was not read in full. The only BurnMon mention I saw there is a calendar line (243). No This Week item (A1..A8, B1..B5) is named.

## 7. Open flags for next session
- Release: whether this ships in the next alpha is Wilco's decision. The park to 2026-11-01 stands except for the Station feature.
- Related, read from code only, not observed: `C:\ZND\50_projects\burnmon\internal\live\live.go:278` takes the context gauge from the last turn, so for Copilot it likely shows a helper's 253 to 3,800 tokens. The context-runway fit probably sees the same interleaving.
- Delete the scratchpad store copies, they contain session titles.
- `burnmon-dev.exe~` is stray and untracked, not from this session.
- The Station screen has not been seen with a live Copilot session since the fix.

## 8. Related files
- `C:\ZND\50_projects\burnmon\02_roadmap\2026-10-03_station_ui_and_backlog.md`
- `C:\ZND\50_projects\burnmon\internal\stage\stage.go` (line 88, Compacting rule)
- `C:\ZND\50_projects\burnmon\internal\live\live.go` (line 753, CompactedAt)
- `C:\ZND\50_projects\burnmon\internal\adapter\copilotvsc\copilotvsc.go`
