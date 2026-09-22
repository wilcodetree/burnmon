# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-22 (tick 41A) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** History (v0.2 P2) is code-complete and committed on main, not pushed.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry, tick 41A)
**Supersedes:** nothing

## 1. Headline
BurnMon's History tab (v0.2 spec item P2) is built and committed to `main` locally:
one page replacing the old Months/Weeks/Days tabs, with period/range/vendor/owner filters,
a totals block, one chart and one table, backed by a new bound Go function `bmHistory`. Not
pushed to the remote yet.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (repo root): `9b509e7` "feat: History page, one
  bmHistory binding, vendor-aware totals (P2)". 5 files changed, 722 insertions, 85 deletions.
- **New files:** `C:\ZND\projects\burnmon\internal\history\history.go`,
  `C:\ZND\projects\burnmon\internal\history\history_test.go`.
- **Files touched:** `C:\ZND\projects\burnmon\cmd\burnmon\main.go` (new `bmHistory` binding),
  `C:\ZND\projects\burnmon\internal\report\template.html` (History UI built out, old
  `#months`/`#weeks`/`#days` sections and their `periodRows`/`renderPeriods` functions deleted),
  `C:\ZND\projects\burnmon\SESSION_LOG.md` (tick 41A entry prepended).
- **On a branch, not merged:** nothing; this landed directly on `main`.
- Not pushed: `main` is one commit ahead of `origin/main` as of this session.

## 3. What did NOT happen (and why)
Not pushed to the remote (left for Wilco, per house rule that pushes are his manual step).
No per-vendor cost book work happened (that is v0.3, unchanged). The vendor strip on Now (P3)
was not touched, it is scheduled for week 41 session B. Insight, forecast and adapter items
(I1 to A3) were not touched, out of this session's scope. No live re-query of a real BurnMon
store was run; verification was `go test ./... -count=1` (all packages pass, including the new
`internal/history` tests against a real temporary SQLite store fixture), `node --check` on the
extracted inline dashboard script, and `.\build.ps1` (both `burnmon-cli.exe` and `burnmon.exe`
build clean). No manual click-through of the built app window happened this session.

## 4. Findings worth propagating
- [RESULT] `internal\history\history_test.go` passes: a store fixture with two vendors
  (`claude-code`/anthropic, `codex`/openai) across three ISO weeks in September 2026 produces
  correct per-week and per-month bucket totals (sessions, turns, tokens) and a cost figure that
  matches `pricing.Defaults()`'s sonnet rate by hand, when the vendor filter is narrowed to one
  covered vendor.
- [STATE] Open design call, put to Wilco directly rather than guessed: when the History vendor
  filter is "All" and the filtered data mixes covered vendors (Claude, Codex) with uncovered
  ones (Hermes, Copilot CLI), what should the totals block show? Decided: cost only ever appears
  when the filter is narrowed to one covered vendor (`anthropic` or `openai`); "All" always shows
  the `cost_note` "tokens only until v0.3" text, even if the filtered data happens to be
  all-covered. Implemented in `coveredVendorForAgent` in
  `C:\ZND\projects\burnmon\internal\history\history.go`.
- [STATE] Build order per the spec (`C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md`,
  section 4) has P2 History in week 41 session A, which this session completes; P3 vendor strip
  and A1 Hermes fixture are week 41 session B, still open.

## 5. Hub-level decision (if any)
Nothing. The mixed-vendor cost call above is a BurnMon-internal implementation decision, not a
cross-project, positioning, pricing, park/unpark or CIPHER-wall call, so it stays in this
project's own files rather than `C:\ZND\10_holding\03_logs\decisions.md`.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` (hub one-pager): note P2 History as done, tick 41A.
- `C:\ZND\10_holding\02_roadmap\roadmap.md`: if it tracks BurnMon's v0.2 build order line by
  line, mark week 41 session A complete.
- No change expected to `DEADLINES.md` (v0.2's 2026-11-14 tag date is unaffected) or
  `portfolio.md`'s BurnMon description (still accurate as written).

## 7. Open flags for next session
- Push `main` to the remote (Wilco's manual step, command already given to him in chat).
- Week 41 session B: P3 vendor strip on Now, and A1 Hermes fixture recording, both still open.
- No manual UI click-through of the built `burnmon.exe` happened this session; worth a quick
  look at the History tab in the real app before trusting the filter UX end to end.

## 8. Related files
- Spec: `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` (section 2.2, P2).
- Session log: `C:\ZND\projects\burnmon\SESSION_LOG.md` (tick 41A entry, top of file).
- Prior briefs this week: `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.2_39B_migrations_owner.md`,
  `..._40A_tool_calls.md`, `..._40B_five_tabs.md`.
