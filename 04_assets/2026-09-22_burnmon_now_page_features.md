---
type: analysis
title: BurnMon, the Now page and live features
created: 2026-09-22
tags: [burnmon, features, live, context, forecast]
status: decided 2026-09-22
---

# BurnMon: the Now page and live features

Follows the grill (`_grill_state.md`) and Wilco's feature list of 2026-09-22. Codex and
Copilot adapters are already in the plan (weeks 41 and 43); this note covers the rest:
the live view, the Now page, the dev and business switch, the forecast chart, and what
"seeing the context of all tools live" can mean with the data that exists.

## 0. TLDR

The Now page is feasible with file watchers alone, no OTel, for Claude Code, Cowork,
Codex and Hermes: their trails are appended per message, so a watcher gives one to two
second latency. Copilot is the exception: the CLI writes totals at session end and VS Code
Copilot writes no tokens at all unless OTel is on, so Copilot rows on the Now page say
"totals at session end" or "enable OTel" until v1.1. Context is visible per running
session: Claude and Codex trails carry enough per turn to compute context fill, cache hit
ratio and re-prefill events, and that is the feature nobody ships: not "how many tokens",
but "why did that turn cost 120K, and which of your five agents is about to hit its
window". The dev and business switch is one toggle over the same events: dev shows
tokens, context and cache classes; business shows euros, credits and the forecast.

## 1. What "live" means per vendor (from the facts list, 2026-09-22)

| Vendor | Written when | Watcher latency | Live context | Live plan window |
|---|---|---|---|---|
| Claude Code, Cowork | every assistant message, JSONL append | 1 to 2 s | yes: usage per turn gives context size (input + cache read + cache write), model known | no on disk; derive from own rate against plan |
| Codex CLI, VS Code, Desktop | `token_count` event per turn, JSONL append | 1 to 2 s | yes: `last_token_usage`; window VERIFY (`model_context_window` in the payload is reported by parsers, confirm on a live file) | yes: `rate_limits` 5h and weekly `used_percent`, `resets_at`, written per turn |
| Hermes | SQLite `messages` per message, WAL | poll 5 s | partial: per-message token_count, window VERIFY | no |
| Copilot CLI | session totals at shutdown | none until session end | no | no (credits via GitHub API, not local) |
| Copilot VS Code | nothing without OTel | with OTel file exporter: 1 to 2 s | with OTel: per request tokens, VERIFY context | no |

Consequence for the architecture note: the 15-minute refresh model (claudecost) stays for
history; the Now page gets a watcher (`fsnotify` on Windows, macOS, Linux; WSL folders
polled, since inotify does not cross the boundary, ASSUMED) and Hermes a 5-second poll.
The store ingests the same events either way, dedup by request id unchanged.

## 2. The Now page (first page)

Perfadvisor's live monitor is the model: gauges first, a scrolling chart, then detail.

**Top strip, one card per running session** (a session is "running" when its trail changed
in the last 10 minutes; ASSUMED threshold, configurable): vendor and surface icon (Claude
Code CLI, Codex VS Code, Hermes), model, project or client, started, last turn, and three
numbers: context fill (a gauge, current context over the model's window), tokens this
session, cost so far in the active mode. A fourth, for Claude only: cache clock, the time
since the last turn against the 5-minute cache TTL, because the next turn after expiry
pays a full cache write (BurnRate finding 1: cache read is 95 to 98% of input). "Cache
expires in 2:13" is the single most useful live number for a Claude user and nobody
shows it. VERIFY the TTL that applies to Claude Code sessions (5 minutes default, 1 hour
on the extended-TTL option, per Anthropic pricing page).

**Live burn chart**: tokens per minute, last 30 minutes, stacked by session, colour by
vendor, a thin line for cost per minute on the second axis. Turn markers on the line; hover
a marker: turn number, token classes, cost, tool calls in that turn. A spike with a
different hatch when `cache_creation` dominates: a re-prefill (context rebuilt, after a
compaction, a model switch, an edited CLAUDE.md, or an expired cache).

**Forecast chart, directly under**: same time axis extended to end of day and end of
month, two bands: the plan's line (what the last four weeks say today will cost, weekday
aware) and the live line (what the current burn rate says if it continues). Where they
diverge is the story. The month-end number with its error band in the corner. The
forecast card follows the plan rule: no forecast without a scored week behind it; before
that, the chart shows history only and says why.

**Plan window strip** (where data exists): Codex 5h and weekly bars from `rate_limits`,
live; Claude and Copilot bars derived from own events against the configured plan,
labelled "derived". Never an undocumented endpoint.

## 3. Context, live: what can be seen and what cannot

What the trails give per turn: total context sent (Claude: input + cache read + cache
write; Codex: input + cached input), how much of it was served from cache, how much was
new, output, and for Claude the tool calls in the turn. From that, per running session:

1. **Context fill gauge** against the model's window (Claude and Codex window sizes from
   the price book, dated; Hermes and Copilot VERIFY). Colour bands at 60, 80 and 90%.
   Claude Code auto-compacts near the window (the threshold is Anthropic's, VERIFY the
   current figure); the gauge shows the compaction as a drop and lists it as an event.
2. **Cache hit ratio** per turn and per session: cached over total input. A healthy
   Claude session sits above 95%; a session that dips is paying for something. The dip
   turn is clickable: which tool result or file read arrived that turn.
3. **Re-prefill events**: a turn where cache write exceeds, say, 20K tokens. Listed with
   the likely cause when it can be inferred from the trail: compaction just happened,
   model changed between turns, the gap since the previous turn exceeded the TTL, or the
   first turn of a resumed session. This is BurnRate's finding turned into a live alarm.
4. **Context growth per turn**: a sparkline of context size across turns, so a session
   that adds 8K per turn from tool results is visible before it hits the wall. Claude's
   `/compact` advice from BurnRate ("compact at turn 25 to 30") becomes a line on the
   sparkline, per session, with the actual turn count next to it.
5. **Parallel agents**: Claude subagents live in `<session>/subagents/`; the Now page
   draws them under their parent with their own gauge. Codex and Hermes: one row per
   session; no subagent structure in the trail (VERIFY for Codex).
6. **What is in the context** is not in the usage data. The trail does carry the messages,
   so a dev-mode drawer can show the last N turns' tool calls and file paths read, which
   is "what filled it", not "what it contains". Nothing is sent anywhere; it is the
   developer's own file. Out of scope: parsing prompt text into categories.

What cannot be seen live: Copilot (see section 1), the model's own reasoning tokens for
Claude (not in usage), and the true plan window for Claude and Copilot.

## 4. Dev and business switch

One toggle in the header, remembered per machine. Same events, two vocabularies.

| Element | Dev | Business |
|---|---|---|
| Session card numbers | context fill, tokens, cache hit ratio | cost so far, plan credits left, client |
| Live chart unit | tokens per minute by class | euros per minute, one line per client |
| Forecast | tokens to window and to plan limit | euros to month end, error band |
| Events | re-prefill, compaction, model switch | "expensive turn: EUR 0,42, cause: cache expired" |
| Drawer | tool calls, file paths, request ids | active time per client, export button |

The business mode is where Talon and an advisory client look; the dev mode is where a
developer learns why. Both are the same 40 events; the switch costs one template branch.

## 5. Other features worth a slot (not yet in the plan)

- **Turn ticker**: a one-line feed under the chart, newest first: "14:02:11 Codex VS Code,
  turn 31, 2.1K new, 118K cached, EUR 0,03". It makes the abstract tangible and is free.
- **Idle-but-loaded warning**: a session with a large context and no activity for longer
  than the TTL, so the developer knows the next question costs a re-prefill; the fix is
  to ask now or to accept it.
- **Compare two sessions** side by side: same task on Claude and Codex, same chart.
- **Session replay**: scrub through a finished session's context growth turn by turn. The
  talks material (BurnRate posts) comes straight from this view.
- **Daily digest line** in the Now page footer: today so far against the same weekday's
  average, one sentence.
- **Budget guard hook** (v1.1, from the CodeBurn scan): a Claude Code Stop hook that
  warns at a per-day cap; Codex and Copilot hooks the same way.

## 6. What this changes in the plan (applied 2026-09-22)

- v0.1 (2026-10-17): Now page with Claude and Codex live sessions, context gauges, cache
  clock, live burn chart, turn ticker; history pages as claudecost. The forecast chart
  slot exists but shows history only until a scored week exists (plan row week 44 to 46).
- v0.2 (2026-11-14): forecast live line, re-prefill events, Hermes live, Copilot rows with
  the honest label, dev and business switch.
- v0.3 (2026-12-12): unchanged (client map, active time, export, merge, macOS, Linux).
- Risk added: the Now page pulls v0.1 from "claudecost with a Codex adapter" to a new
  first page; one session a week may not carry it. Mitigation: the week 42 "done when"
  becomes "Now page shows Wilco's running Claude and Codex sessions with context fill and
  a live chart", nothing more; ticker and cache clock slip to v0.2 if needed.

## 7. Decisions, taken 2026-09-22

1. The Now page enters v0.1 (2026-10-17): running Claude and Codex sessions with context
   fill, a live burn chart; cache clock and turn ticker if the week allows, else v0.2.
   The forecast chart slot shows history only until a scored week exists.
2. Default mode: dev. Talon's `burnmon.json` sets business as default. The toggle is one
   click and remembered per machine.

## 8. VERIFY list carried

Codex `model_context_window` in the rollout payload; Claude Code auto-compact threshold
and cache TTL as applied to Claude Code; Hermes and Copilot window sizes; whether inotify
sees WSL folders from Windows (ASSUMED no, poll); Codex subagent structure in the trail.
