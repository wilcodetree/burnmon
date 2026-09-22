# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-22 - **Owner:** Wilco de Tree
**Project:** BurnMon (forked from claudecost)
**Purpose:** BurnMon week 39 fork is complete: renamed, built, equality-tested, tagged v0.0.1 and pushed to a public GitHub repo.
**Read order:** this file, then `C:\ZND\10_holding\01_projects\burnmon.md`, then `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_burnmon_plan.md`
**Supersedes:** nothing

## 1. Headline
BurnMon v0.0.1 is forked from claudecost, renamed throughout, builds clean, reproduces claudecost's numbers, and is pushed to `https://github.com/wilcodetree/burnmon` (branch `main`, tag `v0.0.1`). Steps 1 through 7 of the week 39 handoff are done.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (local repo, remote `https://github.com/wilcodetree/burnmon.git`):
  - `6e1147d` Fork claudecost into burnmon, unchanged (v0.0.0 baseline) (done before this session, per the handoff's step 1).
  - `3d00f22` Rename claudecost to burnmon (v0.0.1): module, imports, folders, build, config, dashboard title.
- **Tag:** `v0.0.1` on `3d00f22`, pushed.
- **Pushed:** `main` and `v0.0.1` both on `origin` (`https://github.com/wilcodetree/burnmon`), confirmed by the push output (new branch `main`, new tag `v0.0.1`).
- **Files touched** (full paths, all under `C:\ZND\projects\burnmon\`):
  - `go.mod` (module `claudecost` to `burnmon`)
  - `cmd\claudecost\main.go` to `cmd\burnmon-cli\main.go` (folder rename, import paths, config filename `claudecost.json` to `burnmon.json`, cache dir, report filename prefix, version string)
  - `cmd\claudecost-app\main.go` and `main_test.go` to `cmd\burnmon\main.go` and `main_test.go` (folder rename, import paths, window title `Claude Cost` to `BurnMon`, mutex name, app data dir, log filename, warming page title)
  - `internal\agg\agg.go`, `internal\scan\parse.go`, `internal\dataset\dataset.go`, `internal\pricing\pricing.go` (import paths and doc comments)
  - `internal\report\template.html` (dashboard `<title>` and `<h1>` to BurnMon, CLI/exe name references in the rebuild notice and refresh-error strings, EN and NL)
  - `build.ps1` (output exe names, go-winres target folders)
  - `winres\app.json`, `winres\cli.json` (product name, internal name, original filename fields)
  - `.gitignore` (ignore `burnmon.exe` / `burnmon-cli.exe` instead of the old names)
  - `README.md` (new top section: what BurnMon is, v0.0 equals claudecost, plan link, origin credit; the rest of claudecost's README kept verbatim, by explicit scope in the handoff, so it still says `claudecost` and `claudecost.json` in places)
  - `AGENTS.md`, `DEADLINES.md` (rewritten as short pointer files; DEADLINES now carries the five plan dates)
  - `LICENSE` untouched (MIT, Wilco de Tree)

## 3. What did NOT happen (and why)
- `claudecost.example.json` was left named and worded as-is (not renamed to `burnmon.example.json`), since it was not in the handoff's explicit step 2 rename list and README's body (kept verbatim per step 5) still points at that exact filename.
- README's body sections (CLI usage, Build, Configuration) were not rewritten to say `burnmon`/`burnmon.json`; the handoff was explicit that only the top two paragraphs change this week, doc polish is deferred.
- No schema, SQLite store, or Codex adapter work: out of scope for week 39, per the handoff's "Not this week" list.
- The equality test did not produce a byte-identical JSON diff on the first pass; see section 4 for why, and how it was still verified as a rename-only change, not a logic change.
- `hub-update` was not run; this brief only writes the record, propagation into STATUS/DEADLINES/portfolio is the next hub session's job.

## 4. Findings worth propagating
- [RESULT] `go build ./...` and `go vet ./...` both exit clean on the renamed module (verified live this session, not from a plan).
- [RESULT] `burnmon-cli.exe -months 2 -json` and `claudecost-cli.exe -months 2 -json`, run about 9 seconds apart, produced a diff (`generated_at` timestamp, three sessions with a few extra tool calls and cost, one pair of adjacent sessions swapped order) that traces to live transcript growth, not the rename: a control test running the unmodified `claudecost-cli.exe` twice in a row, 1 second apart, produced 348 diff lines of the identical shape (the same live session's transcript growing between runs, this very conversation's own transcript among the sources scanned). This confirms the rename did not change any computed number; it is not a byte-for-byte static-fixture proof, since neither binary was pointed at a frozen source snapshot.
- [RESULT] Repo is public at `https://github.com/wilcodetree/burnmon`, `main` and tag `v0.0.1` both pushed, confirmed by push output.
- [STATE] BurnMon plan week 40 (schema and SQLite store) is next, due 2026-10-03 per the plan's timeline.

## 5. Hub-level decision (if any)
Nothing. This is a project-internal fork/rename step, no cross-project time, park/unpark, consultancy, positioning, or CIPHER-wall call was made.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md`: repo now exists and is pushed; update the "no code yet" line in Current status.
- `C:\ZND\10_holding\02_roadmap\roadmap.md`: BurnMon side-track pointer already present per the plan's section 9; confirm it reflects week 39 as done.
- `C:\ZND\10_holding\04_engineering\migration\2026-09-08_znd_migration_map.json`: if this map tracks per-project repo state, BurnMon now has its own repo separate from claudecost.
- `C:\ZND\projects\claudecost\README.md`: the plan (section 9) says claudecost's README should gain a pointer to BurnMon now that the repo exists; not done this session, flagged for the hub or a claudecost-side session.

## 7. Open flags for next session
- claudecost's own README does not yet point at BurnMon (see section 6).
- The equality test relied on a live-vs-live comparison plus a control run, not a frozen fixture; if a hub reader wants a stricter byte-for-byte proof later, that needs a static transcript snapshot and `-source` pointed at it.
- Week 40 (schema, SQLite store, Claude adapter onto the schema) is the next plan row, due 2026-10-03.
- The estate rename window (2026-09-28 to 2026-10-11) may move `C:\ZND\projects\*`; BurnMon was forked before it, per the plan's risk register.

## 8. Related files
- `C:\ZND\10_holding\handovers\2026-09-22_burnmon_week39_handoff.md` (the handoff this session executed)
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_burnmon_plan.md`
- `C:\ZND\projects\burnmon\04_assets\2026-09-22_token_monitor_architecture.md`
- `C:\ZND\10_holding\01_projects\burnmon.md`
- `https://github.com/wilcodetree/burnmon` (repo, tag `v0.0.1`)
