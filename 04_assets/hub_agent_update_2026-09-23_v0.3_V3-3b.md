# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 (v0.3 session V3-3b) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** V3-3b (monitor mode U3, dev-mode text rendering U4) is code-complete on main, tested and partially real-window checked, not yet committed or tagged; hand-off for whoever commits next, and for whoever can re-run the interactive part from an unlocked session.
**Read order:** this file, C:\ZND\projects\burnmon\SESSION_LOG.md (top entry)

## 1. Headline
U3 and U4 are implemented: a header Monitor/Full switch, persisted in `burnmon.json`'s new `"view"` field through the same merge-into-JSON save path Settings uses (`writeConfigKey`, generalised from the old `writeSubscriptionConfig`), a Settings-dialog default-view field, and a chrome-less monitor page reusing the Now page's own chart/cards/vendor strip in business mode, swapped for a real block-character/ASCII-border text page in dev mode. Fully tested (`go test ./... -count=1`, `go vet ./...`, `.\build.ps1`, `node --check` on both inline `<script>` blocks, all green). Real-window checked only partially: a new self-managed `tools\uicheck\check_v3b.go` proved the config-driven startup state via the dev eval channel (no click needed), but every click-driven step in it, and in the pre-existing `v3` check, failed because this laptop's console session was locked for the whole run (`query session` showed the foreground window as the Windows lock screen) — confirmed an environment condition, not a regression, since the untouched `v3` check failed the identical way. See section 3.

## 2. What changed on disk
- Committed: nothing yet. Every change below is an uncommitted working-tree edit in C:\ZND\projects\burnmon (branch main).
- Files touched (full paths):
  - C:\ZND\projects\burnmon\internal\pricing\pricing.go (Config.View, Config.MonitorView())
  - C:\ZND\projects\burnmon\internal\pricing\pricing_test.go (TestLoadView, TestDefaultsAreFullView)
  - C:\ZND\projects\burnmon\internal\dataset\dataset.go (Payload.View, straight from cfg.View)
  - C:\ZND\projects\burnmon\cmd\burnmon\main.go (writeSubscriptionConfig generalised to writeConfigKey(path, key, value); new ccSaveView binding; settingsPayload.DefaultView; applySettings also writes "view"; settingsModalHTML gains a Default view select)
  - C:\ZND\projects\burnmon\internal\report\template.html (header #btn_monitor button; #monitor_exit_btn; #monitor_text_view and its #mt_chart/#mt_sessions/#mt_vendorstrip children; #now_extra wrapper around the turn ticker and forecast; monitor-mode/monitor-dev CSS; UI.view state; setView/applyViewMode; renderMonitorChartText/monitorSessionBoxHTML/mtVendorStripText/renderMonitorText; renderNow and renderVendorStrip each call renderMonitorText when body carries monitor-dev; applyMode calls applyViewMode)
  - C:\ZND\projects\burnmon\burnmon.example.json ("view" example block)
  - C:\ZND\projects\burnmon\scripts\uicheck.ps1 (self-managed check list generalised from a hardcoded "w1" case to a `$selfManagedChecks` array, now `@("w1", "v3b")`)
  - C:\ZND\projects\burnmon\tools\uicheck\check_v3b.go (new; self-managed real-window check for U3/U4)
  - C:\ZND\projects\burnmon\SESSION_LOG.md (V3-3b paragraph)
  - C:\ZND\projects\burnmon\STATUS.md (Known gaps: the locked-session re-verification item)
  - C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_session_prompts.md (V3-3b ticked)
- Untracked / outside a repo: `testdata\uicheck\out\v3b-*.png` (gitignored, evidence only; the click-driven ones show the Windows lock screen, not burnmon, per section 3 below — do not mistake them for real coverage).

## 3. What did NOT happen (and why)
- Not committed, not tagged, not pushed. House rule: git writes on C:\ZND are a manual step for Wilco. Exact commands below.
- The click-driven half of `scripts\uicheck.ps1 v3b` (Full view / Monitor round trip, business-mode monitor visuals, the restart-keeps-the-choice proof) did not complete: this laptop's console session was locked (`query session` → `console ... Active` but the foreground window was `Windows Default Lock Screen`) for the whole run, so `BitBlt` screenshots captured the lock screen's own background image, and `SendInput` clicks landed on it too, never reaching burnmon's window — confirmed independent of `check_v3b.go`'s own code (which re-asserts the window's topmost z-order via `ensureWindowSize` before every click and screenshot, the same trick `main.go`'s dispatch already applies for every other check) by re-running the untouched, previously-passing `v3` check: it failed identically (`btn_mode label after click = "Dev", want "Business"`). The eval-only assertions in `check_v3b.go` (startup state: `#topbar`/`#now_extra` hidden, `#monitor_text_view` shown, `monitor-dev` on `body`, all read via the dev TCP eval channel, which does not depend on screen compositing) passed cleanly and are real evidence the markup/CSS/JS wiring is correct.
- Needs Wilco to run `.\scripts\uicheck.ps1 v3b` (and, to confirm the environment theory, `.\scripts\uicheck.ps1 v3`) once from an actually unlocked interactive session, and check the resulting screenshots show real burnmon content rather than a lock screen.

## 4. Findings worth propagating
- [RESULT] Full test suite green: `go test ./... -count=1` (every package), `go vet ./...`, `.\build.ps1`, `node --check` on both inline `<script>` blocks in `template.html`.
- [RESULT] Eval-only real-window state (no click needed) confirmed correct: opening burnmon with `"view": "monitor"` in an exe-adjacent `burnmon.json` lands directly on the chrome-less, real-text monitor page, no header, no tab bar, no Chart.js canvas.
- [FINDING] `scripts\uicheck.ps1` only special-cased "w1" as self-managed; generalised to a list so a second self-managed check (v3b, which needs to control `burnmon.json` before its own launch and restart the exe mid-check) doesn't collide with the script's own shared-instance Start-Process flow. Any future self-managed check just needs adding to that one array.
- [FINDING] A self-managed uicheck's clicks/screenshots need their own `ensureWindowSize` call immediately before each one, not just once after launch: `main.go`'s dispatch does this automatically for every non-self-managed check right before it runs, but a self-managed check (like v3b, which does a 90-second Collect wait plus several more sleeps and clicks before its first interactive step) gets no such help and needs to re-assert the window's topmost z-order itself each time, since a long-running check gives another window plenty of time to reclaim the top of the z-order in between.
- [ENVIRONMENT] This session's automation environment had a locked console session throughout (confirmed via `query session` and reading the foreground window's title/class: "Windows Default Lock Screen" / `Windows.UI.Core.CoreWindow`). `SendInput`/`BitBlt`-based uicheck steps cannot do anything meaningful under a locked session; the dev TCP eval channel (`cmd\burnmon\uicheck_devserver.go`) still works fine, since it never touches the screen. Worth keeping in mind for any future session run the same way: check `query session` first if a click-driven uicheck step behaves strangely.

## 5. Hub-level decision (if any)
Nothing. Project-internal implementation detail, no cross-project time, positioning or CIPHER call involved.

## 6. What the next hub read should update
Nothing in the hub canonical files (STATUS, DEADLINES, portfolio, decisions.md) needs to change from this brief alone. The v0.3 session prompts checklist is already ticked for V3-3b.

## 7. Open flags for next session
- Commit is still Wilco's to run (no tag this session, per the build order). Exact commands, from C:\ZND\projects\burnmon:

```
git add internal/pricing/pricing.go internal/pricing/pricing_test.go internal/dataset/dataset.go cmd/burnmon/main.go internal/report/template.html burnmon.example.json scripts/uicheck.ps1 tools/uicheck/check_v3b.go SESSION_LOG.md STATUS.md "02_roadmap/2026-09-23_v0.3_session_prompts.md" "04_assets/hub_agent_update_2026-09-23_v0.3_V3-3b.md"
git commit -m "v0.3 V3-3b: monitor mode (U3), dev-mode text rendering (U4)"
```

  (No tag this session, per the build order table; git push is a separate, deliberate step, not included above.)
- Re-run `.\scripts\uicheck.ps1 v3b` (and `v3`, as a control) from an unlocked interactive session and confirm the screenshots show real content; update `STATUS.md`'s Known gaps entry once done.
- V3-4 (export K4, merge K5) is next per the build order.

## 8. Related files
- Spec: C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_spec.md (section 2.5 U3/U4)
- Colour table referenced: C:\ZND\projects\burnmon\internal\report\template.html (`VENDOR_COLOR_FAMILIES`, v0.2.2)
- Prior brief: C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-23_v0.3_V3-3.md
- Session log: C:\ZND\projects\burnmon\SESSION_LOG.md
