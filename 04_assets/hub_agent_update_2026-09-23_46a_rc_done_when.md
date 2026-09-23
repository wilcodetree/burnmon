# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 (46A, release candidate Done-when pass) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** report the v0.2 release-candidate Done-when and VERIFY pass: all six section 5
items pass with fresh evidence, all five section 6 VERIFY items resolved to a stated
assumption and its UI label (or explicit absence of one), zero event loss confirmed
against a genuine synthetic v0.1 schema, no fix needed.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry, 46A),
then `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (checklist).
**Supersedes:** nothing; the filename uses the real date (2026-09-23), not the session
prompt's literal `2026-11-1X` placeholder, matching every other brief already in this
project's `04_assets` and the house date-prefix rule. The roadmap's week numbers (39-46)
are build-order labels, not calendar weeks, per 45B's finding; unchanged this session.

## 1. Headline

BurnMon v0.2's release-candidate check ran clean: all six Done-when items from spec
section 5 pass, verified directly against the real store and the real exe with two Claude
Code, one Cowork and two Codex sessions genuinely running concurrently; all five VERIFY
items from section 6 now have a stated code assumption and a UI label, or an explicit
"not labeled anywhere yet" where nothing applies it; a v0.1 store built from a genuine
pre-migration schema (not the real store, which is already past that point) lost zero of
250 events on migration to head. No failure found, so nothing was fixed. Not committed,
not tagged; commands below.

## 2. What changed on disk

- **Not committed.** This session's changes sit unstaged on local `main`, matching the
  session prompt's own instruction to stop before committing.
- **Files touched** (full paths): `C:\ZND\projects\burnmon\SESSION_LOG.md` (46A paragraph
  prepended), `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (46A
  ticked), this brief (new).
- **No source code changed.** The Done-when and VERIFY pass found no failing item, so the
  session's own rule ("fix what fails, nothing else") had nothing to apply to.
- **Scratch verification programs** (`cmd\v01check`, `cmd\histcheck`, `cmd\vendorcheck`,
  `cmd\forecheck`, `cmd\stripcheck`) were written, run and deleted in the same session;
  none remain on disk.

## 3. What did NOT happen (and why)

Not committed or tagged: per the session prompt, stop before tagging, print the commands,
Wilco's own call. Not fixed: no Done-when or VERIFY item failed this pass. Not re-run: the
"Overview tab" leftover copy flagged by 45A and left open by 45B is unchanged, still open,
still not this session's scope (Done-when/VERIFY only, per the prompt). Not addressed: the
`04_assets\hub_agent_update_2026-09-23_45b_done_when_pass.md` file from 45B is still
untracked in git; left as found, not this session's commit to make.

## 4. Findings worth propagating

- **[RESULT]** All six Done-when items (spec section 5) pass, with fresh evidence this
  session (not carried from 45B): Now defaults to the `now` tab unreachably from
  `#overview`; the vendor strip (`vendorstrip.Build` run directly against the real store)
  returned honest totals for all five agents plus a total row; a re-prefill finding fired
  on this session's own first turn; the Now chart's drawing code is unchanged since 44B's
  own 5-minute live-watched check. History's `{period: week, vendor: codex}` query
  returned 159,870,436 tokens across 217 sessions and 1,842 turns in one call. Hermes (4
  events) and Copilot CLI (24 events) both resolve through the shared agent-label map to
  "Hermes" and "Copilot CLI". The forecast slot returned `locked: true` with the gate text
  verbatim and `scored_weeks: 0`.
- **[RESULT]** Zero event loss on a genuine v0.1 migration, tested properly this time: a
  synthetic store built from migration1's exact pre-migration DDL (no `schema_version`
  table, no `owner` column), 250 events across 10 sessions, migrated to schema version 5
  through the real `store.Open` path with all 250 events surviving. `TestMigrateRealV01Store`
  in the repo now only re-opens the real store, which is already at schema 5 on this
  laptop; it no longer exercises a genuine pre-migration file and was not relied on for
  this claim.
- **[RESULT]** VERIFY carried (spec section 6), assumption and UI label for each: Claude
  Code cache TTL is 60 minutes (subscription-seat default, checked 2026-09-22), applied in
  the re-prefill cause and shown in the spike drawer's cause text when it fires. Claude
  Code auto-compact threshold is documented (~967,000 tokens, 1M-token beta window) but
  applied by no rule and shown nowhere, since burnmon prices the 200K standard tier and
  detects compaction by its effect instead. Hermes context window is confirmed absent from
  the data (not merely unverified); the Now card gauge honestly shows "context window
  unknown" for every Hermes session. Copilot CLI's `data.db` assumption was confirmed
  false in 43B (real file `session-store.db`, usage arrives per call, not at close); the
  adapter and UI no longer assume "totals at session end" anywhere, treating it like
  Hermes. Copilot VS Code OTel export is confirmed yes (corrected 2026-09-23) but wired
  into no adapter and labeled nowhere in the UI, a v0.3 scope call still open for Wilco.
  Codex has no subagent nesting in the trail; its internal auto-review sub-runs are
  filtered out by a `thread_source` flag before a session reaches the Now page, by design.
- **[STATE]** One nuance this pass caught that 45B's report-level grep missed: the
  generated report HTML embeds real historical session titles verbatim as data, and
  several of Wilco's own actual Claude Code session titles (from the real claudecost
  project) mention "claudecost-app" and one quotes a colleague's Dutch permission message.
  This is ingested user data, not BurnMon's own copy, and was left untouched (verbatim
  source evidence, not a template string to rewrite). The house no-Dutch/no-em-dash/
  no-claudecost check applies to `template.html`'s own authored copy, which is clean
  except the seven already-flagged, unreachable em dashes in the dead `#overview` section.

## 5. Hub-level decision (if any)

None. This session found no failure and made no fix; nothing here changes scope, pricing,
positioning, or crosses the CIPHER wall.

## 6. What the next hub read should update

`C:\ZND\10_holding\01_projects\burnmon.md` can note the release-candidate Done-when pass
is clean and BurnMon v0.2 is ready to tag pending Wilco's own commit and the 46B session.
No DEADLINES.md change; the 2026-11-14 tag date still assumes real calendar weeks pass for
forecast scoring, which has not started advancing past week 39 yet (45B's finding,
unchanged).

## 7. Open flags for next session (46B)

- Commit and tag are Wilco's manual step; commands below.
- `04_assets\hub_agent_update_2026-09-23_45b_done_when_pass.md` is still untracked; fold
  it into whichever commit picks it up.
- The "Overview tab" leftover copy in About (flagged by 45A, left open by 45B) is still
  open, still not a Done-when failure, still a judgment call for whoever next touches
  `template.html` prose.
- Copilot VS Code OTel wiring is a real v0.3 scope decision (confirmed technically
  possible), not built.
- Forecast scoring has one real week (39) on the books; it advances only with real
  wall-clock time.

## 8. Related files

`C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` (sections 5 and 6),
`C:\ZND\projects\burnmon\SESSION_LOG.md` (46A entry, top of file),
`C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (checklist, 46A
now ticked), `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-23_45b_done_when_pass.md`
(prior Done-when pass, 45B).

## Commands for Wilco (not run this session)

```powershell
# runs in: PowerShell on the laptop, cwd C:\ZND\projects\burnmon
git add -A
git commit -m "docs: 46A release candidate Done-when and VERIFY pass, no fix needed"
git tag v0.2.0-rc.1
git push origin main --tags
```
