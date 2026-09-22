---
type: plan
title: BurnMon, the vendor-agnostic token and cost monitor, plan
created: 2026-09-22
tags: [burnmon, claudecost, burnrate, token-monitor, product]
status: active
---

# BurnMon plan

Grill trail: `C:\ZND\10_holding\04_assets\_grill_state.md`, architecture note
`2026-09-22_token_monitor_architecture.md`, CodeBurn scan
`2026-09-22_external_scan_codeburn.md`, licence pass `2026-09-22_burnmon_licence_and_ip.md`,
all in `04_assets\`. Now page and live features: `04_assets\2026-09-22_burnmon_now_page_features.md` (decided 2026-09-22).

## 0. TLDR

BurnMon is the successor of claudecost: one portable executable that reads Claude Code,
Codex, Copilot CLI, Hermes and Cowork trails from the developer's own disk into a local
SQLite store, prices them three ways, forecasts the month, and attributes tokens and active
time to projects and clients. Built one session a week beside Siteoffice from 2026-09-22.
v0.1 (Windows, Claude and Codex adapters, SQLite, coverage floor, the live Now page) by 2026-10-17; the
Valona Amsterdam team tests from that build (team-lead yes pending). v0.2 (Copilot CLI,
Hermes, forecast) by 2026-11-14, v0.3 (client attribution, export and merge, macOS and
Linux builds) by 2026-12-12. Decision on 2026-12-19: continue to a team line, keep as a
free tool, or stop.

## 1. Hypotheses

| # | Hypothesis | Measure | Kill signal |
|---|---|---|---|
| H1 | Developers on mixed agents want one local number and will run a tool that never phones home | At least 5 of the Valona pilot developers still run BurnMon in week 4 of the pilot | Fewer than 3 after week 4, or "I use ccusage" from the majority |
| H2 | A forecast with its own error band changes behaviour | At least 2 pilot developers report a change (model switch, compact earlier, plan change) citing the forecast | Nobody cites it after 6 weeks, or the error band stays above 30% in month 2 |
| H3 | Client attribution is worth money to a small firm | Talon uses a BurnMon export as one of the three PoC closing numbers, and one of Talon or an advisory client asks for the team line | No firm asks by 2026-12-19 |

## 2. Scope

In (v0.1 to v0.3): adapters for Claude Code and Cowork (carried from claudecost), Codex,
Copilot CLI, Hermes; the five-class token schema; SQLite store with per-file cursors;
three price books (API list per vendor, Copilot AI credits, subscription share) shipped
as dated JSON; dedup by vendor plus request id; dashboard with vendor, month, week, day,
session, project and client views; the Now page (running sessions with context fill gauge, cache clock, live burn chart per minute, turn ticker, plan-window strip where written to disk, forecast chart under it); dev and business switch, dev default, Talon config sets business; forecast card with error band and track record;
active time per session from transcript timestamps; `scan`, `report`, `export`, `merge`,
`forecast`, `price-check` CLI commands; portable exe for Windows, macOS (untested,
labelled) and Linux; WSL discovery from Windows; MIT licence; README in EN.

Out, said out loud: any network call from the app; accounts; cloud sync; undocumented
vendor endpoints; org APIs; VS Code Copilot tokens without OTel; OTel receiver (v1.1);
BurnRate time-log join (v1.1); hooks; budget guard; Gemini, Cursor, OpenCode adapters;
per-person comparison; code signing; NL README (v1.1); a paid tier before a firm asks.

## 3. Roles

- Wilco: product owner, builder, first user, price-book keeper (15 minutes per vendor per month).
- Valona Amsterdam team lead: says yes or no to the pilot (RELAYED nothing yet); no data or
  budget from Valona.
- Valona pilot developers: install the public build, report weekly in one message.
- Talon (Martijn, Bart, Jeroen): receive BurnMon through the Groundwork Kit slot claudecost
  holds today; supply one export per laptop for the PoC closing number.
- Claude sessions: Fable or Opus for adapter design and forecast method, Sonnet for
  adapter mechanics and the dashboard, Haiku or a script for price-book refresh.

## 4. Timeline

Availability caveat: one session a week, Siteoffice sprint 2 runs to 2026-10-11 and the
estate rename window (2026-09-28 to 10-11) moves `C:\ZND\projects\claudecost`; a slipped
week slips every row below by a week, and the plan says so rather than compressing.

| When | What | Done when |
|---|---|---|
| Week 39 (2026-09-22) | Grill closed, hub propagated, repo `burnmon` forked from claudecost, name reserved | this plan is in the hub; repo builds `burnmon.exe` that equals claudecost today |
| Week 40 (to 10-03) | Schema and SQLite store; Claude adapter moved onto the schema | claudecost numbers reproduce from the store byte for byte |
| Week 41 (to 10-10) | Codex adapter (cumulative `token_count`, `turn_context` model, `rate_limits` sample) | Wilco's own Codex sessions priced and deduped |
| Week 42 (to 10-17) | v0.1: Now page (file watchers, running Claude and Codex sessions, context fill, live burn chart), vendor column, price books as dated JSON, `price-check` | Now page shows Wilco's running Claude and Codex sessions with context fill and a live chart; exe handed to the Valona team lead. Cache clock and ticker slip to v0.2 if the week is short |
| Week 43 (to 10-24) | Pilot start (if yes); Copilot CLI adapter against a real pilot install (A3) | first pilot report received; Copilot tokens visible or the gap named |
| Weeks 44 to 46 (to 11-14) | Hermes adapter (5 s poll); forecast live line with error band and track record; re-prefill and compaction events; Copilot rows with the honest label; dev and business switch; v0.2 | forecast shown only once it has one closed week to score against; the switch flips every number on the Now page |
| Weeks 47 to 50 (to 12-12) | Client map, active time, `export` and `merge`, macOS and Linux builds from Actions; v0.3 | Talon receives v0.3 through the Groundwork Kit; one merged report exists |
| 2026-12-19 | Decision | H1 to H3 scored in the hub, one decision block written |

## 5. Terms

Price: free, MIT, for individuals and for the pilot. Period: pilot 2026-10-19 to
2026-12-18 (8 weeks after a yes). Invoicing: none. Included support: one weekly message
thread with the pilot; no SLA. Data location: the developer's own laptop; BurnMon writes
only under its own local folder; no export leaves a laptop unless the developer runs
`export`. Ending: the pilot ends by date; the developer deletes the folder and the tool is
gone. Decision at the end: continue toward a paid team line (`merge`, client reports,
support), keep as a free tool feeding talks, or stop and hand the adapters to ccusage as
PRs.

## 6. Risks

| Risk | From | Answered by |
|---|---|---|
| Copilot storage changes again during the build | facts list C, scan section 3 | Week 43 row: adapter built against a live pilot install, docs of CodeBurn as the map; A3 |
| Forecast is wrong once and never trusted | note 4.2 | Week 44 to 46 "done when": not shown without a scored week; error band always visible |
| Valona says no or says nothing | note section 7, A1 | Week 42 hands the build to the team lead; if no answer by 10-24, Wilco and Talon are the pilot and H1 is rescored on Talon |
| CodeBurn ships client attribution first | scan section 4 (Teams waitlist) | H3 is about a paying ask, not a feature; the entity and EU posture do not copy |
| One session a week is not enough | roadmap decision 7 | rows slip a week each, plan says so; kill check at week 46: no v0.2, then park |
| macOS build broken and nobody notices | decision 5 | README label; first Mac pilot user is asked in week 43 |
| The estate rename moves claudecost mid-fork | DEADLINES 09-28 to 10-11 | fork in week 39, before the window |
| Price books go stale | note section 6 | `price-check` prints dates; monthly 15-minute task in Wilco's calendar |
| New risk (2026-09-22, features note): the Now page pulls v0.1 from "claudecost plus Codex" to a new first page on one session a week | `04_assets/2026-09-22_burnmon_now_page_features.md` section 6 | Week 42 "done when" is the minimum (sessions, gauge, chart); ticker and cache clock slip to v0.2 |
| New risk: BurnMon and claudecost both alive at Talon | note 4.1 | Week 47 to 50 replaces claudecost in the Kit; claudecost README points at BurnMon |

## 7. Assumptions register

| # | Assumption | Source | Confirms or kills it | Status |
|---|---|---|---|---|
| A1 | Valona Amsterdam team is available as pilot | Wilco, phase 0 | team-lead answer by 2026-10-24 | ASSUMED |
| A2 | Every agent leaves a readable local trail | facts list A to D | done | CONFIRMED (Copilot layout VERIFY, see A3) |
| A3 | Current Copilot CLI stores tokens in `data.db` | tokenuse docs, ccusage #1174 | inspect one pilot install in week 43 | VERIFY |
| A4 | Claude Pro/Max plan prices as listed | aggregators | read claude.com/pricing in week 42 | VERIFY |
| A5 | 6 to 10 sessions reach v0.3 | note section 7 | week 46 kill check | ASSUMED |
| A6 | burnmon.com / .dev / .app are free | sandbox probe gave no answer | registrar lookup this week | VERIFY |
| A7 | Talon accepts BurnMon in place of claudecost in the Kit | Talon PoC plan :50 | ask Martijn when v0.3 exists | ASSUMED |
| A8 | Transcript timestamps give usable active time | Claude and Codex JSONL have per-message timestamps (HELD) | week 47 spot check against Wilco's own time log | ASSUMED |

## 8. Why not the nearest alternatives

CodeBurn (scan): free, 41 agents, forecast, project and PR attribution, quota rings. It has
no client dimension, no time, no archive past each tool's retention, telemetry on by
default outside the EU, and no entity to contract with. Using it would give the pilot most
of v0.2 today and none of H3; contributing to it would put ZeroNonsense.dev's client
dimension into a US-law project with an unnamed owner.

tokscale: CLI plus a public leaderboard with GitHub login, the opposite of local-only. No
desktop app, no client dimension.

tokenuse: the closest shape (Rust desktop, four tools, durable `archive.db`, no telemetry),
33 stars, one maintainer, no client or forecast. If BurnMon stops, its adapters or the
archive idea are where a contribution would go.

ccusage: 18.7k stars, the reference CLI; its adapters are the parsing benchmark for ours.
No desktop, no client, no forecast.

## 9. What this plan changes elsewhere

- `03_logs\decisions.md`: one decision block (BurnMon replaces claudecost, differentiator
  order, pilot, licence).
- `01_projects\burnmon.md`: new one-pager; `portfolio.md`: new row, claudecost noted as
  absorbed; BurnRate row gains "Phase 2 proxy superseded by BurnMon adapters for the
  coding-agent case".
- `DEADLINES.md`: 2026-10-17 v0.1, 2026-10-24 pilot answer, 2026-11-14 v0.2, 2026-12-12
  v0.3, 2026-12-19 decision.
- `02_roadmap\roadmap.md`: a pointer to this plan under the side-track allocation.
- Groundwork Kit and the Talon PoC plan: claudecost slot becomes BurnMon at v0.3.
- `C:\ZND\projects\claudecost`: README gains a pointer once the `burnmon` repo exists.
