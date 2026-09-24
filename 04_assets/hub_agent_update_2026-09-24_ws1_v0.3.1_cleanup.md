# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-24 - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** WS1 cleanup session (v0.3.1, patch): dev-only mode, Sessions harness fix, History stacked chart, ready to commit, not yet committed.
**Read order:** this file, `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry), `C:\ZND\projects\burnmon\STATUS.md`.
**Supersedes:** nothing (WS1-specific, sits alongside `hub_agent_update_2026-09-24_v0.3_V3-6_ship.md`, which shipped v0.3.0 the same day).

## 1. Headline

BurnMon v0.3.1, a cleanup pass over v0.3.0: dev is now the only mode (Dev/Business toggle
and Monitor mode deleted, code and all), the Sessions tab no longer mislabels every
Codex/Copilot CLI session as Claude Code and no longer mis-prices Copilot/Hermes calls as
Claude, History's chart now stacks one bar segment per vendor. Code-complete and verified
against the real local store; not committed, not pushed, not tagged.

## 2. What changed on disk

- **Committed:** none. Every change below is uncommitted in the working tree.
- **On a branch, not merged:** not applicable, this ran on `main` directly (per the WS1
  plan, `main` is where this workstream lands; WS2 is the one on a branch/worktree).
- **Files touched** (full paths, all under `C:\ZND\projects\burnmon\`):
  - `internal\report\template.html` (large: deleted Monitor markup/CSS/JS, the
    dev/business toggle and every `isBusiness()` branch, added the Sessions Harness
    column/filter and the History stacked-bar chart)
  - `internal\pricing\pricing.go` (removed `Config.Mode`/`View`, `BusinessMode()`,
    `MonitorView()`)
  - `internal\pricing\cost.go` (new `Config.EventCost`, refactored the existing batch
    cost functions to share per-event helpers with it)
  - `internal\pricing\pricing_test.go`
  - `internal\dataset\dataset.go` (removed `Payload.Mode`/`View`, added `AgentLabels`,
    `dedupSessions`' key gained Vendor)
  - `internal\dataset\dataset_test.go`, `internal\dataset\fromstore.go` (Session now
    carries Vendor/Agent, cost routed through `EventCost`), `internal\dataset\fromstore_test.go`
  - `internal\scan\types.go` (new `Vendor`/`Agent` fields on `scan.Session`, new
    `scan.AgentLabel` map)
  - `internal\history\history.go` (new `Totals.ByVendor`), `internal\history\history_test.go`
  - `cmd\burnmon\app.go` (removed `settingsPayload.DefaultView` and the Settings dialog's
    "Default view" select; version bumped `0.3.0` -> `0.3.1`)
  - `cmd\burnmon\main.go` (removed the `ccSaveView` binding)
  - `cmd\burnmon-cli\main.go` (version bumped `0.3.0` -> `0.3.1`)
  - `burnmon.example.json` (removed the `mode`/`view` example keys)
  - `scripts\uicheck.ps1` (`selfManagedChecks` trimmed to `w1`)
  - `README.md`, `STATUS.md` (rewritten sections: adapters, pages, cost, forecast, config,
    known gaps; a stale STATUS.md line about Copilot CLI corrected against the adapter's
    own doc comment)
  - `SESSION_LOG.md` (new top entry)
- **Deleted:** `tools\uicheck\check_v3.go`, `tools\uicheck\check_v3b.go`,
  `tools\uicheck\check_u5.go` (every case in all three tested only the removed
  mode/monitor features).
- **Untracked, not part of this change:** `02_roadmap\2026-09-24_ws1_burnmon_cleanup.md`
  and `02_roadmap\2026-09-24_ws2_burnmon_dev.md` (the two workstream plans themselves,
  already present at session start).

## 3. What did NOT happen (and why)

- **Not committed, not pushed, not tagged.** Per the WS1 plan: hand Wilco the exact
  PowerShell commands, he runs them. Commands are in the chat transcript, not repeated
  here.
- **No live click-through of History's per-client table.** The dev/business toggle that
  used to gate it is gone; the table's gate changed to "client rules exist" (a judgment
  call, see section 7), verified by reading the JS and the still-passing Go test
  (`TestBuildClientFilterAndRows`), not by dropping a scratch `owners` config next to the
  exe and clicking through it live (V3-6's own Done-when check 2 did that; this session
  did not repeat it, to avoid mutating Wilco's real `burnmon.json`/store state twice in
  one day).
- **`w1` (uicheck, WebView2 startup-paint timing) not fixed.** Failed twice this session
  (see section 4); the WS1 plan pre-authorizes this exact check as a known pre-existing
  flake to report, not fix.
- **No macOS/Linux artefact work, no OTLP receiver, no Copilot Business/Enterprise credit
  book, no ChatGPT/Codex credit table.** All out of this session's scope, unchanged from
  v0.3.0's own open items.

## 4. Findings worth propagating

- **[RESULT]** Full Go test suite green: `go test ./... -count=1` across every package
  (`cmd\burnmon`, `cmd\burnmon-cli`, every `internal\*` package with tests), including 6
  new/changed tests (`TestDedupSessionsKeepsSameSessionIDAcrossVendors`,
  `TestBuildSessionCarriesVendorAndAgent`,
  `TestSessionsFromEventsPricesGitHubEventsThroughCopilotBook`,
  `TestSessionsFromEventsNoPriceForUnbookedVendor`,
  `TestBuildByVendorSumsToRowTotals`, `TestLoadModeAndCopilotPlan` rewritten).
- **[RESULT]** `go vet ./...` clean, `.\build.ps1` succeeded (`burnmon.exe`,
  `burnmon-cli.exe` both built), `node --check` clean on both of the template's inline
  `<script>` blocks.
- **[RESULT]** Live-window verification against Wilco's real store (1096 real sessions,
  6 real vendors with data) via a one-off `scripts\uicheck.ps1 scratch` check (written,
  run, then deleted, not part of the permanent suite): the Sessions tab's new Harness
  filter lists all six real vendors (Claude Code, Codex, Copilot CLI, Copilot (VS Code),
  Cowork, Hermes) with correct labels; a real Cowork session now reads "Cowork" with
  "Surface: Claude Desktop" in its tooltip, was previously indistinguishable from Claude
  Code; the History chart's real x-axis labels read
  `2026-08-24,2026-08-31,2026-09-07,2026-09-14,2026-09-21` (all `yyyy-mm-dd`, every
  period) with six real, differing stacked-vendor series.
- **[RESULT]** `scripts\uicheck.ps1` `w0`, `w2`-`w8` all passed against the real store,
  including `w8` (the Now chart's cost-axis toggle), previously a documented "known gap"
  in `STATUS.md`. Flagged as an open discrepancy in `STATUS.md` rather than marked fixed:
  no code in this session touched that logic, so it may be a timing flake rather than a
  real fix; a future session should re-run `w8` a few times before closing or reopening
  that gap.
- **[STATE]** `w1` (WebView2 first-paint-within-1.5s check) failed twice this session:
  once catching an unrelated File Explorer window's z-order in its screenshot (a stray
  already-running `burnmon.exe`, the user's own long-lived monitor instance from 08:49,
  was blocking clean launches; closed with Wilco's explicit go-ahead mid-session, asked
  first via AskUserQuestion), once with "no window found within 1.5s" after that was
  cleared, on a laptop running multiple concurrent Claude Code sessions plus WS2's own
  build in its worktree at the time. Pre-authorized in the WS1 plan as a known,
  pre-existing, load-sensitive flake; reported, not fixed, no regression evidence.
- **[RESULT]** A fresh Opus read-only review agent ran over the full diff before handoff
  and found three real issues, all fixed and re-verified (`go vet`/`go test`/`node
  --check` all clean again afterward): (1) `internal\live\live.go`'s `turnCost` and
  `ApplySessionTotals` had the exact same non-OpenAI-vendor-priced-as-Claude bug
  `buildSession` had just been fixed for (the WS1 plan's own diagnosis was scoped to
  `fromstore.go` and missed this second instance); both now route through the new
  `cfg.EventCost`. (2) The Sessions table's model-pill colour rule
  (`m==='Fable'`/`m==='Opus'`) stopped matching once Anthropic's own per-call label
  became the book's exact-model name ("Claude Opus 5.5") instead of the family name
  ("Opus"), a real regression from this session's own pricing fix; changed to a substring
  check. (3) The pricing fix's real scope reaches further than "Copilot/Hermes no longer
  priced as Claude": Anthropic's own events on Sessions and Now are now also priced from
  `AnthropicBook`'s exact model id instead of the family-generic table, a deliberate,
  correct consequence this session's earlier documentation under-stated; now documented
  properly in `STATUS.md` and the `fromstore.go` comment (see section 7). Two low-severity
  findings (a leftover dead-code trail from the isBusiness() removal;
  `copilot_credits_left`/`BusinessCost` computed but no longer read by anything) are noted
  in `STATUS.md`'s Known gaps rather than fixed further this pass.
- **[STATE]** The post-review fixes were not re-verified with a fresh live-window
  screenshot: a second `burnmon.exe`, WS2's own build in its worktree
  (`.claude\worktrees\burnmon-dev\burnmon.exe`), was running and actively under test at
  that point, holding the single-instance lock; unlike the stray instance in section 3,
  this one belongs to concurrent, legitimate work and was left alone. Confidence instead
  from the full Go test suite (including `internal\live`'s own `BusinessCost`/cost tests,
  still green) and the pre-fix live pass covering the equivalent code paths.

## 5. Hub-level decision (if any)

Nothing. This is a project-internal cleanup and bugfix pass, no cross-project time,
park/unpark, consultancy, positioning, pricing, voice or CIPHER-anonymity call in it.

## 6. What the next hub read should update

- `C:\ZND\10_holding\01_projects\burnmon.md` (the hub one-pager): note v0.3.1 shipped
  (once Wilco commits/tags), dev-only mode, the Sessions/History fixes.
- `C:\ZND\10_holding\02_roadmap\roadmap.md` / `portfolio.md`: no state change expected
  (BurnMon's own next decision is still 2026-12-19, per `STATUS.md`'s "Next" section),
  but worth a pass to confirm nothing there still references the now-removed
  Dev/Business toggle or Monitor mode as a feature.

## 7. Open flags for next session

- **Judgment call to review:** History's per-client table, previously gated on business
  mode AND client rules, now shows whenever client rules exist (business mode itself is
  gone). This was treated as "not actually a business-only branch, just gated that way"
  rather than a feature to delete outright, since it answers the v0.3 spec's own product
  sentence ("what did this client cost"). Worth a quick nod from Wilco that this reading
  was right.
- **`w8`'s pass this session** (previously a documented known gap) needs a couple more
  runs before `STATUS.md`'s "Known gaps" entry is either closed or reopened with new
  evidence.
- **Commit/push/tag** for v0.3.1 is Wilco's own manual step; the exact PowerShell
  commands are in the session's chat transcript.
- **`OpenAIBook`** (the newer, C1-era price book) is missing two of the four OpenAI model
  ids the older `OpenAIPrices` map already covers (`gpt-5.6-terra`, `gpt-5.6-luna`); this
  session deliberately left Sessions' and Now's OpenAI pricing path on the older, complete
  map rather than switch to the incomplete newer book. Worth a follow-up session to
  complete `OpenAIBook` and then unify the two.
- **Anthropic pricing granularity changed on Sessions and Now**, not just Copilot/Hermes:
  both now price a Claude call from `AnthropicBook`'s exact model id instead of the old
  family-generic table (Sonnet's family price was $3/$15, the book's `claude-sonnet-5` is
  $2/$10); every model id live on Wilco's laptop is in the book, so the real-number risk
  is low, but a model id the book has not caught up with would now show "no price" rather
  than a guessed Sonnet figure. See `STATUS.md`'s Cost section.
- **Two backend figures now computed but unread**: the vendor strip's
  `copilot_credits_left` and `live.Session.BusinessCost` (their only display line/reader
  were both deleted with the toggle). Harmless, flagged in `STATUS.md`'s Known gaps for a
  future removal pass rather than fixed here.

## 8. Related files

- `C:\ZND\projects\burnmon\02_roadmap\2026-09-24_ws1_burnmon_cleanup.md` (this session's
  own plan).
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-24_ws2_burnmon_dev.md` (the parallel
  workstream, BurnMon Dev, untouched by this session).
- `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-24_v0.3_V3-6_ship.md` (the
  same-day v0.3.0 release brief this one follows).
- `C:\ZND\projects\burnmon\SESSION_LOG.md` (top entry, full session narrative).
