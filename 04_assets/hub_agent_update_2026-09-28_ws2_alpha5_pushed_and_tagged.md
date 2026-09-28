# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-28 - **Owner:** Wilco de Tree
**Project:** BurnRate (BurnMon)
**Purpose:** v0.4.0-alpha.5 (gap-break threshold fix) is now pushed and tagged on origin, not just committed.
**Read order:** this file, `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-28_ws2_follow_up_v0_4_0_alpha5.md` (the code-complete brief this one supersedes for shipped state), `SESSION_LOG.md` top entry.
**Supersedes:** `hub_agent_update_2026-09-28_ws2_follow_up_v0_4_0_alpha5.md`'s own sections 1, 3 and 7 (their "not pushed, not tagged" state) - everything else in that brief still stands.

## 1. Headline
`v0.4.0-alpha.5` is shipped: both commits (`8d64ae9` code, `74c5a4e` docs) are on `origin/main`,
and the `v0.4.0-alpha.5` tag is on origin, pointing at `8d64ae9`. Confirmed by re-querying
origin directly this session (`git ls-remote`), not by re-reading the earlier report.

## 2. What changed on disk
- No new file changes this pass - this brief only records that the prior session's own two
  commits and one tag reached `origin` (`https://github.com/wilcodetree/burnmon.git`).
- **Verified on origin, this session, live:**
  - `git ls-remote origin main` -> `74c5a4e91ac96b6408ebdad093dff87e529c6af4` (matches local
    `HEAD` exactly).
  - `git ls-remote --tags origin` -> `refs/tags/v0.4.0-alpha.5` at
    `8d64ae96d8b397a52eb4cfb549fa3e5278ac2c15` (the code commit, not the docs commit after it -
    correct, the version bump landed in the code commit).
  - Local `main` tracks `origin/main` at the same SHA (`git branch -vv`).
- Working tree still shows `go.mod` modified (pre-existing, line-ending-only, left out of both
  commits as intended).

## 3. What did NOT happen (and why)
- **The `mirror` remote (`ssh://cipher/~/znd-mirrors/burnmon.git`) was not touched.** House
  rule for this project: push and tag to `origin` only. Not re-verified live this pass (no
  read access assumed reliable over that SSH remote from this session); flagged as an open
  item below if that ever needs confirming.
- **No new code, no new tests, no new review this pass.** This brief only confirms the ship
  state of already-committed work; see the prior brief for the actual fix's own findings.

## 4. Findings worth propagating
- [RESULT] `origin/main` HEAD = `74c5a4e91ac96b6408ebdad093dff87e529c6af4`, matching local
  `main` exactly (`git rev-parse main origin/main` returned the identical hash twice).
- [RESULT] `origin` tag `v0.4.0-alpha.5` exists and points at `8d64ae96d8b397a52eb4cfb549fa3e5278ac2c15`
  (the code commit that bumped the version string), confirmed via `git ls-remote --tags origin`,
  a direct query of the remote, not a local-only `git tag` listing.

## 5. Hub-level decision
Nothing - a push/tag confirmation, not a decision.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` (the hub one-pager) - update from "v0.4.0-alpha.5,
  committed not pushed" to "v0.4.0-alpha.5, pushed and tagged on origin".

## 7. Open flags for next session
- Whether the `mirror` remote (`ssh://cipher/~/znd-mirrors/burnmon.git`) needs `v0.4.0-alpha.5`
  too, or whether this project's own remote policy means it deliberately stays behind - not
  checked this pass, worth confirming with Wilco once.

## 8. Related files
- `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-28_ws2_follow_up_v0_4_0_alpha5.md`
- `C:\ZND\projects\burnmon\SESSION_LOG.md`
