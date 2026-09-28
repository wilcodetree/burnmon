# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-26 - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** close the WS1 to WS3 plus BurnMon Dev workstream: WS2 merged into `main`, tags
corrected, branch and worktree removed, alpha.3 planned for later.
**Read order:** this file, then the same-day briefs it supersedes in part:
`hub_agent_update_2026-09-26_ws2_ws3_planning.md`,
`hub_agent_update_2026-09-26_ws2_phase5_verify_release.md`,
`hub_agent_update_2026-09-26_ws3_shared_ingest_performance.md`,
`hub_agent_update_2026-09-26_ws2_performance_patch.md` (all in this folder).
**Supersedes:** the "not merged, not pushed, not tagged" state in those four briefs.

## 1. Headline

BurnMon Dev (`burnmon-dev.exe`, v0.4.0-alpha.2) is merged into `main` and pushed; `burnmon.exe`
is v0.3.2. The workstream is closed. v0.4.0-alpha.3 is planned for later, no date.

## 2. What changed on disk (read from Wilco's pasted git output, 2026-09-26)

- `main` on origin: `69a071e` v0.3.2 (WS3), `45e05f8` docs, `a8a4865` roadmap,
  `f7f1c26` merge of `burnmon-dev` (`--no-ff`, no conflicts). After the merge on `main`:
  `go vet ./...` clean, `go test ./... -count=1` all packages ok, `.\build.ps1` built
  `burnmon-cli.exe`, `burnmon.exe` and `burnmon-dev.exe`.
- Tags on origin: `v0.3.2` (`69a071e`), `v0.4.0-alpha.1` (`0089baf`, phase 5), `v0.4.0-alpha.2`
  (`c236162`, performance patch).
- `v0.4.0-alpha.1` was wrong on origin: an older local tag on `c67a7af` (phase 2b) blocked the
  new tag, so the first push sent the old one. Repointed to `0089baf` and force-pushed.
- Branch `burnmon-dev` deleted locally and on origin; worktree
  `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev` removed.
- `C:\ZND\projects\burnmon\02_roadmap\roadmap.md`: item 8 (WS1 to WS3 plus BurnMon Dev, done)
  and item 9 (v0.4.0-alpha.3, later) added.

## 3. What did NOT happen (and why)

- v0.4.0-alpha.3 not run: Wilco accepted alpha.2. Items in
  `C:\ZND\projects\burnmon\02_roadmap\2026-09-26_parked_after_alpha2.md`.
- Hub `SESSION_LOG.md` and the BurnMon one-pager not edited by the hub chat: the Cowork shell
  was down (Windows update blocks the workspace) and Edit truncates large files on the hub
  mount. The next hub-update pass propagates this brief.

## 4. Findings worth propagating

- [RESULT] `burnmon.exe` v0.3.2: 13,541 to 371 handles, about 957 MB to 87 MB peak RAM, about
  0.2 percent average CPU (WS3 report).
- [RESULT] `burnmon-dev.exe` v0.4.0-alpha.2, 10 min active with `burnmon.exe` running: Go
  process 0.28 percent average CPU, 195.4 MB peak; WebView2 0.51 percent, 671.1 MB. Minimized:
  Go 297.2 MB peak, above the 250 MB target (parked for alpha.3).
- [RESULT] All day boundaries and displayed times are local time; storage stays UTC; the
  Copilot credit reset stays UTC (GitHub's billing boundary).
- [STATE] Microsoft To Do sign-in done by Wilco on his own account, 2026-09-26.
- [STATE] Leftovers seen in `git worktree list` and `git branch -a`, not touched: an old
  worktree `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-v0.1-step1` (branch
  `worktree-burnmon-v0.1-step1`, `fc8aff8`), and a stale `mirror/burnmon-dev` ref from remote
  `mirror` (`ssh://cipher/~/znd-mirrors/burnmon.git`). Unverified whether the mirror is updated
  by a job.

## 5. Hub-level decision

Nothing. Project-internal.

## 6. What the next hub read should update

`C:\ZND\10_holding\01_projects\burnmon.md` (v0.3.2 and v0.4.0-alpha.2 on `main`; clear the
2026-09-26 "dangling alpha.1 tag" and "WS3 not started" flags), `C:\ZND\10_holding\01_projects\portfolio.md`
(BurnMon line), `C:\ZND\10_holding\SESSION_LOG.md`.

Update, same day: both leftovers fixed by Wilco. `fc8aff8` confirmed merged into `main`,
worktree `burnmon-v0.1-step1` and its branch removed; `git push --mirror mirror` deleted
`burnmon-dev` and `worktree-burnmon-v0.1-step1` on the mirror and brought it to `f7f1c26` plus
all three tags. `git worktree list` now shows only `C:\ZND\projects\burnmon`.

## 7. Open flags for next session

- Nightly mirror (`ZND-NightlyMirror`, `C:\ZND\10_holding\04_engineering\backup\mirror_repos.ps1`):
  missed 2026-09-24 to 2026-09-26. Every hidden run hung in a plain `ssh` call (never in
  `git push`) until the 1-hour limit; visible runs passed. SSH timeouts alone (hub commit
  `ea9fa17f`) did not fix it. `ssh -n` on the two plain calls (hub commit `be633431`) did: the
  scheduled run on 2026-09-27 08:33 finished in about 1 minute, `LastTaskResult 0`, 20 pushed,
  0 errors. Cause (stdin never closing without a console) inferred, not proven. Confirm the
  2026-09-27 23:30 run.
- v0.4.0-alpha.3, later.
