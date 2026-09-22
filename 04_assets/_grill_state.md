# Grill state: vendor-agnostic token and cost monitor

Skill: grill-me-product. Started 2026-09-22 in the hub; moved to `C:\ZND\projects\burnmon\04_assets\` the same day (plan in `..\02_roadmap\`). Hub keeps only `01_projects\burnmon.md`, the portfolio row, decisions, DEADLINES.

## Phase 0, fixed on 2026-09-22

- Kind: product.
- Subject: a standalone, local-first live token and cost monitor for developers who use
  more than one AI coding agent (Claude Code, OpenAI Codex, GitHub Copilot, Hermes), via
  VS Code and CLIs. Successor of `claudecost`, which is Claude-only.
- For whom: individual developers first (per developer, local only). Later: teams, and
  Talon as a possible customer. Test group: Wilco plus the Valona Amsterdam development
  team (ASSUMED: Valona has not agreed yet).
- Owner: ZeroNonsense.dev product. Outputs go to the ZND hub. No Valona material enters
  `C:\ZND` (wall `valona`).
- Platforms: Windows, macOS, Linux, WSL.
- Dates fixed: none. Money fixed: none.
- Output language: English.

## Phase reached

Phase 3 Part 2 appended and phase 4 (CodeBurn scan) done, 2026-09-22. Phase 5 complete. Phase 6 plan written (`02_roadmap\2026-09-22_burnmon_plan.md`) and phase L written. Phase 7 complete 2026-09-22: decisions.md block, portfolio row plus BurnRate note, 01_projects/burnmon.md, five DEADLINES rows, roadmap pointer, SESSION_LOG paragraph. Skipped: nothing. Wilco's steps: fork repo, registrar check, Valona ask.

## Files written

- `_grill_state.md` (this file)
- `2026-09-22_token_monitor_sources_and_facts.md` (phase 1 source map, phase 2 facts list)
- `2026-09-22_token_monitor_architecture.md` (phase 3, Parts 1 and 2)
- `2026-09-22_external_scan_codeburn.md` (phase 4)
- `02_roadmap\2026-09-22_burnmon_plan.md` (phase 6)
- `2026-09-22_burnmon_licence_and_ip.md` (phase L)
- `01_projects\burnmon.md`, rows in `portfolio.md`, `DEADLINES.md`, `roadmap.md`, block in `03_logs\decisions.md`, paragraph in `SESSION_LOG.md` (phase 7)
- `handovers\2026-09-22_burnmon_week39_handoff.md` (week 39 Claude Code handoff)
- `2026-09-22_burnmon_now_page_features.md` (Now page, live, context, dev/business switch; decided)
- `..\02_roadmap\2026-09-22_v0.1_spec.md` and `..\02_roadmap\2026-09-22_v0.1_session_prompt.md` (build handoff, Sonnet, three steps)

## Decisions taken 2026-09-22 (phase 5)

1. Differentiator order: coverage parity Windows-first (floor), forecast, client attribution; all in v1.
2. Name: BurnMon.
3. Hours v1: session duration from transcripts, "active time". BurnRate time-log join v1.1.
4. Pilot: Valona Amsterdam team from the first build (team-lead yes pending, ASSUMED).
5. macOS: ship untested via GitHub Actions runner, labelled.
6. Licence: MIT core, paid team line (merge, client reports) later.
7. Start now, one session a week beside Siteoffice; v0.1 target 2026-10-17.
8. Now page (live sessions, context gauges, live burn chart) enters v0.1; dev mode default, Talon config sets business.

Still open: domain registration (VERIFY), Valona team-lead approval (A1).

## Assumptions register

| # | Assumption | Status | Confirms or kills it |
|---|---|---|---|
| A1 | Valona Amsterdam dev team is available as test group | ASSUMED | Wilco asks the team lead |
| A2 | Every listed agent leaves a local, readable usage trail | CONFIRMED (Claude, Codex, Hermes HELD; Copilot layout unstable, VERIFY current install) | Phase 2 research |
| A3 | Copilot CLI current build stores tokens in `data.db` | VERIFY | Inspect one Valona laptop with a current Copilot CLI |
| A4 | Subscription pricing figures (Claude Pro/Max) | VERIFY | Read claude.com/pricing |

## v0.2 grill, 2026-09-22 (release scoping, one question at a time, Fable)

Input: Wilco's asks (Now default, vendor tokens day/week/month, merge Months/Weeks/Days,
English only, smooth chart, Codex card only after Refresh), roadmap v0.2 items, the
"explain the burn" proposals. Thirteen questions, all answered the same day:
1 keep everything in one v0.2 release on 2026-11-14; 2 v0.1.2 patch first; 3 tabs Now,
History, Sessions, Tools, About; 4 vendor strip under the cards, tokens only; 5 dev and
business switch to v0.3 complete; 6 forecast stays with a visible gate; 7 Wilco records
Hermes and Copilot CLI fixtures locally (Microsoft Copilot out of scope, no local trail);
8 Copilot CLI only, one-hour OTel check for VS Code Copilot; 9 `internal/insight`,
computed on the fly, marker + ticker + drawer + Sessions tab; 10 versioned additive
migrations first; 11 two sessions a week, no slip order; 12 fixed 30-minute chart,
time axis sliding 2 s per poll, damped Y max; 13 owner split (Valona / ZND / personal by
path rule, empty by default) into v0.2 as the light client map, with the wall rule for
anything published.
New goal recorded: BurnMon plus modelwatch to understand Valona and ZND token usage,
learn from it, possibly a LinkedIn post; content work once both have weeks of data.
Files: `..\02_roadmap\2026-09-22_v0.1.2_patch_spec.md`, `..\02_roadmap\2026-09-22_v0.2_spec.md`,
roadmap and DEADLINES updated, hub roadmap allocation, decisions.md, DEADLINES rows.
