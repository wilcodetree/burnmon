# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-22 (session 42A) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** v0.2 session 42A (insight package, re-prefill and compaction) is done, merged, and its two VERIFY items are closed with sourced facts.
**Read order:** this file, then C:\ZND\projects\burnmon\SESSION_LOG.md (top entry, 2026-09-22 42A), then C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md section 2.3 (I1, I2) for the rule text this implements.
**Supersedes:** nothing (first brief for 42A; follows hub_agent_update_2026-09-22_v0.2_41B_vendor_strip_hermes.md)

## 1. Headline
BurnMon v0.2 session 42A is code-complete and merged to main: new package internal\insight implements I1 (the Finding shape) and I2's first two rules (re-prefill, compaction), wired into both bmLive and a new burnmon-cli insight command, with the spec's two VERIFY items (Claude Code cache TTL, auto-compact threshold) now closed against live docs instead of left as placeholders.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (repo root): `f362810` "feat: insight package, re-prefill and compaction findings (I1, I2 part 1)". 9 files changed, 681 insertions, 1 deletion.
- **Files touched** (full paths):
  - `C:\ZND\projects\burnmon\internal\insight\insight.go` (new)
  - `C:\ZND\projects\burnmon\internal\insight\insight_test.go` (new)
  - `C:\ZND\projects\burnmon\internal\pricing\pricing.go` (new Config fields and consts: ClaudeCodeCacheTTLMinutes, ClaudeCodeAutoCompactTokens, ReprefillCacheWriteThreshold, ClaudeCodeCacheBookDate)
  - `C:\ZND\projects\burnmon\internal\live\live.go` (Session.Findings, computed per poll via insight.Analyze on the already-windowed turns)
  - `C:\ZND\projects\burnmon\internal\store\store.go` (new Store.EventsForSession query)
  - `C:\ZND\projects\burnmon\internal\store\store_test.go` (TestEventsForSession)
  - `C:\ZND\projects\burnmon\cmd\burnmon-cli\main.go` (new insight session-id subcommand, with a --json flag)
  - `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (42A ticked done)
  - `C:\ZND\projects\burnmon\SESSION_LOG.md` (new top entry)
- **On a branch, not merged:** none; work went straight to main per this project's usual flow.
- **Untracked / outside a repo:** none from this session. Note: `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1.2_patch_spec.md` shows as modified in git status but predates this session (present in the working tree before 42A started); left untouched and unstaged by this session, not part of `f362810`.

## 3. What did NOT happen (and why)
- Not pushed to origin/main: local commit only, per this project's own convention of stopping before push/tag for Wilco to run.
- I2's other two rules (context-runway, expensive-turn) are NOT built: that is session 42B, next in the plan.
- I3 (markers on the chart, turn ticker, spike drawer, Sessions tab findings) is NOT built: session 43A.
- No real production Finding has been eyeballed against a live long session beyond one smoke test (see section 4); the fixture tests are synthetic, not a replay of a real marathon session.
- The bmLive per-poll cost was measured only via a synthetic 2,000-turn fixture in a unit test (under 5ms), not profiled inside the running burnmon.exe app itself.

## 4. Findings worth propagating
- **[RESULT]** `go test ./... -count=1` and `.\build.ps1` both green after the change, full project, not just the new package (run 2026-09-22, this session).
- **[RESULT]** VERIFY closed for the v0.2 spec's I2 cache-TTL item: per code.claude.com/docs/en/costs (fetched live 2026-09-22), Claude Code's prompt-cache lifetime is one hour on a Claude subscription seat (Pro/Max/Team/Enterprise), dropping to five minutes once a session draws on usage credits, and five minutes by default on a bare API key or cloud provider. This differs from the spec draft's assumed flat 5-minute figure. Compiled-in default in pricing.Config.ClaudeCodeCacheTTLMinutes is set to the one-hour subscription figure, since Wilco's own pricing.Config.Subscription block already models a seat-based subscription, not an API key. Overridable per machine via burnmon.json's new claude_code_cache_ttl_minutes.
- **[RESULT]** VERIFY closed for the auto-compact threshold: per code.claude.com/docs/en/model-config (fetched live 2026-09-22), there is no single fixed percentage; Claude Code compacts when the conversation reaches the model's context window, except models on a native 1M-token window (Sonnet 5, the Fable models, Opus 4.7+ on the Anthropic API), which compact early at about 967,000 tokens by default. Recorded in pricing.Config.ClaudeCodeAutoCompactTokens for the record; not applied by either v0.2 rule, since burnmon's own price book prices the 200K standard tier and the compaction rule detects the drop by its effect, not by comparing against this number.
- **[RESULT]** internal/insight.Analyze measured under 5ms for a synthetic 2,000-turn session in TestAnalyze_UnderFiveMillisecondsPerSession, inside the spec's per-poll budget.
- **[STATE]** One live smoke test against a real running session in Wilco's own store (this Claude Code session, id 591a35c6-b293-45c7-9d57-c83fd8d6bd82) returned one re-prefill finding, cause "first turn after resume", confidence 0.4, via burnmon-cli insight (session id) --json. Single manual check, not a systematic validation against known-cause real sessions.
- **[STATE]** A CLI argument-order bug was found and fixed during this same session, before commit: `burnmon-cli insight <session-id> --json` (flag after the positional argument, as the spec's own example writes it) silently ignored --json because Go's flag package stops parsing at the first non-flag argument. Fixed by splitting positional and flag args by hand before flag.Parse. No user-facing regression since this shipped and was fixed in the same session, but noting it since it is exactly the kind of thing that would otherwise look like a real bug report later.

## 5. Hub-level decision (if any)
Nothing. The cache-TTL default choice (one hour, subscription case) is a project-internal implementation call inside BurnMon's own price book, not a cross-project, pricing, or positioning decision.

## 6. What the next hub read should update
- `C:\ZND\10_holding\02_roadmap\roadmap.md`: the BurnMon line currently reads "...42A (insight package) next." Update to reflect 42A done, live HEAD f362810 (not yet pushed), 42B next.
- `C:\ZND\10_holding\01_projects\burnmon.md`: the project one-pager should gain a short 42A line matching the roadmap.md convention used for prior sessions (39B, 40A, 40B, 41A, 41B).
- No DEADLINES.md change: the 2026-11-14 v0.2 tag date is unaffected, this session was on the two-sessions-a-week pace.
- No portfolio.md change expected beyond whatever the one-pager update triggers.
- Not a Mission Deck (This Week) item: BurnMon runs as a side-track per roadmap.md, not an A1..A8/B1..B5 plan item, so section 6's status-glyph rule does not apply here.

## 7. Open flags for next session
- f362810 is not pushed to origin/main yet; push is Wilco's call, per this project's stop-before-push convention.
- Session 42B (context-runway, expensive-turn, tag v0.2.0-alpha.3) is the next item in `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md`.
- The pre-existing uncommitted change to `02_roadmap/2026-09-22_v0.1.2_patch_spec.md` (present before this session started) is still sitting unstaged in the working tree; worth Wilco's own look, not something this session touched or explains.
- The cache-TTL default (one hour, subscription case) is a judgment call, not a hard spec answer; flagged to Wilco directly in-session, no objection raised yet.

## 8. Related files
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` (section 2.3, I1/I2)
- `C:\ZND\projects\burnmon\04_assets\2026-09-22_burnmon_now_page_features.md` (section 3, live context features)
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (42A prompt, now ticked)
- `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry, full technical detail)
- `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.2_41B_vendor_strip_hermes.md` (prior brief this one follows)
