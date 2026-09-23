# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 (46B, tag v0.2.0) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** report v0.2.0 shipped: version constants set, docs and roadmap updated,
commit and tag commands printed for Wilco.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry, 46B),
then the 46A brief (`hub_agent_update_2026-09-23_46a_rc_done_when.md`) for the rc.1
Done-when/VERIFY evidence this tag carries forward unchanged.
**Supersedes:** nothing. Filename uses the real date (2026-09-23), not the session
prompt's literal `2026-11-14` placeholder, matching the convention 46A already set in this
folder and the house date-prefix rule (`C:\ZND\AGENTS.md`).

## 1. Headline

BurnMon v0.2.0 shipped. 46A's release-candidate pass (tag `v0.2.0-rc.1`, commit `541c6f2`)
found zero failing Done-when or VERIFY items, so 46B had nothing to fix: the session
prompt's own instruction ("fix the fails listed for rc.1 if Wilco confirmed them") had no
fails to apply to. This session set both version constants to `0.2.0`, updated STATUS.md,
DEADLINES.md and the project `roadmap.md`, and prepared the tag. Not tagged or pushed;
commands below, Wilco's own step.

## 2. What changed on disk

- `cmd\burnmon-cli\main.go:35`: `version` constant `0.1.2` to `0.2.0`.
- `cmd\burnmon\main.go:47`: `version` constant `0.1.2` to `0.2.0`.
- `STATUS.md`: header and "Next" section rewritten for the shipped state; "Next" now points
  at v0.3 instead of the release-candidate week.
- `DEADLINES.md`: the 2026-11-14 v0.2 row marked "DONE 2026-09-23, tagged `v0.2.0`".
- `02_roadmap\roadmap.md`: item 3 (v0.2) marked done with the 46A rc pass result; item 4
  (v0.3) marked "next".
- `SESSION_LOG.md`: one paragraph prepended (46B).
- `02_roadmap\2026-09-22_v0.2_session_prompts.md`: 46B ticked.
- This brief (new).
- No template or adapter code changed; no source fix was needed.

## 3. What did NOT happen (and why)

Not committed or tagged: per the session prompt, stop before tagging, print the commands,
Wilco's own call. Not fixed: 46A's Done-when/VERIFY pass (spec section 5 and 6) found no
failing item, so nothing here was a fix target; this matches the SESSION_LOG 46A entry
verbatim ("No failure found this pass, so nothing was fixed"). Not re-litigated: the
"Overview tab" leftover copy in About (flagged 45A, still open at 45B and 46A) stays open;
version/docs/roadmap only, per this session's own scope. Not touched: `C:\dev\Work`.

## 4. Findings worth propagating

- **[RESULT]** No fails carried into the tag. 46A ran all six spec-section-5 Done-when
  items and all five section-6 VERIFY items against the real store and real exe; all six
  passed, all five VERIFY items resolved to a stated assumption and UI label (or an
  explicit "not labeled anywhere" where nothing applies it). 46B's own instruction to "fix
  the fails listed for rc.1" therefore had an empty list to work from.
- **[RESULT]** `go test ./... -count=1` and `.\build.ps1` both green after the version bump.
- **[STATE]** v0.3 scope carried forward unchanged from the spec's section 3: per-vendor
  cost and credits, the dev/business switch (moved from v0.2, ships complete in v0.3), the
  full client map (active time, export, merge), macOS and Linux builds. Copilot VS Code
  OTel wiring is a live v0.3 scope call (confirmed technically possible in 43B, not built).
- **[STATE]** VERIFY items still open in the sense of "a v0.3 decision, not a v0.2 defect":
  Copilot VS Code OTel wiring (confirmed yes, not wired into any adapter or labeled in the
  UI, per 46A). All other VERIFY items from spec section 6 (Claude Code cache TTL,
  auto-compact threshold, Hermes context window, Copilot CLI `data.db` layout, Codex
  subagent structure) are resolved, not open, per 46A's pass.
- **[STATE]** Forecast scoring state, unchanged since 45B/46A: real wall-clock week is
  still 2026-W39, `scored_weeks: 0`, the gate stays locked until a real ISO week actually
  elapses. This is a real-time constraint, not a defect; scoring advances only with
  calendar time passing on Wilco's own laptop.

## 5. Hub-level decision (if any)

None. v0.2.0 shipping is a scheduled release per the 2026-09-22 v0.2 grill, not a new scope
or pricing decision; nothing here crosses the CIPHER wall or touches Valona.

## 6. What the next hub read should update

`C:\ZND\10_holding\01_projects\burnmon.md` and `02_roadmap\roadmap.md` (hub-level) can mark
BurnMon v0.2 as shipped 2026-09-23, ahead of the 2026-11-14 target date carried in
`DEADLINES.md` and the hub roadmap's own BurnMon block. Next milestone: v0.3, 2026-12-12.

## 7. Open flags for next session

- Commit, tag (`v0.2.0`) and push are Wilco's manual step; commands below.
- The "Overview tab" leftover copy in About (flagged 45A, open through 45B and 46A) is
  still open, a judgment call for whoever next touches `template.html` prose.
- Copilot VS Code OTel wiring: real v0.3 scope decision, confirmed technically possible,
  not built.
- Forecast scoring has one real week (39) on the books; advances only with real wall-clock
  time, unaffected by the v0.2 tag.
- `04_assets\hub_agent_update_2026-09-23_45b_done_when_pass.md` is still untracked in git
  as of 46A's check; not re-checked this session, still someone else's commit to fold in.

## 8. Related files

`C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` (sections 5 and 6),
`C:\ZND\projects\burnmon\SESSION_LOG.md` (46B entry, top of file),
`C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (checklist, 46B now
ticked), `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-23_46a_rc_done_when.md`
(rc.1 Done-when/VERIFY evidence this tag carries).

## Commands for Wilco (not run this session)

```powershell
# runs in: PowerShell on the laptop, cwd C:\ZND\projects\burnmon
git add -A
git commit -m "release: BurnMon v0.2.0, version constants, STATUS/DEADLINES/roadmap updated (46B)"
git tag v0.2.0
git push origin main --tags
```
