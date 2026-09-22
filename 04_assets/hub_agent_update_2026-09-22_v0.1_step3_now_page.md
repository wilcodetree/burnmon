# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-22 - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** v0.1 Step 3 (the Now page) is shipped, tagged, and pushed; v0.1 as a whole is complete, three and a half weeks ahead of its 2026-10-17 deadline.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry), then `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1_spec.md` Step 3 for the source requirements.
**Supersedes:** nothing (companion to `hub_agent_update_2026-09-22_v0.1_step1_schema_store_adapter.md` and `hub_agent_update_2026-09-22_v0.1_step2_codex_adapter.md`, not a replacement for either).

## 1. Headline
v0.1 Step 3, the Now page, is code-complete, tested, built green, committed, tagged `v0.1.0`, and pushed to `origin/main`. v0.1 (schema/store/Claude adapter, Codex adapter, Now page) is done.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (GitHub: `wilcodetree/burnmon`): `693ea5a` "feat: the Now page (v0.1 Step 3)".
- **Tagged and pushed:** `v0.1.0` on `693ea5a`, pushed to `origin/main` and `origin/v0.1.0` (confirmed by the push output: `f9d4b68..693ea5a main -> main`, `* [new tag] v0.1.0 -> v0.1.0`).
- **Files touched** (full paths): `C:\ZND\projects\burnmon\internal\live\live.go` and `live_test.go` (new package), `C:\ZND\projects\burnmon\internal\watch\watch.go` and `watch_test.go` (new package), `C:\ZND\projects\burnmon\cmd\burnmon\main.go` (bound `bmLive`, live watcher startup), `C:\ZND\projects\burnmon\cmd\burnmon-cli\main.go` (`live -json` verb), `C:\ZND\projects\burnmon\internal\dataset\dataset.go` (`Cache.IngestFile`), `C:\ZND\projects\burnmon\internal\pricing\pricing.go` (`context_window` table, `LiveRunningWindowMinutes`), `C:\ZND\projects\burnmon\internal\report\template.html` (new "Now" tab), `C:\ZND\projects\burnmon\go.mod` / `go.sum` (added `github.com/fsnotify/fsnotify`), `C:\ZND\projects\burnmon\SESSION_LOG.md`.
- **Untracked, still outside the repo (so it is not lost):** this file and the two prior step briefs sit in `C:\ZND\projects\burnmon\04_assets\`, not yet ingested by a hub read.

## 3. What did NOT happen (and why)
- No live Codex session ran alongside Claude Code during the building session, so the spec's exact done-when, "both sessions show a context percentage and the chart moves within 2 seconds of a turn, watched together", was not observed directly. It was confirmed structurally instead: `burnmon-cli.exe live -json` correctly returned this very Claude Code session as the one running entry (context 234,819 of a 200,000-token window), and real, already-stale Codex rollout events from earlier in the day flowed through the same code path onto the chart's tail, correctly excluded from the running-sessions list once past the 10-minute window. Wilco has said he will run Claude Code and Codex side by side against the actual `burnmon.exe` app window to close this gap himself.
- The WebView2 app window's on-screen rendering was not visually inspected (no screenshot tooling in that session). Verified instead: the app started with no bind or watcher-startup error in `burnmon-app.log`, and the freshly rendered `dashboard.html` carries the new `now` tab markup, the `ch_now` canvas, and the `startNowPolling` call.
- Cache clock and turn ticker were left out of v0.1 on purpose, per the spec's own "in scope only if the week allows" clause; it did not.
- v0.2 has no spec file yet. `DEADLINES.md` names its scope (Copilot CLI, Hermes, forecast line, re-prefill events, dev/business switch) and its 2026-11-14 date, but nobody has run a spec/grill session for it. Wilco has said he will do that spec run himself next.
- Nothing was pushed or propagated beyond the one commit and tag above; no hub file was touched by this session.

## 4. Findings worth propagating
- [RESULT] v0.1 is complete: all three spec steps (schema/store/Claude adapter, Codex adapter, Now page) are merged to `main` and tagged (`v0.1.0-alpha.1`, `v0.1.0-alpha.2`, `v0.1.0`), against a 2026-10-17 target, roughly 3.5 weeks early.
- [RESULT] `go test ./...`, `go vet ./...`, and `.\build.ps1` are all green at `693ea5a`, including new tests for `internal\live` (running-vs-stale sessions, subagent nesting, unknown-model gauge, and a fixture-append test matching the spec's own done-when) and `internal\watch` (native fsnotify sees a new file and a newly created subdirectory; the WSL poll path fires once per real mtime change, not on a re-poll of an unchanged file).
- [RESULT] The spec's open WSL question is answered, not assumed: a throwaway `fsnotify` probe pointed at a live `\wsl.localhost\Ubuntu-24.04\...` folder on Wilco's own laptop failed to even add the watch (`ReadDirectoryChanges: Incorrect function`), confirmed by writing into that real folder from `wsl -d Ubuntu-24.04` while the probe ran and observing no event. `internal\watch` therefore never attempts fsnotify on a WSL root; those are polled by mtime every 5 seconds, which was the spec's fallback plan, just for a stronger reason ("cannot watch at all" rather than "sees it late").
- [STATE] Full side-by-side live verification (Claude Code and Codex both running against the actual app window) is pending; Wilco is doing this himself next.
- [STATE] A v0.2 spec does not exist yet; Wilco is running that spec session himself next.

## 5. Hub-level decision (if any)
Nothing. Context-window figures used in the price book (200,000 tokens for the Claude family, 400,000 for the Codex/GPT family seen on this laptop) are a project-internal pricing-table detail, not a cross-project or company-positioning call.

## 6. What the next hub read should update
- `C:\ZND\10_holding\02_roadmap\roadmap.md` and `C:\ZND\projects\burnmon\DEADLINES.md`: v0.1 done 2026-09-22 against a 2026-10-17 target.
- `C:\ZND\10_holding\01_projects\portfolio.md` and `C:\ZND\projects\burnmon\04_assets\hub_agent_update` folder: fold in this brief plus the two prior ones (Step 1, Step 2) if the hub has not already ingested them; all three are still sitting untracked in this project's `04_assets`.
- `C:\ZND\10_holding\01_projects\burnmon.md` (the hub one-pager): reflect v0.1 shipped, v0.2 not yet spec'd.
- Wiki `hot.md`: worth a line, v0.1 shipped early with a live Now page.

## 7. Open flags for next session
- Close the live-verification gap: watch the Now page update within 2 seconds of a turn with Claude Code and Codex both running against `burnmon.exe`.
- Run a spec/grill session for v0.2 (Copilot CLI, Hermes, forecast line, re-prefill events, Copilot honest-label rows, dev/business switch) before any v0.2 build session starts.
- Three prior hub briefs for this project (Step 1, Step 2, and this one) are still untracked in `C:\ZND\projects\burnmon\04_assets\` and need a hub-update ingest pass.

## 8. Related files
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1_spec.md` (the spec this step implements).
- `C:\ZND\projects\burnmon\04_assets\2026-09-22_burnmon_now_page_features.md` (design source for the Now page, sections 2 and 3).
- `C:\ZND\projects\burnmon\04_assets\2026-09-22_token_monitor_architecture.md`.
- `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.1_step1_schema_store_adapter.md` and `hub_agent_update_2026-09-22_v0.1_step2_codex_adapter.md` (prior steps' briefs).
- `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry, this session's full technical account).
