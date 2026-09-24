# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-24 (V3-6, tag v0.3.0) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** report v0.3.0 shipped: release-candidate Done-when/VERIFY pass against the
built exe and the real local store, one real bug found and fixed along the way, version
constants set, docs and roadmap updated, commit and tag done, push command printed for
Wilco.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry,
V3-6), then `02_roadmap\2026-09-23_v0.3_spec.md` sections 5 and 6 for what was checked.
**Supersedes:** nothing.

## 1. Headline

BurnMon v0.3.0 shipped, 15 days ahead of the 2026-10-09 date. All seven spec
section-5 Done-when items pass against the built exe and the real local store; all six
section-6 VERIFY items resolved to a stated source and date, or an explicit
still-unverified label where research could not close them. One real, reproducible bug
was found and fixed along the way (Hermes's macOS/Linux default data path was wrong,
confirmed against Hermes's own live docs), and one stale local test artifact was found
and cleared (a leftover `burnmon.json` forcing monitor mode, which had made the v0.3-3b
click-driven check unrunnable on 2026-09-23). One real, reproducible product bug was
found and left open, deliberately not fixed (Now chart's cost-axis toggle), since it
predates this release and a real fix needs a dedicated Chart.js debugging session, out of
scope for a release-candidate pass. Committed and tagged `v0.3.0` locally; not pushed,
command below, Wilco's own step.

## 2. What changed on disk

- `internal\adapter\hermes\hermes.go`: `DefaultDBPath` rewritten. Was guessing
  `~/Library/Application Support/Hermes/state.db` (darwin) and
  `$XDG_DATA_HOME/Hermes/state.db` (linux), the per-OS app-data convention other
  BurnMon paths use; Hermes's own docs (confirmed live) say it never follows that
  convention, it is always `~/.hermes` (or `$HERMES_HOME`) on every non-Windows OS. Now
  checks `$HERMES_HOME` first, then `~/.hermes` (darwin/linux) or `%LOCALAPPDATA%\Hermes`
  (Windows, unchanged, confirmed live since v0.2). Two new tests.
- `internal\adapter\hermes\hermes_test.go`: `TestDefaultDBPathHermesHomeOverride`,
  `TestDefaultDBPathNoInstallIsEmpty`.
- `cmd\burnmon\app.go`: `version` constant `0.2.3` to `0.3.0`.
- `cmd\burnmon-cli\main.go`: `version` constant `0.2.3` to `0.3.0`.
- `README.md`: rewritten for v0.3: dev/business mode and cost basis, the extended
  owner/client map and active time, monitor mode, export and merge, two new
  `burnmon-cli` command lines, a new "The Groundwork Kit" section (install, set
  `"mode": "business"`, run export/merge on a cadence, written generically rather than
  naming a specific Kit client since this is a public README), and the default-roots
  table corrected for Hermes and Copilot CLI (both were marked VERIFY, both now
  confirmed); "Not yet there (v0.3)" section removed since it now is.
- `STATUS.md`: rewritten in full for v0.3's shipped state, the store's real schema
  version (7), and a Known gaps section carrying every open item below.
- `DEADLINES.md`: the v0.3 row marked DONE 2026-09-24, tagged `v0.3.0`, 15 days early.
- `02_roadmap\roadmap.md`: item 7 marked DONE, same wording.
- `SESSION_LOG.md`: one long paragraph prepended (V3-6), with pass/fail evidence for
  every Done-when and VERIFY item.
- `02_roadmap\2026-09-23_v0.3_session_prompts.md`: V3-6 ticked.
- `%LOCALAPPDATA%\burnmon\burnmon.json`: the stale `{"view":"monitor"}` leftover reset
  to `{"view":"full"}` (not a repo file; a real fix on this laptop, not a code change).
- This brief (new).

## 3. What did NOT happen (and why)

Not fixed: the Now chart's cost-axis toggle bug (`scripts\uicheck.ps1 w8`, the right-hand
axis stays hidden after its series is shown). Confirmed real and reproducible against the
current build, confirmed pre-existing (V3-3c already isolated the identical failure
against a pre-U5 build), judged not "small" (a real Chart.js layout fix, not a one-line
change) and out of this session's own scope (Done-when/VERIFY/docs, not a UI bugfix
session); carried into `STATUS.md`'s Known gaps instead. Not wired in: GitHub Copilot
Business/Enterprise credit allotments, though now confirmed (1,900/3,900 per user/month,
pooled), since their org-pooled billing model does not fit `CopilotCreditsLeft`'s
single-machine design and no per-seat price could be confirmed from a plain page fetch
either; a real design question for whoever wants Business/Enterprise support, not a
should-have-been-obvious gap. Not resolved: plan assumption A8 (active time vs. an
external time log) stays unverified, `C:\ZND\10_holding\03_logs\time\` still does not
exist on this laptop. Not pushed: commit and tag are done, push is Wilco's own step,
command below. Not touched: `C:\dev\Work`.

## 4. Findings worth propagating

- **[RESULT]** All seven Done-when items pass against the real store; full evidence
  (row counts, DOM reads, grep results) is in `SESSION_LOG.md`'s V3-6 entry, not
  repeated here.
- **[RESULT]** All six VERIFY items resolved: three already stood (Anthropic/OpenAI
  prices, Copilot VS Code span attributes), two were newly confirmed today (GitHub
  Copilot CLI's `~/.copilot` default, Copilot Business/Enterprise credit numbers, the
  ChatGPT/Codex non-existence of a comparable credit table), one was found wrong and
  fixed (Hermes's macOS/Linux default path), one stays explicitly unverified (A8).
- **[BUG, FIXED]** Hermes's guessed macOS/Linux default data path (V3-5, itself marked
  VERIFY at the time) was wrong: real default is `~/.hermes`, not a platform app-data
  folder. Anyone relying on that V3-5 default on a real Mac or Linux machine before this
  fix would have found no Hermes data at all.
- **[BUG, NOT FIXED, flagged]** The Now chart's cost-axis toggle does not show its axis
  when the cost series is shown via the chart legend. Real, reproducible, pre-existing.
- **[STATE]** The real local store is at schema version 7 (v0.3's `client` column
  migration), confirmed via a fresh `TestMigrateRealV01Store` run this session; no event
  lost across the full v0.1-to-v0.3 migration chain.
- **[STATE]** Real export/merge and Copilot VS Code data both flow correctly through the
  current build against the real store: a `--owner ZND` export held zero Valona rows and
  zero paths, and the real `github`-vendor (Copilot VS Code) tokens appeared correctly in
  both the vendor strip and a merged report.

## 5. Hub-level decision (if any)

None. v0.3.0 shipping is the scheduled release per the 2026-09-23 v0.3 grill, not a new
scope or pricing decision; nothing here crosses the CIPHER wall or touches Valona.

## 6. What the next hub read should update

`C:\ZND\10_holding\01_projects\burnmon.md` and the hub `roadmap.md` can mark BurnMon v0.3
as shipped 2026-09-24, 15 days ahead of the 2026-10-09 target carried in `DEADLINES.md`.
Next milestone: the 2026-12-19 decision (continue to a paid team line, keep free, or
stop), informed by roughly nine weeks of Talon's own BurnMon exports through the
Groundwork Kit once Talon actually starts using v0.3 in place of claudecost.

## 7. Open flags for next session

- Push (`git push origin main --tags`) is Wilco's manual step; command below.
- The Now chart's cost-axis toggle bug (`w8`) is real, reproducible and unfixed; needs a
  dedicated Chart.js debugging session, not a doc-pass afterthought.
- GitHub Copilot Business/Enterprise support (credits confirmed, per-seat price and the
  pooled-billing design question both still open) is a real feature decision, not a bug.
- A8 (active time vs. an external time log) stays unverified until
  `C:\ZND\10_holding\03_logs\time\` exists with a comparable entry.
- The "Overview tab" leftover copy in `template.html`'s About section (flagged since
  v0.2, still unresolved) remains open, a judgment call for whoever next touches that
  file's prose.

## 8. Related files

`C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_spec.md` (sections 5 and 6),
`C:\ZND\projects\burnmon\SESSION_LOG.md` (V3-6 entry, top of file),
`C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_session_prompts.md` (checklist, V3-6
now ticked), `C:\ZND\projects\burnmon\STATUS.md` (Known gaps section).

## Commands for Wilco (not run this session)

```powershell
# runs in: PowerShell on the laptop, cwd C:\ZND\projects\burnmon
git push origin main --tags
```
