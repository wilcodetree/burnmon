# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-29 - **Owner:** Wilco de Tree
**Project:** BurnMon (BurnMon Dev, WS2)
**Purpose:** BurnMon Dev v0.4.0-alpha.8 is code-complete and verified locally: Codex turns now reach BurnMon live, and every Codex session that had collapsed into one stored event has been re-read in full.
**Read order:** this file, `C:\ZND\50_projects\burnmon\STATUS.md` (the `v0.4.0-alpha.8` entry), `C:\ZND\50_projects\burnmon\02_roadmap\2026-09-29_ws2_codex_live_tail.md`
**Supersedes:** nothing (follows `hub_agent_update_2026-09-29_v0_4_0_alpha7_smooth_tick.md`)

## 1. Headline
v0.4.0-alpha.8 (spec items 0 to 5) is built and tested on `main` in the working tree, not committed. The 15-minute real-run proof passed, and the store's Codex numbers now match the rollout files exactly.

## 2. What changed on disk
- **Committed:** nothing. Wilco commits by hand (no git writes on `C:\ZND`).
- **Modified:** `C:\ZND\50_projects\burnmon\internal\adapter\codex\codex.go`, `codex_test.go`; `C:\ZND\50_projects\burnmon\internal\adapter\copilotvsc\copilotvsc.go`, `copilotvsc_test.go`; `C:\ZND\50_projects\burnmon\internal\store\store.go`, `store_test.go`; `C:\ZND\50_projects\burnmon\internal\dataset\dataset.go`; `C:\ZND\50_projects\burnmon\internal\watch\watch.go`, `watch_windows.go`, `watch_other.go`, `watch_test.go`; `C:\ZND\50_projects\burnmon\internal\live\live.go`, `live_test.go`; `C:\ZND\50_projects\burnmon\cmd\burnmon\app.go`; `C:\ZND\50_projects\burnmon\cmd\burnmon-dev\app.go`, `page.html`; `C:\ZND\50_projects\burnmon\README.md`, `STATUS.md`, `SESSION_LOG.md`.
- **New:** `C:\ZND\50_projects\burnmon\internal\watch\tail.go`; `C:\ZND\50_projects\burnmon\testdata\codex\three-turns-no-ordinal.jsonl`; this brief.
- **Untracked, not mine:** `C:\ZND\50_projects\burnmon\02_roadmap\2026-09-29_ws2_codex_live_tail.md` (the spec).
- **Outside the repo:** store backup taken before the re-ingest, `C:\Users\WilcoDeTree\AppData\Local\burnmon\burnmon.db.bak-2026-09-29-alpha8`.

## 3. What did NOT happen (and why)
- Not committed, not pushed, not tagged: Wilco's manual step.
- `burnmon.exe` keeps version `0.3.2`, though it shares the fixed ingest code and was rebuilt. No release note for it.
- The burn chart was not screenshotted turn by turn. "In the chart within 5 s" is inferred: the store had each turn within about 2 s, and the chart reads the store every 1 s.
- Not established: why nine 2026-09-22/23 guardian-review rollouts, whose lines carry `ordinal` today, were stored collapsed. Their data is repaired either way.
- No secret was handled.

## 4. Findings worth propagating
- [RESULT] Root cause of the missing Codex turns: current Codex builds write no `ordinal` on rollout lines, so every turn got the same key and the store kept one event per session. Proof on the real file: 26 `token_count` lines, 1 stored event. Across the store, 11 collapsed sessions were found and re-read.
- [RESULT] Codex month (local September, vendor-strip token count): 165,847,499 tokens in 1,492 events before the re-ingest, 176,203,422 in 1,655 right after, and 179,584,148 in 1,681 at 19:31 (Codex kept working).
- [RESULT] Live tail: 15-minute real run with the chat held open by Codex and no file touched. 13 turns, each in the store within about 2 s, including one after a 12-minute idle gap. Codex TODAY is 12,896,168 tokens in 161 events, exactly equal to the files' `last_token_usage` sum.
- [RESULT] Copilot in VS Code poll on the real 46.7 MB OTel file: about 490 ms per poll before, 0 to 24 ms after (one full read at start). The TODAY row is 230,585 tokens in 16 requests, equal to the file. The 0 Wilco saw at 18:36 came from the strip's first build right after a restart.
- [RESULT] `go vet` and `go test ./...` green. uicheck d0 to d22 green; d11 (tick cadence) failed twice under post-build load and passed on the third run.
- [STATE] Accepted residual risks, in STATUS: a double ingest of one append (harmless); a millisecond-scale race between the reset and a live ingest at startup; byte-offset keys shifting if Codex ever rewrites a rollout.

## 5. Hub-level decision (if any)
Nothing. All calls here are project-internal.

## 6. What the next hub read should update
`C:\ZND\10_holding\01_projects\burnmon.md` (current version and state), the BurnMon line in the hub STATUS or portfolio, `C:\ZND\wiki\hot.md`. No Mission Deck This Week item is named in the spec. None is claimed here.

## 7. Open flags for next session
- Commit and push alpha.8 (Wilco).
- d11 flakiness under load: watch whether it recurs outside post-build runs.
- Earlier open flags from the alpha.7 brief still stand, including the stale v0.3.2 header at the top of `C:\ZND\50_projects\burnmon\STATUS.md`.

## 8. Related files
- `C:\ZND\50_projects\burnmon\02_roadmap\2026-09-29_ws2_codex_live_tail.md`
- `C:\ZND\50_projects\burnmon\04_assets\hub_agent_update_2026-09-29_v0_4_0_alpha7_smooth_tick.md`
