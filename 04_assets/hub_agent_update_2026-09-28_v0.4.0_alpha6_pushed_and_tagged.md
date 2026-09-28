# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-28 - **Owner:** Wilco de Tree
**Project:** BurnRate (BurnMon)
**Purpose:** v0.4.0-alpha.6 (session colour locked grey before its vendor was known, the fix) is now pushed and tagged on origin, not just committed.
**Read order:** this file, `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-28_v0.4.0_alpha6_colour_cache_fix.md` (the code-complete brief this one supersedes for shipped state), `C:\ZND\projects\burnmon\SESSION_LOG.md` top entry.
**Supersedes:** `hub_agent_update_2026-09-28_v0.4.0_alpha6_colour_cache_fix.md`'s own sections 1, 3 and 7 (their "not pushed, not tagged" state) - everything else in that brief still stands.

## 1. Headline
`v0.4.0-alpha.6` is shipped: both commits (`789a40d` the fix, `da4a1ee` the hub brief docs
commit) are on `origin/main`, and the `v0.4.0-alpha.6` tag is on origin, pointing at `789a40d`
(the code commit). Confirmed by re-querying origin directly this session (`git ls-remote`), not
by re-reading the earlier report. Wilco also committed, in the same push, a separate untracked
file this session had correctly left alone: `04_assets\hub_agent_update_2026-09-28_ws2_alpha5_pushed_and_tagged.md`
(`1e4f4aa`, his own commit, the alpha.5 ship-confirmation brief from an earlier session).

## 2. What changed on disk
- No new code this pass - this brief only records that the prior session's own two commits
  and one tag reached `origin` (`https://github.com/wilcodetree/burnmon.git`), plus one
  additional commit Wilco made himself in the same push.
- **Verified on origin, this session, live:**
  - `git ls-remote origin main` -> `1e4f4aa2c06105b8b2e92245d821be2dd35a4402` (matches local
    `HEAD`/`main` exactly, and `git branch -vv` shows `main` tracking `origin/main` at that
    same SHA with nothing ahead or behind).
  - `git ls-remote --tags origin` -> `refs/tags/v0.4.0-alpha.6` at
    `789a40dcadb9b0aa143bd0bb3fe2cadfc4916f85` (the code commit that bumped the version
    string, not the later docs commits - correct).
  - `git rev-parse main origin/main` returned the identical hash twice.
- Working tree still shows `go.mod` modified (pre-existing, line-ending-only, left out of
  every commit this project's own convention has made so far).

## 3. What did NOT happen (and why)
- **The `mirror` remote was not touched this pass**, per this project's own remote policy
  (push and tag to `origin` only). Not re-verified live this pass; same open item the prior
  alpha.5 ship-confirmation brief already flagged, still unconfirmed with Wilco.
- **No new code, no new tests, no new review this pass.** This brief only confirms the ship
  state of already-committed work; see
  `hub_agent_update_2026-09-28_v0.4.0_alpha6_colour_cache_fix.md` for the actual fix's own
  findings (root cause, the Go/JS fix, the independent Opus review, the full verify pass).

## 4. Findings worth propagating
- [RESULT] `origin/main` HEAD = `1e4f4aa2c06105b8b2e92245d821be2dd35a4402`, matching local
  `main` exactly (`git rev-parse main origin/main` returned the identical hash twice, this
  session, live).
- [RESULT] `origin` tag `v0.4.0-alpha.6` exists and points at
  `789a40dcadb9b0aa143bd0bb3fe2cadfc4916f85` (the fix commit, correct - version bump landed
  there), confirmed via `git ls-remote --tags origin`, a direct query of the remote.

## 5. Hub-level decision
Nothing - a push/tag confirmation, not a decision.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` (the hub one-pager) - update from "v0.4.0-alpha.6,
  committed not pushed" (if it was ever set that way from the prior brief) to "v0.4.0-alpha.6,
  pushed and tagged on origin".

## 7. Open flags for next session
- Whether the `mirror` remote needs `v0.4.0-alpha.6` too, or whether this project's own
  remote policy means it deliberately stays behind - still not checked, same open item
  carried over from the alpha.5 ship-confirmation brief.

## 8. Related files
- `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-28_v0.4.0_alpha6_colour_cache_fix.md`
- `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-28_ws2_alpha5_pushed_and_tagged.md`
- `C:\ZND\projects\burnmon\SESSION_LOG.md`
