# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-22 - **Owner:** Wilco de Tree
**Project:** BurnMon (formerly claudecost)
**Purpose:** 42B (I2 part 2) shipped and tagged: two new insight rules, one line wired onto the Now card, v0.2.0-alpha.3 tagged.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry), then `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` section 2.3
**Supersedes:** `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.2_42A_insight_package.md` (extends it, does not replace it)

## 1. Headline
42B is shipped, merged to `main` and tagged: `internal/insight` gained `context-runway`
and `expensive-turn`, the runway line is on the Now page's session card, and the branch
was tagged `v0.2.0-alpha.3`. Not code-complete-on-branch, not pending: committed, tagged,
build green.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (branch `main`): `3a19d91` "feat: context
  runway and expensive-turn insight rules (I2 part 2, 42B)".
- **Tagged:** `v0.2.0-alpha.3` on `3a19d91`, annotated "I2 complete (re-prefill,
  compaction, context-runway, expensive-turn)".
- **Files touched** (full paths, all in the commit above):
  `C:\ZND\projects\burnmon\internal\insight\insight.go`,
  `C:\ZND\projects\burnmon\internal\insight\insight_test.go`,
  `C:\ZND\projects\burnmon\internal\live\live.go`,
  `C:\ZND\projects\burnmon\internal\report\template.html`,
  `C:\ZND\projects\burnmon\SESSION_LOG.md`,
  `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (42B ticked).
- Six prior sessions' handoff briefs (39B through 42A) were untracked in
  `C:\ZND\projects\burnmon\04_assets\` and were folded into this same commit at
  Wilco's choice, rather than left for a separate one: `hub_agent_update_2026-09-22_v0.2_39B_migrations_owner.md`,
  `..._40A_tool_calls.md`, `..._40B_five_tabs.md`, `..._41A_history_page.md`,
  `..._41B_vendor_strip_hermes.md`, `..._42A_insight_package.md`.
- **Untracked, not this session's work:** `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1.2_patch_spec.md`
  shows as modified in `git status` but predates this session; left alone and left out of
  the 42B commit.

## 3. What did NOT happen (and why)
Nothing was pushed to a remote (no remote push step was requested or run this session).
The v0.1.2 patch spec's own pending edit was not touched, resolved or committed: it is
someone else's in-flight change, not this session's to fold in. 43A (markers, ticker,
spike drawer on the Now chart, per the build order) has not started; `context-runway` and
`expensive-turn` findings exist in `bmLive`'s output now, but only the runway line is
rendered anywhere in the UI, by design (42B's own prompt said "keep everything else for
43A").

## 4. Findings worth propagating
- [RESULT] `go test ./... -count=1` and `.\build.ps1` both green after the change, run
  from `C:\ZND\projects\burnmon`.
- [RESULT] bmLive's per-poll cost with all four I2 rules (re-prefill, compaction,
  context-runway, expensive-turn), measured against Wilco's real local BurnMon store
  (built this session from his actual Claude Code transcripts via `burnmon-cli.exe`):
  1,858 sessions, 51,563 events, 6,295 findings total. Best of 3 full passes: 7.7
  microseconds average per session, 2.05 ms worst case. Comfortably inside the 5 ms
  per-session budget the existing synthetic-marathon test already asserts. Full number and
  method are in `C:\ZND\projects\burnmon\SESSION_LOG.md`'s 42B entry.
- [RESULT] Open choice resolved with Wilco directly this session (recorded in the chat,
  not re-derivable from the diff alone): the spec's "95th percentile of the session's turn
  cost or tokens" for `expensive-turn` was ambiguous; Wilco chose tokens over cost, since
  it needs no `pricing.Config` model-family lookup and matches `re-prefill`'s own
  token-threshold shape.
- [STATE] 43A (I3: markers, ticker, drawer, Sessions tab findings display) is next per the
  build order table, week 43, not yet started.

## 5. Hub-level decision (if any)
Nothing. The tokens-vs-cost choice in section 4 is project-internal (BurnMon's own metric
definition), not a cross-project, pricing, positioning, or CIPHER-wall call.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` (hub one-pager): 42B done, `v0.2.0-alpha.3`
  tagged, next up 43A.
- `C:\ZND\10_holding\02_roadmap\roadmap.md` if it tracks BurnMon by build-order week: week
  42 both sessions (42A, 42B) now closed.
- No DEADLINES.md change expected: the v0.2 release date (2026-11-14) and the two-session-
  a-week cadence are unaffected.

## 7. Open flags for next session
43A needs a Claude Code and a Codex session running live before starting (per the session
prompts file's own note at line 161), to check markers appear within 2 seconds of a turn.
No remote push has happened for this branch; confirm with Wilco whether `v0.2.0-alpha.3`
and `main` should be pushed, and to where.

## 8. Related files
`C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` (section 2.3, I1-I3),
`C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (42B prompt, line
68-71; checklist line 147),
`C:\ZND\projects\burnmon\SESSION_LOG.md` (42B entry, top of file),
`C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.2_42A_insight_package.md`
(the 42A brief this one extends).
