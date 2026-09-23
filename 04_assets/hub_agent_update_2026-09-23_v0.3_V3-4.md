# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 (v0.3 session V3-4) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** V3-4 (export K4, merge K5) is code-complete on main, tested and run for real on this laptop, not yet committed or tagged; hand-off for whoever commits and tags `v0.3.0-alpha.2` next.
**Read order:** this file, C:\ZND\projects\burnmon\SESSION_LOG.md (top entry)

## 1. Headline
`burnmon-cli export` and `burnmon-cli merge` are both implemented, tested, and run for real
against this laptop's own store. Export groups real turns into day/owner/client/vendor/model
rows (five token classes, cost on every basis C2's `CostForEvents` covers, active minutes
recomputed per row), refuses to run without `--owner` when owner rules are configured, defaults
its label to the literal `"dev-1"` (never the hostname), and carries no path, session id, prompt
or project name, guarded by a dedicated leak-scanning test. Merge combines any number of export
files into `merged.json` (one column per label, totals per client/vendor/ISO week, no cap,
refuses a schema mismatch) plus a static `report.html` rendered entirely server-side with Go's
`html/template`, no `<script>` tag at all. Fully tested (`go vet ./...`, `go test ./... -count=1`,
`.\build.ps1`, all green) and exercised for real: an export with `--owner ZND` on this laptop's
real store produced 32 rows, all owner `"ZND"`, zero occurrences of "Valona" and zero path
separators in the file; merged against a second export under a different label, opened
`report.html` in the default browser, three plain tables matched the JSON.

## 2. What changed on disk
- Committed: nothing yet. Every change below is an uncommitted working-tree edit in
  C:\ZND\projects\burnmon (branch main).
- New packages (full paths):
  - C:\ZND\projects\burnmon\internal\export\export.go, export_test.go (K4)
  - C:\ZND\projects\burnmon\internal\merge\merge.go, merge_test.go (K5)
  - C:\ZND\projects\burnmon\internal\mergereport\mergereport.go, mergereport_test.go (K5's
    report.html, static tables, no chart, no script)
- Existing files touched:
  - C:\ZND\projects\burnmon\internal\dataset\fromstore.go (new exported
    `ActiveTimeMinutes(events, cfg)`, the same K2 gap-sum algorithm applied directly to an
    arbitrary event slice rather than a whole session, for export's per-row active minutes)
  - C:\ZND\projects\burnmon\cmd\burnmon-cli\main.go (`export` and `merge` subcommands wired into
    `main()`'s dispatch, `runExport`, `runMerge`)
  - C:\ZND\projects\burnmon\SESSION_LOG.md (V3-4 paragraph)
  - C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_session_prompts.md (V3-4 ticked)
- Nothing touched in C:\dev\Work.

## 3. What did NOT happen (and why)
- Not committed, not tagged, not pushed. House rule: git writes on C:\ZND are a manual step for
  Wilco. Exact commands below.
- No `burnmon.json` with owner rules existed anywhere on this laptop before this session (checked
  both locations the app/CLI read). To exercise `--owner` meaningfully, a temporary `burnmon.json`
  (owner rules matching the house tree: `C:\dev\Work\* -> Valona`, `C:\ZND\* -> ZND`) was dropped
  in the session's scratchpad only, never in the repo or next to a deployed exe, and used with
  `burnmon-cli reown` once to backfill `Owner` on the real store's 51,795 events. This is a real,
  intended mutation of the local `%LOCALAPPDATA%\burnmon\burnmon.db`, not a repo change; it broke
  `TestMigrateRealV01Store` (which snapshot-compares that same real store and expects `Owner ==
  ""`, the pre-K1 state), caught by the full `go test ./... -count=1` pass, and was reverted in
  the same session by running `reown` once more against an empty-`owners` config (P6's documented
  "empty means no owner column at all"); the test was confirmed green again before stopping. No
  permanent `burnmon.json` was left on the machine.
- The second, Valona-inclusive export made to give merge a second, different-totals file to
  combine (`--owner ZND --owner Valona --label dev-2`) stayed in the session scratchpad only and
  was never published anywhere, per the wall rule.

## 4. Findings worth propagating
- [FIX] `runMerge`'s positional/flag argument split had a real bug, found only by running the
  built exe, not by the package's own unit tests: it recognised a token as "the value of a flag"
  purely by not starting with `-`, so `merge a.json b.json --out DIR` mistook `DIR` for a third
  positional file and errored `flag needs an argument: -out`. Fixed by consuming the token right
  after any `-`-prefixed argument as that flag's own value. `cmd\burnmon-cli\main.go`.
- [FINDING] Active time (K2) is naturally a per-session figure; K4 asks for it per
  day/owner/client/vendor/model row instead, which one running session's events can straddle
  (a model switch mid-session, a UTC day boundary). Resolved by recomputing K2's gap-sum algorithm
  directly on each row's own event subset (new `dataset.ActiveTimeMinutes`) rather than trying to
  allocate a whole session's `ActiveMinutes` across the rows it touches; simplest option that
  stays consistent with how tokens and cost are already aggregated per bucket everywhere else in
  this codebase (`internal/history.Build` does the same thing for its buckets).
- [DECISION, not asked, made for simplicity] K5's report.html has no chart: "totals per client,
  vendor and week" reads as three plain tables, and the spec's "inline Chart.js ... if a chart is
  needed" reads as conditional. Skipping it means report.html needs zero embedded script at all
  (pure `html/template` output), which is a stronger, simpler answer to "no network script" than
  vendoring Chart.js and proving it never phones home.

## 5. Hub-level decision (if any)
Nothing. Project-internal implementation detail, no cross-project time, positioning or CIPHER
call involved.

## 6. What the next hub read should update
Nothing in the hub canonical files (STATUS, DEADLINES, portfolio, decisions.md) needs to change
from this brief alone. The v0.3 session prompts checklist is already ticked for V3-4.

## 7. Open flags for next session
- Commit and tag are still Wilco's to run. Exact commands, from C:\ZND\projects\burnmon:

```
git add internal/export internal/merge internal/mergereport internal/dataset/fromstore.go cmd/burnmon-cli/main.go SESSION_LOG.md "02_roadmap/2026-09-23_v0.3_session_prompts.md" "04_assets/hub_agent_update_2026-09-23_v0.3_V3-4.md"
git commit -m "v0.3 V3-4: export (K4) and merge (K5)"
git tag v0.3.0-alpha.2
```

  (git push is a separate, deliberate step, not included above.)
- V3-5 (Copilot VS Code A4, macOS/Linux builds B1) is next per the build order; V3-5's prompt
  needs Wilco to first copy his real Copilot VS Code OTel span file, prompt text stripped, into
  `C:\ZND\projects\burnmon\testdata\copilotvsc\`.

## 8. Related files
- Spec: C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_spec.md (section 2.2 K4/K5)
- Wall rule: C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md (section 2.2 P6)
- Prior brief: C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-23_v0.3_V3-3b.md
- Session log: C:\ZND\projects\burnmon\SESSION_LOG.md
