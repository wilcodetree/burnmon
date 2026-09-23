# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 (v0.3 session V3-2) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** V3-2 (client map K1, active time K2) is code-complete on main, reviewed by a fresh Opus pass and fixed, not yet committed or tagged; hand-off for whoever commits/tags next.
**Read order:** this file, C:\ZND\projects\burnmon\SESSION_LOG.md (top entry), C:\ZND\projects\burnmon\docs\superpowers\plans\2026-09-23-client-map-active-time.md
**Supersedes:** hub_agent_update_2026-09-23_v0.3_V3-2.md (this file replaces the earlier draft of itself, written before the review pass)

## 1. Headline
K1 (client map) and K2 (active time) are implemented, fully tested (go test ./... -count=1 and .\build.ps1 green), and have been through one fresh-context review (Opus, via the Agent tool) that found and confirmed a Critical correctness bug plus several Important issues; all of them were fixed in the same session and re-verified, so the code on disk now reflects the fixed version, not the reviewed one. Not yet committed to git: per the workspace house rule (no git writes on C:\ZND), every change is staged on disk for Wilco to commit and tag himself.

## 2. What changed on disk
- Committed: nothing yet. Every change below is an uncommitted working-tree edit in C:\ZND\projects\burnmon (branch main).
- Files touched (full paths):
  - C:\ZND\projects\burnmon\internal\pricing\pricing.go (OwnerRule gains Client/Remote; Config gains ActiveIdleMinutes; a unified matchRule used by both OwnerFor and ClientFor)
  - C:\ZND\projects\burnmon\internal\pricing\gitremote.go (new; reads the .git config origin remote directly, no git binary, follows a worktree commondir chain; normalizeRemote and remoteMatches, whole-segment matching)
  - C:\ZND\projects\burnmon\internal\pricing\gitremote_test.go (new; includes a worktree-chain test)
  - C:\ZND\projects\burnmon\internal\pricing\pricing_test.go (ClientFor tests, a v0.2-config-unchanged test, plus post-review tests for the remote-only-rule bug, whole-segment matching, and the unified match)
  - C:\ZND\projects\burnmon\internal\schema\event.go (Event.Client)
  - C:\ZND\projects\burnmon\internal\store\migrations\migrations.go (migration 7: client column)
  - C:\ZND\projects\burnmon\internal\store\store.go (client column read/write; ReownEvents takes ownerFor and clientFor)
  - C:\ZND\projects\burnmon\internal\store\store_test.go (client round-trip, reown-reapplies-client, schema-version bumped 6 to 7 in two pre-existing tests)
  - C:\ZND\projects\burnmon\internal\dataset\dataset.go (ingest sets Client via a per-pass memoized cfg.ClientFor)
  - C:\ZND\projects\burnmon\internal\dataset\fromstore.go (buildSession computes Client, ActiveMinutes; ActiveMinutesByClient)
  - C:\ZND\projects\burnmon\internal\dataset\fromstore_test.go (active-time tests, including a same-instant-events case added post-review)
  - C:\ZND\projects\burnmon\internal\scan\types.go (Session.Client, Session.ActiveMinutes)
  - C:\ZND\projects\burnmon\cmd\burnmon-cli\main.go (reown calls a memoized clientFor too)
  - C:\ZND\projects\burnmon\burnmon.example.json (owners comment shows a client and remote example)
  - C:\ZND\projects\burnmon\SESSION_LOG.md (V3-2 paragraph, rewritten after the review to describe the fixed design and list what was caught)
  - C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_session_prompts.md (V3-2 ticked)
- New plan doc: C:\ZND\projects\burnmon\docs\superpowers\plans\2026-09-23-client-map-active-time.md
- Untracked / outside a repo: none.

## 3. What did NOT happen (and why)
- Not committed, not tagged, not pushed. House rule: git writes on C:\ZND are a manual step for Wilco. Exact commands below.
- Real-log spot-check comparison did not happen: C:\ZND\10_holding\03_logs\time\ does not exist anywhere under the hub (confirmed by listing), so there is no 2026-09-22 entry to compare against. K2 active-time figures for three real sessions are recorded (section 4) but unverified against an external source; plan assumption A8 explicitly allows for the case where no entry exists.
- No UI change: K1 and K2 both say no UI yet beyond what already renders sessions, so History, Sessions and Now pages are untouched this session.
- Client on real, in-production sessions is still empty: the real burnmon.json has no owners rules configured yet, so ClientFor correctly has nothing to match; a real client rule plus burnmon-cli reown is what will populate it.
- The real local store was migrated (schema version 6 to 7) as a side effect of running the spot-check read against the real store file; this is the intended, additive migration path (same as every prior burnmon schema change), not a special action taken for this brief.
- The review agent did not run build.ps1 (only the Go test suite); the implementer ran build.ps1 separately, twice (before and after the fix pass), both green.
- Two Minor findings from the review were left as-is, not fixed: normalizeRemote does not reshape an ssh:// remote with an explicit port or a non-git SSH user (documented in its own comment instead); per-client active time is a plain sum of that client's sessions, so overlapping sessions (an agent subsession inside its own parent session) can push a client's total past real wall-clock time. Both are ledgered in SESSION_LOG.md as deferred.

## 4. Findings worth propagating
- [RESULT] Full test suite green: go test ./... -count=1 (20 packages, no failures) after the K1/K2 implementation, the review pass, and the fix pass.
- [RESULT] build.ps1 green (burnmon-cli.exe, burnmon.exe), run twice.
- [RESULT] Fresh Opus review (Agent tool, general-purpose agent, read-only on the checkout) found one Critical bug: a remote-only owner rule (no match path) made the old, independent OwnerFor match every path via an empty prefix, so a client's owner rule could leak onto every other project, including a Valona one. Confirmed by the reviewer in a throwaway scratch copy, then reproduced and fixed here with a new test (TestOwnerForRemoteOnlyRuleDoesNotMatchEveryPath) that fails on the pre-fix code and passes now.
- [RESULT] The same review found the git worktree case never actually worked (a worktree's .git file points at a directory with no config of its own, only a commondir pointing back at the real one) and that remote matching was a bare substring (a pattern for one repo also matched a sibling repo whose name has it as a prefix). Both fixed, both covered by new tests.
- [RESULT] Writing the test for the SSH-normalization ruling against the fixed remoteMatches function caught a second real bug in the fix itself (normalization was only applied on one side of the comparison); caught and fixed within the same TDD cycle, before it reached the log.
- [RESULT] Spot-check (K2, plan A8), three real sessions starting 2026-09-22 UTC, read from the real store: session 0fe88daa (259 calls, 06:54 to 08:54 UTC, 119.8 active minutes); session 7b13b2f8 (51 calls, 07:06 to 07:47 UTC, 41.4 active minutes); session agent-a2b5c1fb (16 calls, 07:07 to 07:16 UTC, 9.4 active minutes). No external time-log entry existed to compare against (see section 3). Note for K4/export work later: agent-a2b5c1fb's window sits entirely inside 0fe88daa's window, a real example of the overlapping-session gap noted above.
- [STATE] Migration numbering: the v0.3 spec text calls this migration 6; the codebase already has a real Version 6 entry (v0.2.1 session_id index), so this session implemented it as migration 7 instead and flagged the spec number as stale (not a live instruction) in SESSION_LOG.md.

## 5. Hub-level decision (if any)
Nothing. Project-internal implementation detail, no cross-project time, positioning or CIPHER call involved.

## 6. What the next hub read should update
Nothing in the hub canonical files (STATUS, DEADLINES, portfolio, decisions.md) needs to change from this brief alone: it is an in-project code milestone, not a cross-project or deadline-relevant event by itself. The v0.3 session prompts checklist is already ticked for V3-2. This Week / Mission Deck: not applicable, burnmon's v0.3 build order is tracked in its own session-prompts file, not the deck.

## 7. Open flags for next session
- Commit and tag are still Wilco's to run. Exact commands, from C:\ZND\projects\burnmon:

```
git add internal/pricing/pricing.go internal/pricing/gitremote.go internal/pricing/gitremote_test.go internal/pricing/pricing_test.go internal/schema/event.go internal/store/migrations/migrations.go internal/store/store.go internal/store/store_test.go internal/dataset/dataset.go internal/dataset/fromstore.go internal/dataset/fromstore_test.go internal/scan/types.go cmd/burnmon-cli/main.go burnmon.example.json SESSION_LOG.md "02_roadmap/2026-09-23_v0.3_session_prompts.md" "docs/superpowers/plans/2026-09-23-client-map-active-time.md" "04_assets/hub_agent_update_2026-09-23_v0.3_V3-2.md"
git commit -m "v0.3 V3-2: client map (K1) and active time (K2)"
git tag v0.3.0-alpha.1
```

  (git push and git push --tags are separate, deliberate steps, not included above.)
- No real client rule exists yet in the live burnmon.json, so client will read "unassigned" or empty on real sessions until one is added and burnmon-cli reown is run.
- Two Minor findings deferred (see section 3): the ssh:// port/user gap in normalizeRemote, and per-client active time not accounting for overlapping sessions.
- V3-3 (C3 dev and business switch, K3 client view, cost on every page, U1/U2 Now-page fixes) is next per the build order.

## 8. Related files
- Spec: C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_spec.md (sections 2.2 K1, K2)
- Plan: C:\ZND\projects\burnmon\docs\superpowers\plans\2026-09-23-client-map-active-time.md
- Prior brief: C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-23_v0.3_V3-1.md
- Session log: C:\ZND\projects\burnmon\SESSION_LOG.md
