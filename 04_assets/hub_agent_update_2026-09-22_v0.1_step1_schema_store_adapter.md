# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-22 (subagent-driven-development session, worktree-isolated) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** v0.1 Step 1 (schema, store, Claude adapter) is code-complete and verified on a worktree branch, not yet merged, tagged or pushed.
**Read order:** this file, then `C:\ZND\projects\burnmon\docs\superpowers\plans\2026-09-22-v0.1-step1-schema-store-adapter.md`, then `C:\ZND\projects\burnmon\SESSION_LOG.md`
**Supersedes:** nothing

## 1. Headline

BurnMon v0.1 Step 1 (schema, store, Claude adapter, per `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1_spec.md`) is code-complete on branch `worktree-burnmon-v0.1-step1`: `go test ./...` and `.\build.ps1` are green, and `burnmon-cli.exe report` built from the new SQLite store matches a v0.0.1 baseline binary byte-for-byte on Wilco's real two months of transcripts, apart from one sign-off'd surface-vocabulary change. Not merged into `main`, not pushed, not tagged.

## 2. What changed on disk

- **Committed** on branch `worktree-burnmon-v0.1-step1` (not on `main`), in worktree `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-v0.1-step1`, 15 commits on top of the v0.0.1 baseline (`0393939`):
  - `848c16f` feat(schema): add vendor-agnostic Event type
  - `69bf1ec` feat(store): add SQLite event/cursor/meta store
  - `15a5abc` feat(adapter): add the Adapter interface
  - `a5e9b40` feat(adapter/claude): move Claude/Cowork parsing behind the adapter interface
  - `c6e56b3` refactor(scan): keep Session/PerModel/SurfaceLabel/ToolGroup in scan, generic surface vocabulary
  - `eb99d28` fix(adapter/claude): keep skill: prefix on Event.Tools keys, remove dead code
  - `911328b` feat(dataset): rebuild scan.Session from stored Events
  - `73a8ba6` refactor(dataset): rewire Collect around the store, remove the gob parse cache
  - `61342d8` refactor(cli): use the store instead of the gob parse cache
  - `86d803f` refactor(app): use the store instead of the gob parse cache, drop the settings-save cache reset
  - `d35c9e3` chore: go mod tidy, mark modernc.org/sqlite as a direct dependency
  - `6243300` fix(store): scope the events dedup key per session, not globally
  - `cfbaaeb` fix(dataset): preserve millisecond precision in session start/end timestamps
  - `36feb49` fix: address final review findings (incremental read data loss, schema version, event pruning, and 4 more)
  - `fc8aff8` docs: SESSION_LOG entry for v0.1 Step 1
- **On a branch, not merged:** `worktree-burnmon-v0.1-step1` carries all of the above; `main` is unchanged at `0393939`.
- **Files touched** (full paths, all under `C:\ZND\projects\burnmon\`): `internal\schema\event.go`, `internal\store\store.go`, `internal\adapter\adapter.go`, `internal\adapter\claude\claude.go` (plus `partial_test.go`, `testdata\basic.jsonl`), `internal\scan\types.go` (new), `internal\scan\parse.go` (deleted, content moved), `internal\dataset\fromstore.go` (new), `internal\dataset\dataset.go`, `cmd\burnmon-cli\main.go`, `cmd\burnmon\main.go`, `go.mod`, `go.sum`, `SESSION_LOG.md`, `internal\report\template.html`, `README.md`.
- **Untracked / outside a repo:** none. The implementation plan lives at `C:\ZND\projects\burnmon\docs\superpowers\plans\2026-09-22-v0.1-step1-schema-store-adapter.md`, committed on the branch.

## 3. What did NOT happen (and why)

- Not merged into `main`. Not pushed. Not tagged. Per the spec's own rule and this session's instructions, commit/tag/push are Wilco's manual step; the session stopped at a green, verified build with the tag command printed.
- The `v0.1.0-alpha.1` tag was NOT created. Exact command to run once merged: `git tag -a v0.1.0-alpha.1 -m "Step 1: schema, store, Claude adapter"`.
- Step 2 (Codex adapter, week 41) and Step 3 (the Now page, week 42) were not started; this brief covers Step 1 only.
- The old `%LOCALAPPDATA%\burnmon\parsecache.gob` cache file is now cleaned up by the app/CLI on startup (best-effort delete), but any machine that has not run the new binary yet still has it sitting on disk until it does.
- Four minor findings from the final code review were deliberately left unfixed (parked, not forgotten): a chunk-relative synthetic dedup-key fallback in the Claude adapter (theoretical today, zero occurrences in 48,085 live events), a strict-greater-than-vs-greater-or-equal mismatch between the store's upsert guard and the adapter's in-memory dedup (means a tied-output re-ingest does not refresh metadata-only changes), `Session.Surface` sourced from `events[0]` rather than the first non-empty value (inconsistent with how `Title`/`CWD` are derived, harmless on current data), and the synthetic-model filter moving from after-dedup to before-dedup versus v0.0.1 (likely a correctness improvement, unremarked semantics change). All four are noted in `SESSION_LOG.md`'s entry for follow-up in Step 2/3.

## 4. Findings worth propagating

- [RESULT] `go test ./...` passes across every package (`internal\schema`, `internal\store`, `internal\adapter\claude`, `internal\dataset`, `internal\scan`, `cmd\burnmon`); `.\build.ps1` builds both `burnmon-cli.exe` and `burnmon.exe` clean, `CGO_ENABLED=0` preserved throughout (verified: every new dependency, `modernc.org/sqlite` and its transitive deps, is pure Go).
- [RESULT] Real-data verification: built a v0.0.1 baseline binary from commit `3d00f22` in a separate worktree, ran it and the new binary with `-no-cache` (forcing a fresh parse on both sides) against the same real Claude Code/Cowork transcripts on Wilco's machine, and byte-diffed the JSON report payloads. Final result after two bug fixes: zero differences, apart from the one sign-off'd surface-vocabulary change (see below).
- [RESULT] Two real bugs were found this way, both fixed and re-verified:
  1. The SQLite store's original dedup key was `(vendor, request_id)`, a global key exactly as the spec's `Event.RequestID` doc comment specified. Cowork mirrors a conversation's transcript across several session-id files sharing the same request IDs; the global key silently reassigned shared events to whichever file's ingest ran last, corrupting both sessions' totals (one real session collapsed from 67 calls to 2). Fixed by scoping the key to `(vendor, session_id, request_id)`, restoring v0.0.1-equivalent behavior (cross-session duplicate detection is dataset's `dedupSessions` job again, not the store's).
  2. A final whole-branch code review (dispatched on the most capable model available, after the byte-diff was already clean) caught a second, more serious bug the byte-diff structurally could not see: the incremental adapter reader (`bufio.Scanner`) was consuming a trailing, not-yet-complete line and advancing its cursor past it, permanently losing that line's turns once the file finished being written moments later. This is exactly the live-watch code path Step 3 depends on. Fixed by rewriting the reader around `bufio.Reader.ReadString`, which only advances past a genuinely newline-terminated line; proven by a new offset-accounting regression test.
- [STATE] Surface vocabulary changed intentionally (Wilco's sign-off, recorded in the plan): `cowork`/`chat` merged into `desktop`, `code` renamed to `cli`, `code_agent` kept as its own bucket. This is the one acknowledged diff from v0.0.1's report JSON.

## 5. Hub-level decision (if any)

Nothing. The dedup-key-scoping and incremental-reader decisions were project-internal engineering calls within BurnMon, made with Wilco's sign-off inside this session; none touch cross-project time, park/unpark, consultancy/talks, company positioning/pricing/voice, or the CIPHER anonymity wall.

## 6. What the next hub read should update

Nothing yet. This branch is not merged, so BurnMon's status in `C:\ZND\10_holding\01_projects\burnmon.md` and `C:\ZND\10_holding\02_roadmap\roadmap.md` should stay exactly as it reads now until Wilco merges and tags. Once he does, the hub one-pager should note v0.1.0-alpha.1 shipped (Step 1 of 3) and that Step 2 (Codex adapter) is next.

## 7. Open flags for next session

- Wilco to review the branch, merge into `main`, run the tag command, and push, at his own pace.
- Step 2 (Codex adapter, week 41) is next per the spec; it will reopen the store's `events` table (already versioned via a new `schema_version` meta row added during this session's final review fix, so a future migration has a real hook to key off).
- The four parked minor findings from section 3 are candidates to fold into Step 2's own review pass, not urgent on their own.

## 8. Related files

- Spec: `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1_spec.md`
- Plan: `C:\ZND\projects\burnmon\docs\superpowers\plans\2026-09-22-v0.1-step1-schema-store-adapter.md`
- Architecture note: `C:\ZND\projects\burnmon\04_assets\2026-09-22_token_monitor_architecture.md`
- Session log: `C:\ZND\projects\burnmon\SESSION_LOG.md`
