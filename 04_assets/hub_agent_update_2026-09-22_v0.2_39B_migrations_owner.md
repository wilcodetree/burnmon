# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-22 (session 39B) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** BurnMon's first v0.2 build session shipped: versioned store migrations and the P6 owner column, committed to `main`.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry), then `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` sections 2.1 S1 and 2.2 P6.
**Supersedes:** nothing (first v0.2 brief; last prior brief was `hub_agent_update_2026-09-22_v0.1.2_patch.md`).

## 1. Headline
Session 39B of the v0.2 build (`2026-09-22_v0.2_session_prompts.md`) is done and merged to
`main`: a real migration runner replaced the old drop-and-rebuild schema path, and P6's owner
split (light client map, empty by default) is wired from config through ingest to a new
`burnmon-cli reown` command. Shipped, not just code-complete: committed on `main` at `30761ef`.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (`main`): `30761ef` "feat: versioned store
  migrations, owner column and reown (S1, P6)". 15 files changed, 616 insertions, 89 deletions.
- **On a branch, not merged:** none, this landed straight on `main` per the session's own
  house rule (project uses no per-task branches).
- **Files touched** (full paths):
  - `C:\ZND\projects\burnmon\internal\store\migrations\migrations.go` (new)
  - `C:\ZND\projects\burnmon\internal\store\store.go` (migration runner, `ReownEvents`, owner
    column plumbing; removed the old `SchemaVersion`/`ensureSchemaVersion` drop-and-rebuild path)
  - `C:\ZND\projects\burnmon\internal\store\store_test.go` (migration and reown tests)
  - `C:\ZND\projects\burnmon\internal\pricing\pricing.go` (`OwnerRule`, `Config.Owners`,
    `Config.OwnerFor`)
  - `C:\ZND\projects\burnmon\internal\pricing\pricing_test.go` (new)
  - `C:\ZND\projects\burnmon\internal\schema\event.go` (`Event.Owner`)
  - `C:\ZND\projects\burnmon\internal\scan\types.go` (`Session.Owner`)
  - `C:\ZND\projects\burnmon\internal\dataset\dataset.go` (`ingest`/`IngestFile` take
    `*pricing.Config`, stamp `Owner` at ingest)
  - `C:\ZND\projects\burnmon\internal\dataset\dataset_test.go`, `fromstore.go`
  - `C:\ZND\projects\burnmon\cmd\burnmon-cli\main.go` (`reown` subcommand)
  - `C:\ZND\projects\burnmon\cmd\burnmon\main.go`, `main_test.go` (live-watch call site
    threading a config copy through)
  - `C:\ZND\projects\burnmon\SESSION_LOG.md` (paragraph prepended, after the file header)
  - `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (39B ticked)
- **Untracked / outside a repo:** none left over; the new migrations package and pricing test
  file above are both now committed, not stray.

## 3. What did NOT happen (and why)
- Not tagged: this was a mid-week build session, not a release point; the checklist's tag
  markers start at 40B (`v0.2.0-alpha.1`).
- Not pushed to any remote: the session stopped before push, per the prompt's own instruction
  ("stop before committing" was the literal ask; push was never requested).
- No UI for owners in this session, by explicit instruction: `Session.Owner` exists in the Go
  payload but nothing in `internal\report\template.html` renders it yet. That is 44A's Sessions
  tab item, not this one.
- `burnmon.json` on this laptop still has an empty `owners` table (the default), so `reown`,
  run once by hand against the real store to smoke-test it, reported 0 rows changed. No real
  owner split has actually been configured or exercised end to end yet.
- One unrelated file, `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1.2_patch_spec.md`, was
  already modified in the working tree before this session started (an earlier F8 wording
  edit) and was deliberately left out of this commit; it is still sitting uncommitted on disk
  and belongs to whichever session made that edit, not this one.
- No secret was on screen or handled in this session; nothing to note here beyond that.

## 4. Findings worth propagating
- [RESULT] `go test ./... -count=1` green across all packages, and `.\build.ps1` green
  (`burnmon-cli.exe`, `burnmon.exe`), confirmed by direct run in this session.
- [RESULT] `TestMigrateRealV01Store` (`C:\ZND\projects\burnmon\internal\store\store_test.go`)
  copied the actual `%LOCALAPPDATA%\burnmon\burnmon.db` on this laptop (33 MB, a genuine v0.1
  store with no `schema_version` table and no `owner` column) into a temp dir and ran it to
  head: every event survived with an identical `(vendor, session_id, request_id)` key, and
  `schema_version` read 2 afterward. This is a live measurement against a real file, not a
  synthetic fixture.
- [STATE] Running `burnmon-cli.exe reown` once by hand against the real store on this laptop
  migrated it live from schema version 0 to 2 (the same migration path a normal app or CLI
  startup will now take on next run). This is expected, intended behavior, not an incident;
  flagging it only so a later hub read does not need to ask why this laptop's real store's
  on-disk schema version changed outside of a normal app launch.
- [STATE] Session prompts checklist: 39B is the only item ticked this session. 40A (tool_calls,
  `burnmon-cli tools --json`, windowed CLI live) is next, per
  `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md`.

## 5. Hub-level decision (if any)
Nothing. This is a project-internal implementation decision (owner column lives on `events`,
not a separate `sessions` table, since BurnMon has never materialized sessions) and does not
touch cross-project time, park/unpark, consultancy/talks, company positioning, or CIPHER.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` (hub one-pager): if it names v0.2 progress by
  session letter, this is 39B done, next is 40A.
- No `STATUS.md`/`DEADLINES.md`/`roadmap.md` change expected from this one session; those move
  at tag points (40B onward per the checklist), not mid-week build sessions.

## 7. Open flags for next session
- 40A (tool_calls table, `burnmon-cli tools --since 30d --json`, CLI `live` using the windowed
  `EventsSince`/`SessionTotals` path instead of `AllEvents`) is next per the session prompts
  file.
- The pre-existing uncommitted edit to `2026-09-22_v0.1.2_patch_spec.md` is still sitting on
  disk, untouched by this session; whoever owns that edit should commit or discard it.
- Owner rules in `burnmon.json` are still empty on this laptop; the owner split has code but no
  real-world exercise yet (see section 3).

## 8. Related files
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` (sections 2.1 S1, 2.2 P6)
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (39B entry and
  checklist)
- `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry, 2026-09-22 v0.2 39B)
- `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.1.2_patch.md` (prior brief)
