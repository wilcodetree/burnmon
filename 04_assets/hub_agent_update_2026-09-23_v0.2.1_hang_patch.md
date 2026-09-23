# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 (v0.2.1 hang patch session) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** Report the v0.2.1 patch: BurnMon's Refresh went "Not Responding" (98.5% CPU,
1,438 MB, 175 MB/s disk); measured the real cause against the real store, fixed what the
numbers showed, and where the numbers still fall short of the stated target.
**Read order:** this file, `C:\ZND\projects\burnmon\STATUS.md`, `C:\ZND\projects\burnmon\SESSION_LOG.md`
**Supersedes:** nothing

## 1. Headline
Refresh's "Not Responding" and its pinned CPU are fixed and measured, not just patched on
faith: the real cause was not the three hypotheses the spec wrote down before measuring
(a slow `Collect`, the old v0.1 payload shape, or an unindexed history read on demand), it
was the Sessions tab firing one lookup per session, unconditionally, on every single page
load, all landing on the same UI thread in one uninterrupted burst. Fixed at the store
layer (a missing index, a separate read connection pool), the UI-binding layer (that lookup
now runs off the UI thread and resolves asynchronously), and the frontend (capped how many
run at once). Peak memory during Refresh dropped by roughly a third but did not reach the
400 MB target; flagged openly rather than claimed. Tests and build both green. Committed
locally, not pushed.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (local commits, not pushed): the hang patch
  itself, STATUS/roadmap/SESSION_LOG updates, this brief, version bump to `0.2.1`.
- **Files touched** (full paths):
  - `C:\ZND\projects\burnmon\internal\store\migrations\migrations.go` (migration 6: a
    `session_id`-only index; `schema_version` head now 6)
  - `C:\ZND\projects\burnmon\internal\store\store.go` (WAL mode, a five-second busy
    timeout, and a second `*sql.DB` handle: one writer connection as before, an
    eight-connection read pool for every read-only query)
  - `C:\ZND\projects\burnmon\internal\store\store_test.go` (schema_version expectations
    updated from 5 to 6 in both `TestFreshStoreAtHeadVersion` and
    `TestMigrateRealV01Store`, the latter run against the real store this session)
  - `C:\ZND\projects\burnmon\internal\dataset\dataset.go` (stage timing added to
    `Collect`/`ingest`; `Weeks` and the unused `CoverageNote` field removed from
    `Payload`, checked against every `D.*` reference in `template.html` first)
  - `C:\ZND\projects\burnmon\cmd\burnmon\main.go` (stage timing around every bound
    function; `bmSessionInsight` and `bmHistory` now return immediately and resolve
    through `w.Dispatch` from a goroutine instead of blocking the UI thread; new
    `asyncResolveJS` helper; a 400 MB `debug.SetMemoryLimit`; version to `0.2.1`)
  - `C:\ZND\projects\burnmon\internal\report\template.html` (`sessionInsight()` and
    `renderHistory()` adapted to the async resolve pattern; `renderSessions()`'s
    fan-out capped to 8 concurrent `bmSessionInsight` calls via a new
    `runWithConcurrency`)
  - `C:\ZND\projects\burnmon\cmd\burnmon-cli\main.go` (version to `0.2.1`)
  - `C:\ZND\projects\burnmon\STATUS.md`, `C:\ZND\projects\burnmon\02_roadmap\roadmap.md`,
    `C:\ZND\projects\burnmon\SESSION_LOG.md` (v0.2.1 recorded, v0.3 renumbered to item 5,
    its stale 2026-12-12 "Next" reference corrected to 2026-10-09)
  - `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-23_v0.2.1_hang_patch.md`
    (this file)
- **On a branch, not merged:** none; all work is on `main`, committed locally.
- **Untracked / outside a repo:** the session's own spec and prompt files
  (`02_roadmap\2026-09-23_v0.3_session_prompts.md`, `02_roadmap\2026-09-23_v0.3_spec.md`)
  were already untracked at session start from earlier v0.3 grill work, not from this
  session; left as Wilco's own in-progress files, not committed by this session.

## 3. What did NOT happen (and why)
- Not pushed to `origin/main`. Push and tag commands handed to Wilco below, per the
  session prompt's own instruction.
- `bmHistory` was NOT rewritten to a SQL aggregate query, despite the spec asking for
  exactly that. Reason: `bmHistory` never reproduced as the driver this session (the
  History tab was never open during any measured hang), and its vendor/owner dropdown
  options are built from the same unfiltered event pass its totals use; narrowing that
  read to the filter's own date range would have silently hidden a filter option outside
  the selected range, a real regression the measurements gave no reason to risk. Moved it
  to the async off-UI-thread pattern instead (same protection principle, no behaviour
  change), and recorded the reasoning in `SESSION_LOG.md` rather than silently doing the
  smaller thing.
- Peak memory during Refresh did NOT reach the 400 MB Done-when target: it dropped from
  the reported 1,438 MB (1.6 to 1.7 GB in this session's own pre-fix repro) to about
  900 MB. An isolated repro proved `Collect`'s own data is under 50 MB live even at this
  store's real scale, so the residual is not a further leak in `Collect`; a 400 MB
  `debug.SetMemoryLimit` was added as the most defensible lever available this session,
  and the gap is named openly in `STATUS.md` rather than claimed fixed.
- `Collect` on a warm store did NOT reliably stay under 5 seconds: it measured 1.2 to
  2.6 seconds in most runs, 5.9 seconds once, when the Sessions-tab fan-out was itself
  saturating the new read pool. Also named openly rather than claimed.

## 4. Findings worth propagating
- [RESULT] The spec's own H1 to H4 hypotheses were written before measuring and did not
  match what the numbers showed once instrumented: `Collect` itself was fast (1.2 to
  2.6 s) and the embedded JSON payload was small (850 KB), so the old v0.1-pipeline theory
  (H2) does not hold. The real driver was `internal\report\template.html`'s
  `renderSessions()` firing one `bmSessionInsight` call per kept session (1,192 to 1,719
  in the real store) unconditionally on every page load, each one landing synchronously
  on the WebView2 UI thread (confirmed by reading `github.com/jchv/go-webview2`'s own
  `Bind`/`callbinding` code), each around 60 ms, serialised, which is what produced both
  the CPU pin and the "Not Responding" title.
- [RESULT] `EventsForSession`'s `WHERE session_id = ?` could not use the existing
  `idx_events_session (vendor, session_id)` index, since `session_id` is not its leading
  column, so every one of those calls fell back to a full table scan. This explains H3's
  175 MB/s disk figure without needing WSL or WAL as the cause.
- [RESULT] `go test ./... -count=1` green across every package, including
  `TestMigrateRealV01Store` run against the real store (event count preserved, version 5
  to 6). `node --check` on the script extracted from `template.html` passed. `.\build.ps1`
  green (both exes built).
- [STATE] Local commits exist on `main` but are not pushed.

## 5. Hub-level decision (if any)
None needed this session. The memory shortfall is a project-internal open item, recorded
in BurnMon's own `STATUS.md` Known gaps, not a hub-level call.

## 6. What the next hub read should update
`10_holding\01_projects\burnmon.md` may want a one-line note that v0.2.1 shipped with one
open item (peak memory during Refresh still around 900 MB, target 400 MB), so it is not
re-surfaced as a surprise later.

## 7. Open flags for next session
- Push and tag `v0.2.1` to `origin/main`, Wilco's call; commands below.
- Peak memory during Refresh (about 900 MB, target under 400 MB) and the one observed
  5.9-second warm `Collect` remain open; candidates for a v0.3 follow-up or a small
  v0.2.2 patch, not solved further this session given the time already spent measuring.
- v0.3 scope (per-vendor cost, dev/business switch, full client map, macOS/Linux builds,
  now dated 2026-10-09) unchanged, still the next real work after this patch.

## 8. Related files
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.2.1_hang_patch.md` (this session's spec/prompt)
- `C:\ZND\projects\burnmon\STATUS.md`, `C:\ZND\projects\burnmon\SESSION_LOG.md`,
  `C:\ZND\projects\burnmon\02_roadmap\roadmap.md`
