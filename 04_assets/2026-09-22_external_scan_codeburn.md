---
type: analysis
title: External scan, CodeBurn (nearest competitor to the token monitor)
created: 2026-09-22
tags: [token-monitor, competitor, codeburn, tokscale, tokenuse]
status: reference
---

# External scan: CodeBurn

SR = self-reported (their README, site, Product Hunt). V = verified from a third party or
independent data. Researched 2026-09-22 by a subagent; sources at the end.

## 0. TLDR

CodeBurn (github.com/getagentseal/codeburn, MIT, 11,148 stars, created 2026-04-13, desktop
v0.9.25 released 2026-09-21) is a free, local-first, multi-vendor usage monitor for 41 coding
agents with a CLI, a local web dashboard, signed macOS desktop and menubar apps, a Windows
Store build, Linux packages, forecasting, project, branch and PR attribution, quota rings and
a Claude Code budget guard. It already does most of what Part 1 of our architecture note
called the differentiator. Where it stops and we start: it has no client or engagement
dimension, no time or billable-hours link, no durable archive beyond each tool's own logs,
desktop telemetry on by default outside the EU, and no legal entity to contract with.

## 1. The company

Brand AgentSeal, GitHub org `getagentseal`, "Germany", hello@agentseal.org (V, GitHub API).
No legal entity named anywhere; no Impressum on codeburn.app or agentseal.org; the Terms
(2026-03-13) pick US law. Founder Resham Joshi (`iamtoruk`, 1,745 commits; next
contributors 188 and 160), ex Development Engineer at Intenta Automotive GmbH, M.Sc. TU
Chemnitz (V, everydev.ai 2025-06-27). Second maker on Product Hunt: Aditya Vikram Singh (SR).
Funding: none announced; GitHub Sponsors with 50 sponsors (V); "Codex and Claude for Open
Source" credit badge (SR). Positioning: "Your AI Bill, Itemized", "privacy-first ccusage
alternative" (SR). Testimonials are social-media quotes; "used by 150k+ developers" (SR)
against 202,820 npm downloads since 2026-04-01 and 38,986 in the last 30 days (V). No case
studies.

## 2. Offerings

| Surface | What it is | Price | Role in their model |
|---|---|---|---|
| CLI/TUI (`npx codeburn`, brew) | dashboard, optimize with apply and undo, compare, yield, guard (Claude Code budget hooks), quota, export, MCP server | free, MIT | the funnel |
| `codeburn web` | same on localhost:4747, LAN pairing by PIN | free | |
| Desktop v0.9.25 | macOS signed and notarized; Windows Store build signed, direct installers unsigned "developer preview"; Linux deb/rpm/AppImage; six languages | free | |
| macOS menubar, Windows tray (Tauri), GNOME extension | today's spend, Forecast tab, quota rings, budget alerts | free | |
| `codeburn sync` | push aggregates to an OIDC endpoint you host; "preview" | free | bridge to Teams |
| CodeBurn Teams | team totals, cost per merged PR, "no leaderboard" | waitlist, no price | the intended paid tier |
| Paid tier | none; /pricing is 404 | | |

Sister product AgentSeal (security) has a Pro tier at $19/month or $199 one-time (V).

## 3. The product, from primary sources

- Sources read: 41 providers, each in docs/providers. Claude `~/.claude/projects`, Codex
  `~/.codex/sessions`, Cursor `state.vscdb`, Gemini, OpenCode, Zed, Warp, Cline family,
  Kimi, Kiro and more. On Windows also `\\wsl$\<distro>\home\*` for Claude and Codex.
- Storage and privacy: CLI reads disk only, "The CLI sends nothing." Desktop and tray have
  anonymous bucketed telemetry with a consent screen, "defaults to off in the EU, EEA, UK
  and Switzerland... on elsewhere" (README Telemetry). No account for anything. Prices from
  LiteLLM daily, exchange rates from Frankfurter.
- Forecast: yes. Menubar Forecast tab, "projected month total, with your pace measured
  against last month", Capacity Dock per quota window ("Runs out in 2d 8h") (CHANGELOG 0.9.25).
- Attribution: project (working directory), git branch and worktree (Claude transcripts
  only), PR at turn level, 13 task categories. No client or customer dimension.
- Quota windows: `codeburn quota` reads Claude, Codex, Gemini, Copilot, Kimi from signed-in
  credentials; 5-hour and weekly rings.
- Pricing: API list per token via LiteLLM with hardcoded fallbacks, cache write 1.25x, read
  0.1x; `plan set claude-max` adds overage lines; Copilot in AI credits, "never token-priced USD".
- Copilot: reads `session-state`, VS Code chatSessions, `agent-traces.db` (preferred),
  `session-store.db`, JetBrains DBs. "legacy JSONL sources only record output tokens", rest
  "estimated from content length".
- Hermes: reads `state.db`; "one parsed call per Hermes session", no per-turn detail.
- Limits in their words: "Attribution is timestamp-window based (heuristic)"; Cursor figures
  "are marked estimated and undercount"; sync "is in preview; the protocol may change".

## 4. The business model, read as a model

Everything shipped is free and MIT. Income today: sponsors and OSS model credits. The free
single-developer install feeds a Teams tier that does not exist yet: `sync push` already
emits "usage-span fields from the Teams boundary spec" and the Teams page collects work
email and team size. Nothing about revenue or customers is verified outside their pages.

## 5. Where they stop and we start

| Question | CodeBurn | Us (per the architecture note, revised in Part 2) |
|---|---|---|
| Unit of work | a tool session, a PR | a client engagement, a billable hour |
| Who writes | the developer's local tools | same, plus the developer's time log (BurnRate script, later Siteoffice) |
| Sync | LAN PIN pairing; OIDC endpoint you host | export files, `merge` command; no network |
| Governance | no legal entity, US-law terms | Dutch eenmanszaak, EU data posture, contractable |
| Always-on | tray or menubar, telemetry on outside EU | one window on demand, zero telemetry |
| Interface | CLI, web, desktop, tray, six languages | one exe, one page, EN and NL |
| Expertise | itemising the bill | what the bill should be per client, and a written opinion on why |
| Distribution | npm, brew, Store, dmg | portable exe, later winget and brew; Groundwork Kit for Talon |
| Harness | 41 adapters, heuristic attribution | 5 adapters done well, deterministic attribution or "unassigned" |

## 6. What we take from them

1. The LiteLLM price feed idea, but shipped as a dated JSON we control, never fetched live.
2. Copilot's `agent-traces.db` as the preferred Copilot source; their provider docs are the
   best map of that moving target.
3. The consent screen pattern, inverted: we have nothing to consent to.
4. The "forecast measured against last month's pace" wording; ours adds the error band.
5. `\\wsl$\<distro>\home\*` scanning on Windows confirms claudecost's registry approach is
   not the only route.
6. The Claude Code budget guard (a hook that stops at a cap) as a v1.1 candidate.
7. Their honesty tags ("estimated", "heuristic") as UI text, not footnotes.

## 7. Open items and flags

- status.agentseal.org returns 502 while the footer says "All systems operational".
- agentseal.com is not theirs (parked). codeburn.app/pricing, /impressum and
  /docs/providers/hermes are 404. Tool count differs across pages (36, 37, 40, 41).
- "Visa has forked CodeBurn internally" is a LinkedIn comment, unverifiable.

## Secondary: tokscale and tokenuse

tokscale (junhoyeo/tokscale, 5,505 stars, v4.17.0 2026-09-15, MIT): Rust CLI and TUI,
about 50 clients, LiteLLM pricing; its distinguishing feature is a public leaderboard with
GitHub login, the opposite of a privacy pitch. No desktop app, no paid tier.
tokenuse (russmckendrick/tokenuse, 33 stars, v1.2.5 2026-09-07, MIT): Rust TUI plus Tauri
desktop (macOS, WinGet, Linux), four tools only, durable local `archive.db`, report export
to HTML, PDF, Excel, opt-in quota sync via a cookie in the OS keychain, no telemetry.
Closest to our shape; single maintainer; the `archive.db` idea is the one we share.

## 8. Sources (all accessed 2026-09-22)

GitHub API: repos/getagentseal/codeburn, releases/latest (mac-v0.9.25, 2026-09-21),
releases/tags/desktop-v0.9.25, users/getagentseal, contributors, sponsors/iamtoruk.
Repo main: README.md, CHANGELOG.md, docs/providers/hermes.md, docs/providers/copilot.md,
docs/sync/README.md, docs/by-branch.md. Sites: codeburn.app (home, /teams, /telemetry,
/compare/ccusage, /docs/providers), agentseal.org (/terms 2026-03-13, /privacy 2026-03-01).
Product Hunt launch ~2026-08-17. everydev.ai/developers/agentseal (2025-06-27).
api.npmjs.org downloads 2026-04-01 to 2026-09-22. tokscale and tokenuse: GitHub API and README.
