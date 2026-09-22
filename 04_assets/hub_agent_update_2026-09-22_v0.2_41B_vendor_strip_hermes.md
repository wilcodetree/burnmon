# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-22 (session 41B) - **Owner:** Wilco de Tree
**Project:** BurnRate (BurnMon)
**Purpose:** Tell the hub that v0.2's vendor strip (P3) and Hermes adapter (A1) shipped and tagged, and that A1's design changed from the spec's own assumption after a live schema check.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry, "2026-09-22, v0.2 41B"), then `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` sections 2.2 P3 and 2.5 A1.
**Supersedes:** nothing.

## 1. Headline
Shipped and tagged `v0.2.0-alpha.2`: the Now page's per-vendor token strip (P3) and the Hermes adapter (A1). Committed, tagged and pushed by Wilco. Both `go test ./... -count=1` and `.\build.ps1` are green.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon`: `c53487c` "feat: vendor strip on Now, Hermes adapter as growing per-session events (P3, A1)". Tag `v0.2.0-alpha.2` on that commit, pushed to `origin/main` with tags.
- **Files touched** (full paths):
  - `C:\ZND\projects\burnmon\internal\vendorstrip\vendorstrip.go` and `vendorstrip_test.go` (new package)
  - `C:\ZND\projects\burnmon\internal\store\store.go` (new `VendorStripTotals` query)
  - `C:\ZND\projects\burnmon\internal\adapter\hermes\hermes.go` and `hermes_test.go` (new package)
  - `C:\ZND\projects\burnmon\cmd\burnmon\main.go` (bound `bmVendorStrip`, added `startHermesPoll`)
  - `C:\ZND\projects\burnmon\internal\report\template.html` (vendor strip table + 60s poll; no History/tab changes)
  - `C:\ZND\projects\burnmon\testdata\hermes\hermes_fixture.db` (new, metadata-only fixture)
  - `C:\ZND\projects\burnmon\SESSION_LOG.md`, `02_roadmap\2026-09-22_v0.2_session_prompts.md` (checklist ticks for 40B, 41A, 41B, the latter two of which had shipped and tagged in prior sessions but were never ticked)
- **On a branch, not merged:** nothing; straight to `main`, matching this project's own convention.
- **Untracked / outside a repo:** nothing left over from this session.

## 3. What did NOT happen (and why)
- A2 (Copilot CLI adapter) and A3 (Copilot VS Code OTel check) did not start. GitHub Copilot CLI is confirmed installed on this laptop (`copilot.exe`, via WinGet) but has never been run: `%LOCALAPPDATA%\github-copilot\` holds only `auth.db` (login token), no session or usage data. Wilco was asked to run one short real Copilot CLI session first; as of this brief he has not yet confirmed doing so.
- The commit and tag were not made by the assistant: this project forbids AI git writes on `C:\ZND` (stale `index.lock` risk), so the assistant printed the commit/tag/push commands and Wilco ran them himself.
- No forecast, insight (I-series) or Copilot work touched; those are weeks 42-43 per the build order.

## 4. Findings worth propagating
- [RESULT] P3 (vendor strip): built as one SQL query in `Store.VendorStripTotals`, grouped by agent with day/week/month `CASE` buckets (UTC calendar, ISO week starting Monday), bound as `bmVendorStrip`, polled once a minute on its own JS timer, independent of the Now page's 2-second `bmLive` poll. Verified with a real store fixture (`internal\vendorstrip\vendorstrip_test.go`).
- [RESULT] A1 (Hermes) schema check, live on Wilco's laptop, `%LOCALAPPDATA%\Hermes\state.db`: `messages.token_count` is null on every row observed, 24 of 24, across every role (user/assistant/tool), on both a stale session and two fresh ones Wilco recorded live when asked. This contradicts the v0.2 spec's "per-message token count" assumption; there is no per-message token breakdown available in Hermes's schema at all. The only real numbers are `sessions`-table running totals (input/output/cache_read/cache_write/reasoning) that grow in place as a session continues. No context-window column exists anywhere.
- [RESULT] Given that finding, the adapter's shape changed from "one Event per message" to "one Event per session, growing" (put to Wilco as a two-way choice; he chose this one). `RequestID` is fixed to the session id, so the store's existing "largest output wins" upsert does the dedup/growth work; the adapter needs no cursor of its own, and does not use `watch.Watcher` (that package is hardcoded to `.jsonl` files; a SQLite WAL file does not fit it), running its own 5-second poll goroutine instead, matching the spec's own "5-second poll, no fsnotify".
- [STATE] Known, accepted trade-off, not fixed this session: because Hermes's Event fields carry lifetime running totals rather than one turn's consumption, the Now page's per-session context gauge (built for "last turn's context fill") would read as ever-growing lifetime usage for a long Hermes session rather than current context fill. Low practical impact in v0.2 since Hermes also has no context-window entry, so the gauge already falls back to "context window unknown" for it.
- [STATE] Copilot CLI's on-disk session/usage data location is still unconfirmed on this laptop; the spec's `data.db` assumption (A3 in the original plan, not this session's A3) has not yet been checked against a real file, since none exists until Wilco runs the CLI once.

## 5. Hub-level decision (if any)
Nothing. The "one Event per session, growing" call is project-internal (BurnMon's own adapter design), not a cross-project, positioning, pricing, voice or CIPHER-wall decision.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` (project one-pager): note P3 and A1 shipped, `v0.2.0-alpha.2` tagged, A2/A3 pending Wilco's Copilot CLI recording.
- `C:\ZND\10_holding\02_roadmap\roadmap.md`: no change to BurnMon's position in the roadmap, still item per its existing entry; only the in-progress detail moves.
- No DEADLINES.md change: 2026-11-14 (v0.2.0) is unaffected, this was on-pace (two sessions a week, week 41 of 39-46).

## 7. Open flags for next session
- Wilco needs to run a short real GitHub Copilot CLI session before 43B (A2) can start; the assistant will then search the laptop for whatever file(s) it wrote, the same discovery pattern used for Hermes.
- VERIFY items still open, carried from the spec: Claude Code cache TTL, Claude Code auto-compact threshold, Copilot CLI `data.db` layout, Copilot VS Code OTel export, Codex subagent structure.
- Sessions 42A/42B (insight package: re-prefill, compaction, context runway, expensive-turn) are next per the build order, untouched by this brief.

## 8. Related files
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` (sections 2.2 P3, 2.5 A1)
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (checklist, 41B line)
- `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry, full technical detail)
- Prior briefs in the same folder: `hub_agent_update_2026-09-22_v0.2_41A_history_page.md`, `..._40B_five_tabs.md`, `..._40A_tool_calls.md`, `..._39B_migrations_owner.md`
