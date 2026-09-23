# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 (v0.3 session V3-3) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** V3-3 (dev/business switch C3, client view K3, Now page fixes and loading states U1/U2) is code-complete on main, real-window checked, not yet committed or tagged; hand-off for whoever commits next.
**Read order:** this file, C:\ZND\projects\burnmon\SESSION_LOG.md (top entry)

## 1. Headline
C3, K3, U1 and U2 are implemented, fully tested (`go test ./... -count=1`, `.\build.ps1`, `node --check` on both inline `<script>` blocks, all green), and checked against the real running window with a new `tools\uicheck\check_v3.go` ("v3") check: the dev/business toggle flips live, real `SendInput` clicks (not eval `.click()`), before/after screenshots kept. One open design gap the spec did not resolve (how "credits left" should be computed with no account-wide credits tracking and no field naming the account's Copilot plan tier) was put to Wilco before writing any code; his answer (a new `copilot_plan` config field, mirroring `subscription.your_seat`) is what shipped.

## 2. What changed on disk
- Committed: nothing yet. Every change below is an uncommitted working-tree edit in C:\ZND\projects\burnmon (branch main). `04_assets\_grill_state.md` and `DEADLINES.md` also show modified in git status but were already dirty before this session started (unrelated prior work); not touched here, not part of this brief's changes.
- Files touched (full paths):
  - C:\ZND\projects\burnmon\internal\pricing\pricing.go (Config.Mode, Config.BusinessMode(), Config.CopilotPlan)
  - C:\ZND\projects\burnmon\internal\pricing\pricing_test.go (mode/copilot_plan load tests, dev-is-default test)
  - C:\ZND\projects\burnmon\internal\pricing\cost.go (CopilotCreditsLeft; BasisCost/VendorCost gain JSON tags, a V3-1 gap that never mattered until this session put BasisCost on the wire)
  - C:\ZND\projects\burnmon\internal\pricing\cost_test.go (CopilotCreditsLeft tests: unconfigured, unknown plan, configured)
  - C:\ZND\projects\burnmon\internal\live\live.go (Session.Client, Session.BusinessCost; BuildSnapshot and ApplySessionTotals both populate it, the latter from synthetic per-model events built off the SQL-aggregated lifetime totals)
  - C:\ZND\projects\burnmon\internal\live\live_test.go (Client/BusinessCost tests, no-book-vendor case)
  - C:\ZND\projects\burnmon\internal\vendorstrip\vendorstrip.go (Build gains a cfg param; CopilotCreditsLeft field, computed from a second EventsSince(monthStart) query, gated on Config.CopilotPlan being set)
  - C:\ZND\projects\burnmon\internal\vendorstrip\vendorstrip_test.go (updated call sites, new CopilotCreditsLeft test)
  - C:\ZND\projects\burnmon\internal\history\history.go (headlineCostUSD replaces the old anthropic/openai-only coveredVendorForAgent gate; Filter.Client, Payload.Clients/ClientRows, buildClientRow)
  - C:\ZND\projects\burnmon\internal\history\history_test.go (rewrote cost-figure expectations to the new headline basis, new client-filter/rows and no-book-vendor tests)
  - C:\ZND\projects\burnmon\internal\forecast\forecast.go (Build gains a cfg param; addCostForecast, EndOfDayCostUSD/EndOfMonthCostUSD/CostCovered, deliberately USD not EUR since the template's own money() converts)
  - C:\ZND\projects\burnmon\internal\forecast\forecast_test.go (updated call sites, cost-forecast and uncovered-vendor tests)
  - C:\ZND\projects\burnmon\internal\dataset\dataset.go (Payload.Mode, straight from cfg.Mode)
  - C:\ZND\projects\burnmon\cmd\burnmon\main.go (bmVendorStrip and bmForecast bindings pass cfg through)
  - C:\ZND\projects\burnmon\internal\report\template.html (header mode toggle; business-mode session cards, live chart, forecast text, vendor strip credits line; History client filter and per-client table; Sessions client column and filter; U1 intro-to-About move and no-sessions-notice relocation; U2 waitForBinding retry helper, loading markup/overlays, tokPrecise(0) fix)
  - C:\ZND\projects\burnmon\burnmon.example.json (mode and copilot_plan example block)
  - C:\ZND\projects\burnmon\tools\uicheck\check_v3.go (new; real-window check for the mode toggle and the U2 startup fix)
  - C:\ZND\projects\burnmon\SESSION_LOG.md (V3-3 paragraph)
  - C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_session_prompts.md (V3-3 ticked)
- Untracked / outside a repo: `testdata\uicheck\out\v3-*.png` (gitignored, evidence only, not for commit).

## 3. What did NOT happen (and why)
- Not committed, not tagged, not pushed. House rule: git writes on C:\ZND are a manual step for Wilco. Exact commands below.
- The client filter/column, the per-client History table, and the Copilot credits-left line have not been seen populated in the real window: this laptop's live `burnmon.json` carries no `owners` rules and no `copilot_plan`, so every one of those stays in its "hidden while no rule exists" state on the real store. All three are covered by fixture tests instead (`internal\history\history_test.go`, `internal\vendorstrip\vendorstrip_test.go`, `internal\pricing\cost_test.go`). Worth a real-window spot-check once a client rule and a Copilot plan exist.
- V3-3b (monitor mode, U3/U4) is explicitly the next session, not part of this one.
- Two of the three uicheck screenshots needed a settle-time fix mid-session: the first attempt at 300ms after a click captured a stale frame (DWM had not repainted yet) even though the DOM eval read back the correct post-click state; bumped to 1000ms and re-ran clean. Noted in case a future check hits the same thing.

## 4. Findings worth propagating
- [RESULT] Full test suite green: `go test ./... -count=1` (every package), `go vet ./...`, `.\build.ps1`, `node --check` on both inline `<script>` blocks in `template.html`.
- [RESULT] Real-window check (`scripts\uicheck.ps1 v3`) passed against the actual running exe with a live Claude Code session: startup never showed the saved-report text (the original U2 bug, `pollVendorStrip`/`pollForecast`/`pollNow` all checked their binding exactly once, immediately, racing go-webview2's own async injection), and a real `SendInput` click flipped the page to business mode (chart axis, cost line, card content, all confirmed in both a DOM read and a screenshot) and back.
- [RESULT] History's cost figure used to only appear when the vendor filter was narrowed to one of two hardcoded vendors (anthropic, openai); it now sums V3-1's headline basis across every vendor present, including github (Copilot). This is a real behaviour change, not a relabel: `TestBuildWeekAndMonth`'s and `TestBuildOneScoredWeekShowsBand`'s own expected numbers had to be recalculated by hand against the dated books, not just their pass/fail status.
- [STATE] `pricing.BasisCost`/`VendorCost` had no JSON tags at all since V3-1; harmless while nothing serialised them, but this session's `live.Session.BusinessCost` field would have shipped a `Basis`/`Label`/`USD` capitalised-field payload to the template had this not been caught before building.
- [OPEN] Decision taken this session, recorded here since it changes what `burnmon.json` can say: `copilot_plan` (a key into the compiled-in `copilot_credits.json` book's plan tiers) is new, undocumented anywhere except `burnmon.example.json`'s own comment and this brief. Worth a mention if Talon's Groundwork Kit note (V3-6) covers Copilot at all.

## 5. Hub-level decision (if any)
Nothing. Project-internal implementation detail, no cross-project time, positioning or CIPHER call involved.

## 6. What the next hub read should update
Nothing in the hub canonical files (STATUS, DEADLINES, portfolio, decisions.md) needs to change from this brief alone. The v0.3 session prompts checklist is already ticked for V3-3.

## 7. Open flags for next session
- Commit is still Wilco's to run (no tag this session, per the build order). Exact commands, from C:\ZND\projects\burnmon:

```
git add internal/pricing/pricing.go internal/pricing/pricing_test.go internal/pricing/cost.go internal/pricing/cost_test.go internal/live/live.go internal/live/live_test.go internal/vendorstrip/vendorstrip.go internal/vendorstrip/vendorstrip_test.go internal/history/history.go internal/history/history_test.go internal/forecast/forecast.go internal/forecast/forecast_test.go internal/dataset/dataset.go cmd/burnmon/main.go internal/report/template.html burnmon.example.json tools/uicheck/check_v3.go SESSION_LOG.md "02_roadmap/2026-09-23_v0.3_session_prompts.md" "04_assets/hub_agent_update_2026-09-23_v0.3_V3-3.md"
git commit -m "v0.3 V3-3: dev/business switch (C3), client view (K3), Now page fixes (U1, U2)"
```

  (No tag this session, per the build order table; git push is a separate, deliberate step, not included above.)
- V3-3b (U3 monitor mode, U4 dev-mode text rendering) is next per the build order.
- Real-window spot-check of the client filter/column, per-client table and credits-left line, once a real `owners` rule and `copilot_plan` exist in the live `burnmon.json` (see section 3).

## 8. Related files
- Spec: C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_spec.md (sections 2.1 C3, 2.2 K3, 2.5 U1/U2)
- Features note: C:\ZND\projects\burnmon\04_assets\2026-09-22_burnmon_now_page_features.md (section 4)
- Prior brief: C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-23_v0.3_V3-2.md
- Session log: C:\ZND\projects\burnmon\SESSION_LOG.md
