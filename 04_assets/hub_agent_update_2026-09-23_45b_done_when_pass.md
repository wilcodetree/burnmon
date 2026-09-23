# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 (45B, buffer/Done-when session) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** report the section 5 Done-when verification pass, one real bug fixed, one open copy flag, and the real-time constraint on forecast scoring's week numbering.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry), then `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (checklist).
**Supersedes:** nothing.

## 1. Headline
BurnMon v0.2's build checklist (39B through 45A) was already fully ticked when this session started, so 45B ran the spec's section 5 Done-when list against the built exe instead; all six items pass, one live house-rule violation (a bare em dash placeholder) was found and fixed, and one prose flag from 45A was left open rather than silently rewritten. Committed, not pushed.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (repo root): `a0f8879` "fix: em-dash placeholder on Sessions tab footer; 45B Done-when verification pass".
- **On a branch, not merged:** nothing, this is on `main` directly, matching the project's usual flow.
- **Files touched** (full paths): `C:\ZND\projects\burnmon\internal\report\template.html` (one-line fix, line 983, em dash placeholder to `n/a` on the Sessions tab footer's average-cost cell), `C:\ZND\projects\burnmon\SESSION_LOG.md` (45B paragraph prepended), `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (45B ticked).
- **Untracked / outside a repo:** none; the exe rebuild (`burnmon.exe`, `burnmon-cli.exe`) and generated reports under `C:\ZND\projects\burnmon\reports\` are gitignored build/output artifacts, not new source.

## 3. What did NOT happen (and why)
Not pushed: the commit sits on local `main` only, per the session prompt's own instruction to stop before committing and print the command; Wilco ran the commit himself after review, push is still his call. Not re-run: the "Overview tab" leftover copy in the About page (flagged already in 45A) was left as-is, since rewriting it is a judgment call about what the About section should say now, not a Done-when failure. Session 46A's own explicit "no em dash in user-facing strings" sweep was not run in full this session; only the one instance the Done-when check surfaced was fixed, the other seven em dashes (all inside the `#overview` section, explicitly retained as dead unrendered markup since 40B/41A) were left for that sweep since they never reach a user. Forecast scoring did not reach week 45 and cannot yet, see section 4.

## 4. Findings worth propagating
- [RESULT] All six Done-when items from spec section 5 pass, verified directly against the real store at `C:\Users\WilcoDeTree\AppData\Local\burnmon\burnmon.db` (not a fixture): Now defaults to the `now` tab and showed 4 real running sessions (2 Claude Code, 2 Codex) with a re-prefill finding (cause "first turn after resume") on this very session's own turn 1; History's `{period: week, vendor: codex}` query returned 62,901,057 tokens across 30 sessions and 695 turns in one call; the v0.1 to v0.2 migration test already covers zero event loss and the real store's own migration (schema 4 to 5) confirmed it live; Hermes (4 events) and Copilot CLI (24 events) both resolve to honest display labels, never a raw agent key; the forecast slot shows its own locked gate text when empty; zero Dutch words found in the generated report HTML.
- [RESULT] One real bug fixed: `template.html` line 983 used a bare em dash as the Sessions tab footer's placeholder when no calls matched a filter, a live user-facing violation of the house no-em-dash rule; replaced with `n/a`.
- [STATE] Forecast scoring: real wall-clock ISO week is 2026-W39 (Sep 21 to 27), not week 45 or 46. The session prompts' week numbers (39 to 46) are build-order labels, not calendar weeks, since the whole 39A through 45A sequence ran inside two real days (2026-09-22 and 2026-09-23). This session's direct call into `forecast.Build` against the production store was the first real invocation ever made against it (the bound `bmForecast` function only fires from the live webview app, never from CLI report generation), and it correctly wrote week 39's plan (1,422,554,946 tokens); the forecast chart stays locked and scoring cannot reach a scored week, let alone week 45, until real weeks actually pass. This is a real-time constraint on the feature as designed, not a bug in it.
- [STATE] Open copy flag, not decided here: the About page still reads "the Overview tab" twice, referring to a tab removed in 40B. 45A flagged this first and chose not to fix it; this session made the same call for the same reason.

## 5. Hub-level decision (if any)
Nothing. This session's only fix (an em dash placeholder) and its one open flag (stale About copy) are project-internal, not a cross-project, pricing, positioning, or CIPHER-wall call.

## 6. What the next hub read should update
`C:\ZND\10_holding\01_projects\burnmon.md` (one-pager) can note the Done-when pass result and that scoring is real-time-gated to week 39 for now, not week 45/46 as the build-order table might suggest at a glance. No DEADLINES.md change; the v0.2 tag date (2026-11-14, per 46B's own prompt) already assumes real calendar weeks will need to pass for forecast scoring regardless, so this finding does not move any date, it just clarifies why the forecast slot will stay locked for a while yet.

## 7. Open flags for next session
- The "Overview tab" leftover copy in the About page (two occurrences) needs a decision: rewrite for the retained hidden widget, or delete the sentences outright. Belongs to whichever session next touches `template.html` prose, or an explicit ask to Wilco.
- Session 46A's own em-dash/Dutch/claudecost sweep still needs to run in full; the seven remaining em dashes in the dead `#overview` section are known and unfixed, by design, until that sweep decides what to do with the section as a whole.
- Forecast scoring has one real week (39) on the books now; it advances only with real wall-clock time, roughly one week per real week, not per session.
- Commit `a0f8879` is local only; push is still Wilco's call.

## 8. Related files
`C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` (section 5, Done when), `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (checklist, 45B now ticked), `C:\ZND\projects\burnmon\SESSION_LOG.md` (45B entry, top of file), `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-23_v0.2_44b_forecast_chart_gate.md` (prior forecast brief), `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-23_45a_readme_status_pass.md` (prior brief that first flagged the Overview tab copy).
