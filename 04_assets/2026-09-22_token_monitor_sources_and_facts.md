# Token monitor grill: phase 1 source map and phase 2 facts list

Date: 2026-09-22. Grill: vendor-agnostic local token and cost monitor (successor of claudecost).
State file: `C:\ZND\10_holding\04_assets\_grill_state.md`. Research by two subagents this
session; every fact below is dated and tagged HELD (primary source read) or VERIFY (secondary
or unconfirmed, with what would confirm it).

## Part 1. Source map (internal)

**`C:\ZND\projects\claudecost\README.md`** (last edited 2026-08-31). Settles the whole current
product surface: Go, standard library, WebView2, two portable exes, no ports, no network,
reads Claude Code and Cowork transcripts, WSL via registry, dedup one entry per `requestId`
keeping the largest `output_tokens`, pricing compiled-in with `claudecost.json` override,
cost formula output-driven with `OutputCostFactor` 1.8085. Leaves open: any other vendor.
"Coverage is Cowork and Claude Code only." (README.md:89). Codex, Copilot, Hermes: 0 hits.

**claudecost `CLAUDE.md`, `AGENTS.md`, `STATUS.md`, `DEADLINES.md`, `00_context\brief.md`,
`02_roadmap\roadmap.md`, `SESSION_LOG.md`.** All stubs, "To be filled, 2026-09-08". There is
no written claudecost roadmap anywhere. No `01_projects\claudecost.md` one-pager, no
portfolio row. Owner of claudecost as a project: open.

**claudecost `docs\2026-08-11_claudecost-design.md`.** Settles origin: Go port of the
`my-usage-dashboard` Cowork plugin, schema 1 preserved, "deliberately personal ... no
aggregation across people, by design; individual usage is personnel-adjacent data". Leaves
open: pricing table and HTML template are hand-mirrored copies; drift risk.

**BurnRate `AGENTS.md`, `STATUS.md`, `docs\findings.md`, `docs\roadmap.md`.** Settles the
Claude JSONL usage fields (`input_tokens`, `cache_creation_input_tokens`,
`cache_read_input_tokens`, `output_tokens`) and the cost truth: cache reads are 95 to 98% of
effective input; Finding 6 (findings.md:75): Anthropic does not appear to charge cache
traffic on the Team plan, so API list price is about 8.5x the billed figure. Phase 2 (API
proxy, "thin Anthropic / OpenAI / OpenAI-compatible layer", docs/roadmap.md:39) is a stub with
no start date. Leaves open: the siteoffice `02_roadmap\roadmap.md` stub is empty, the real
roadmap is `docs\roadmap.md`.

**`portfolio.md`, `roadmap.md`.** BurnRate Maintenance, zero build hours (portfolio.md:34).
marketadvisor archived 2026-09-08, was "a deliberate test of the claudecost-app recipe (Go +
WebView2, one window, no server, no open port)" (portfolio.md:38). claudecost is absent from
the roadmap priority order. Leaves open: no slot for a successor.

**`decisions.md`.** 2026-08-11 (:1214-1220): claudecost and perfadvisor published as public
MIT repos under github.com/wilcodetree, personal, not Valona-owned, Valona invoice and seat
data stripped. Public `C:\ZND\projects\claudecost` diverges from the Valona working copy
`C:\dev\Work\claudecost`. 2026-08-28 (:400): Talon claudecost config, 5 seats, EUR 273,58 per
month. 2026-09-08 (:312): the `claudecost` GitHub repo excluded from renames.

**Talon.** Martijn, Bart, Jeroen (talon.studio), Siteoffice's first customer, pilot since
2026-09-01, PoC 2026-11-01 to 2027-02-01. Got the free Groundwork Kit incl. claudecost.
claudecost "on every laptop" (`02_roadmap\2026-09-08_talon_poc_plan.md:50`); its exports
supply one of the PoC's three closing numbers (:38).

**DEADLINES within 3 weeks.** 2026-09-22: archive `C:\dev` (minus `Work`), where the Valona
claudecost fork lives. 2026-09-27: Siteoffice sprint 1 ends. 2026-09-28 to 10-11: estate
rename window, will move `C:\ZND\projects\claudecost`. 2026-10-04: siteoffice-git kill check.
2026-10-11: sprint 2 ends. 2026-10-15: Talon MLP.

### Conflicts between sources

1. Pricing basis: claudecost prices a subscription share plus API list comparison; BurnRate
   says API list is about 8.5x reality on Team plans. Order-of-magnitude disagreement.
2. Aggregation: claudecost design forbids cross-person aggregation; the Talon PoC plan wants
   claudecost output as a firm-level closing number.
3. Two claudecost copies (public MIT vs Valona working copy in `C:\dev\Work`).
4. Talon seat mix unresolved (decisions.md:350).

## Part 2. Facts list (live research)

### A. Claude Code (CLI and VS Code extension)
- Transcripts `~/.claude/projects/<project>/<session>.jsonl`, plus `subagents/` and
  `tool-results/`; `CLAUDE_CONFIG_DIR` overrides. 2026-09-22, code.claude.com/docs/en/claude-directory. HELD.
- Auto-deleted after `cleanupPeriodDays` (default 30). Same page. HELD.
- Per-assistant-line `message.usage` has the four token classes, model, timestamp, sessionId,
  requestId. HELD via claudecost `internal/scan/parse.go` and BurnRate AGENTS.md:44-46.
- `/usage` computes dollars locally at list price; `modelPricing` managed setting overrides;
  1.1x data-residency multiplier since v2.1.239. code.claude.com/docs/en/costs. HELD.
- Non-interactive use (`claude -p`, SDK) draws from a separate monthly credit pool billed at
  API rates since June 2026. 2026-09-03, lowcode.agency. VERIFY: read support.claude.com.
- OTel: `CLAUDE_CODE_ENABLE_TELEMETRY=1`, metrics `claude_code.token.usage`,
  `claude_code.cost.usage`, logs `claude_code.api_request`. code.claude.com/docs/en/monitoring-usage. HELD.
- Hooks (SessionStart, Stop, SessionEnd, ...) receive `transcript_path` and `session_id`.
  Status line stdin JSON carries `cost.total_cost_usd` and `context_window.current_usage`.
  code.claude.com/docs/en/hooks and /statusline. HELD.

### B. OpenAI Codex (CLI and VS Code extension)
- State under `CODEX_HOME` (default `~/.codex`). developers.openai.com/codex/config-advanced. HELD.
- Rollouts `~/.codex/sessions/YYYY/MM/DD/rollout-<ts>-<uuid>.jsonl`, shared by CLI, Desktop
  and VS Code extension. HELD via openai/codex issues #20165, #21660 (2026).
- Per-turn usage: `event_msg` with `payload.type == "token_count"`, `total_token_usage` and
  `last_token_usage` (input, cached_input, output, reasoning_output, total) plus
  `rate_limits` (5h and weekly used_percent, resets_at). Counters are cumulative per
  session. Model from `turn_context`. ccusage codex guide, 2026-08-16. HELD.
- No cache-write class in Codex logs. HELD.
- Session files can grow to 700 MB to 2 GB (openai/codex #24948) and newer builds may write
  `.jsonl.zst` plus a `state_5.sqlite` index. VERIFY: inspect a fresh install.
- OTel: `[otel]` in config.toml, metric `turn.token_usage` by token_type; `notify` on
  `agent-turn-complete`; hooks in `~/.codex/hooks.json`. developers.openai.com/codex/hooks. HELD.

### C. GitHub Copilot (VS Code chat/agent and Copilot CLI)
- CLI state under `COPILOT_HOME` (default `~/.copilot`); sessions in
  `session-state/<uuid>/` plus `session-store.db`; synced to GitHub by default
  (`"remoteExport": false` opts out). docs.github.com Copilot CLI reference and Chronicle. HELD.
- Legacy `events.jsonl` had session-level totals only (`modelMetrics.<model>.usage`), and the
  CLI stopped writing it around May 2026; newer builds keep token totals in `data.db`.
  ccusage issue #1174, tokenuse docs. VERIFY: inspect a current install.
- VS Code Copilot Chat: OTel off by default; `github.copilot.chat.otel.enabled`, file
  exporter to JSONL, `dbSpanExporter.enabled` persists spans to `agent-traces.db`.
  code.visualstudio.com/docs/agents/guides/monitoring-agents, 2026-09-16. HELD; VERIFY db path.
- Copilot CLI OTel: `COPILOT_OTEL_ENABLED=true`, file exporter, GenAI semantic conventions
  (`gen_ai.usage.*`). HELD via snippet.

### D. Hermes Agent (NousResearch/hermes-agent, MIT)
- `~/.hermes` on Linux, macOS, WSL2; native Windows under `%LOCALAPPDATA%\hermes`. README. HELD.
- All sessions in SQLite `$HERMES_HOME/state.db` (WAL): tables `sessions` (model, token
  counts, billing), `messages`, `session_model_usage`. Developer guide. HELD via snippet;
  VERIFY native Windows path.
- Records actual cost when known. ccusage hermes guide. HELD. No OTel or hooks found. VERIFY.

### E. Official APIs
- Anthropic Usage and Cost Admin API: Admin key, org only, "unavailable for individual
  accounts". platform.claude.com. HELD. Pro/Max: no API. HELD.
- OpenAI Usage and Costs API: Admin key, API usage only, not ChatGPT/Codex subscriptions. HELD.
- Copilot usage metrics API: org owner, `ai_credits_used` per user since 2026-06-19, no
  per-model split. github.blog changelog. HELD. Individual: `GET
  /users/{username}/settings/billing/ai_credit/usage`. HELD via snippet.
- Undocumented `api.github.com/copilot_internal/user` quota endpoint. VERIFY.

### F. Pricing, September 2026
- Claude Pro $20, Max 5x $100, Max 20x $200. VERIFY: primary page not fetched.
- Claude API per MTok (input / cache write / cache read / output): Fable 5.1 10 / 12.50 /
  0.25 / 50; Opus 5 5 / 6.25 / 0.50 / 25; Sonnet 5 2 / 2.50 / 0.20 / 10; Haiku 4.5 1 / 1.25 /
  0.10 / 5. platform.claude.com pricing. HELD.
- ChatGPT Plus $20, Pro $100 or $200, Codex included, extra usage via credits per MTok.
  developers.openai.com/codex/pricing. HELD.
- Copilot on AI credits since 2026-06-01, 1 credit = $0.01, token-metered per model; Pro $10
  (1,500 credits), Pro+ $39 (7,000), Business $19 (1,900 pooled), Enterprise $39 (3,900).
  github/docs variables. HELD. Per-model MTok rates in `models-and-pricing.yml`. HELD.

### H. Competitors (all read local files, all MIT unless noted)
- ccusage, 18,673 stars, v20.0.24 (2026-09-21), CLI, 18 sources incl. all four. HELD.
- tokscale, 5,505 stars, v4.17.0 (2026-09-15), CLI + TUI + leaderboard, ~55 tools. HELD.
- CodexBar, 21,712 stars, v0.64.0 (2026-09-21), macOS menu bar (Linux Qt port), quota windows
  for Codex, Claude, Copilot and more. HELD.
- CodeBurn, 11,148 stars, mac-v0.9.25 (2026-09-21), signed macOS desktop + web. HELD.
- Token Monitor (Javis603), 2,270 stars, v0.60.0 (2026-09-21), desktop widget, multi-device
  sync. HELD.
- tokenuse, 33 stars, v1.2.5 (2026-09-07), Rust TUI + Tauri desktop (macOS, Windows winget,
  Linux), local-only, Claude, Codex, Copilot incl. VS Code stores. HELD.
- ClawMetry, 419 stars, pip web dashboard, 32 runtimes, 4 free without account. HELD.
- tokenmeter, 39 stars, v1.4.0 (2026-05-31), hooks + JSONL watcher, 15 sources. HELD.
- ClaudeBar 1,496, Claude-Usage-Tracker 3,550 (macOS, Claude only), Agent Sessions 871
  (macOS session browser). HELD. ccflare stale (last push 2026-04-19). LiteLLM, Helicone:
  proxies, not local-file monitors. HELD.
- Verdict: multi-vendor local coding-agent monitoring already exists and is crowded. The
  thin spots: Windows-native desktop, WSL plus native dual-root discovery, no account, no
  cloud sync, and per-project or per-client attribution.

## What this means for the design (research summary, not decisions)

1. Local files are the only vendor-neutral, no-admin source. Parsing them is commodity.
2. Token classes differ per vendor; normalise to five classes (input, cache write, cache
   read, output, reasoning) with nulls.
3. Codex counters are cumulative; diff or take-last per session.
4. Copilot is the moving target: three storage layouts in 2026 alone.
5. Subscription users have no per-token bill; "API-equivalent" is the honest label, and
   BurnRate's 8.5x finding means it must be labelled as an upper bound.
6. Quota windows (5h, weekly, credits) are what developers actually watch.
7. Live streaming exists for Claude, Codex and Copilot via OTel or hooks; Hermes by polling.
8. Official APIs are admin-only and skip subscriptions: an optional org tier, never the core.
9. Retention: Claude deletes after 30 days, Copilot syncs to cloud. Snapshot early.
10. Coverage parity is a losing pitch against ccusage and tokscale. Differentiation has to
    come from Windows-first, local-only, prediction and attribution.
