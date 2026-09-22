# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-22 - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** BurnMon v0.1 Step 2 (Codex adapter) is shipped, tagged, and pushed; here is what actually landed and what is still open before Step 3.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md` (2026-09-22, v0.1 Step 2 entry), then `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1_spec.md` for Step 3.
**Supersedes:** `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.1_step1_schema_store_adapter.md` (Step 1's own brief; this one covers the next step, not a correction to it).

## 1. Headline
BurnMon v0.1 Step 2, the Codex adapter, is merged to `main` and tagged. Wilco's own real Codex sessions (34 of them, spanning three months) now appear in the local store with the right model, turn counts and cost, verified against his live `~/.codex/sessions` trail, not only the fixture.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (repo `burnmon`): `f9d4b68` "feat: Codex adapter (v0.1 Step 2)", 13 files changed, 928 insertions, 34 deletions.
- **Tagged and pushed:** `v0.1.0-alpha.2` on `f9d4b68`, pushed to `origin/v0.1.0-alpha.2` and `origin/main` (commit `9b0b133..f9d4b68`).
- **New:** `C:\ZND\projects\burnmon\internal\adapter\codex\codex.go`, `codex_test.go`; `C:\ZND\projects\burnmon\testdata\codex\three-turns.jsonl` (fixture: 3 turns, one `rate_limits` object, a mid-session model switch).
- **Changed:** `C:\ZND\projects\burnmon\internal\scan\wsl.go` and `wsl_other.go` (generalised WSL distro discovery so Codex reuses Claude's, not a second copy), `internal\pricing\pricing.go` (OpenAI price book), `internal\dataset\dataset.go`, `fromstore.go` and their tests (adapter dispatch was hardcoded to Claude for every file; now resolves and classifies both adapters), `internal\scan\types.go` (`Session.Unpriced`), `cmd\burnmon-cli\main.go` (new `price-check` verb).
- **Also present, unrelated to this brief:** `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.1_step1_schema_store_adapter.md` was left untracked by the prior session; not touched here.

## 3. What did NOT happen (and why)
- Step 3 (the Now page, live watch, context windows) has not started. Out of scope for this session.
- The `tools`/`tool_calls` table and `tools --json` CLI verb from Step 1's spec section were never actually implemented in Step 1 (confirmed by reading the shipped code, not assumed); still open, not part of Step 2 either.
- The spec's fallback path (deriving a Codex turn from a `total_token_usage` delta when `last_token_usage` is absent) is implemented but has never run against real data: every rollout line on Wilco's laptop carried `last_token_usage`. Flagged as untested-on-live-data in `SESSION_LOG.md`, not claimed as verified.
- No `.jsonl.zst` compressed rollout sibling was found on this laptop to test the "logged and skipped" path against; the code path exists and is inert (relies on `scan.FindJSONL`'s existing `.jsonl`-suffix filter) but was not exercised by a real `.zst` file.
- `docs\2026-08-17_wsl-source-detection-design.md`'s design was extended, not rewritten; Codex's WSL sources use the same 5-second deadline and the same "two cadences" fast/slow split as Claude's, not a separately tuned budget.
- Nothing was pushed to any Valona-facing location; `C:\dev\Work` was never read or touched this session.

## 4. Findings worth propagating
- [RESULT] 34 of Wilco's real Codex sessions (`gpt-6-astra`, `gpt-5.6-sol`, `gpt-5.6-terra` priced; `codex-auto-review` deliberately left unpriced) ingested correctly from a wiped store via a live `burnmon-cli.exe report` run against his actual `~/.codex/sessions` and `~/.claude/projects` trails, three months back. One example: a GPT-6 Astra session, 230 calls, $56.72 list-price cost.
- [RESULT] 30 sessions, all `codex-auto-review`, correctly rolled up under the new `unpriced_tokens` counter (786,849 tokens in the largest one) rather than silently pricing at some guessed rate.
- [RESULT] Real Codex trail field names diverge from the spec's assumptions in three places, now documented in `SESSION_LOG.md`: `turn_context` is a top-level event (no `event_msg` wrapper, unlike `token_count`); `rate_limits.primary` is not reliably the 5-hour window (varies by CLI build, 0.146.0 vs 0.154.0 seen); `originator` values are `codex-tui` / `Codex Desktop` / `codex_work_desktop` / `Claude Cowork`, none matching the spec's guessed `codex_cli_rs`/`codex_vscode`.
- [RESULT] OpenAI list prices for the four priced model ids, checked 2026-09-22 against `developers.openai.com/api/docs/pricing` (redirects through `/codex/pricing` -> `learn.chatgpt.com/docs/pricing` -> the API rate card): GPT-6 Astra $10/$1 cached/$50 per MTok, GPT-5.6 Sol $4/$0.40/$20, GPT-5.6 Terra $2/$0.20/$12, GPT-5.6 Luna $0.20/$0.02/$1.20 (in/cached-in/out). All four cache at exactly 10% of input rate.
- [STATE] Step 3 (the Now page) is next per the spec, not started.

## 5. Hub-level decision (if any)
Nothing. This is a project-internal implementation step, no cross-project time, positioning, pricing or CIPHER-wall call involved.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` (the hub one-pager): Step 2 shipped, `v0.1.0-alpha.2` tagged and pushed.
- `C:\ZND\10_holding\02_roadmap\roadmap.md`: if BurnMon's week 41 row tracks step completion, mark Step 2 done.
- Mission Deck: no named This Week item (A1..A8, B1..B5) was referenced by this session's instructions, so nothing to set here; flag to Wilco if BurnMon Step 2 was meant to map to one.

## 7. Open flags for next session
- Step 1's `tools --json` CLI verb and `tool_calls` table are still unimplemented; whoever picks up that thread should know it was never actually built despite being in the Step 1 spec section.
- The `total_token_usage`-delta fallback path in the Codex adapter has zero live-data coverage; worth a real test fixture if a rollout without `last_token_usage` is ever found.
- `codex-auto-review` has no published per-token rate anywhere found this session; if OpenAI publishes one later, add it to `internal\pricing\pricing.go`'s `OpenAIPrices` map (it currently prices as 0/unpriced on purpose).
- Step 3 (Now page, live watch via `fsnotify`, context-window table) is next; not started.

## 8. Related files
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1_spec.md` (the full v0.1 spec, all three steps).
- `C:\ZND\projects\burnmon\04_assets\2026-09-22_token_monitor_sources_and_facts.md` (the phase-1/phase-2 research this step's field-name checks were built on).
- `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.1_step1_schema_store_adapter.md` (Step 1's own brief).
- `C:\ZND\projects\burnmon\SESSION_LOG.md` (both the Step 1 and Step 2 entries).
