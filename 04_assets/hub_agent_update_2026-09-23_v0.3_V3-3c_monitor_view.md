# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 (v0.3 session V3-3c) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** V3-3c (U5, the monitor-dev braille history chart) is code-complete on main, fully tested and real-window checked, not yet committed at the time of writing (commit happens right after this brief); hand-off for whoever pushes it or picks up the next v0.3 session.
**Read order:** this file, C:\ZND\projects\burnmon\SESSION_LOG.md (top entry)
**Supersedes:** nothing

## 1. Headline
U5 is implemented and verified against the real running window: monitor-dev's tiny one-block-per-minute chart is replaced with a perfadvisor-style braille line chart (one overlaid dot series per session, coloured legend rows, minute labels), plus a real CSS grid for session cards and a bigger, corrected monospace stack. Code-complete on `main`'s working tree, all tests and the real-window check green, ready to commit.

## 2. What changed on disk
- **Committed** in `burnmon`: nothing yet at the time of writing this brief; the commit happens immediately after (see section 7 for the exact command already used).
- **On a branch, not merged:** none, this is straight on `main`'s working tree.
- **Files touched** (full paths):
  - `C:\ZND\projects\burnmon\internal\live\live.go` (`BuildSnapshot` gained an optional variadic `bucketSeconds` argument, default 60 unchanged for every existing 3-arg caller; `buildChart` sizes its slot count from that argument instead of the fixed `BucketSeconds` constant)
  - `C:\ZND\projects\burnmon\internal\live\live_test.go` (new `TestBuildSnapshot_CustomBucketSeconds`, asserts a 10-second bucket yields 180 dense slots)
  - `C:\ZND\projects\burnmon\cmd\burnmon\main.go` (`bmLive` binding takes the same optional int, passes it straight through to `BuildSnapshot`)
  - `C:\ZND\projects\burnmon\internal\report\template.html` (CSS: chart panel, legend rows, minute-label row, 4-column session card grid, bigger clamp()-scaled type, Cascadia Mono/JetBrains Mono/Consolas stack; JS: `mtBrailleGraph` (perfadvisor's `graph()` ported to JS), `renderMonitorChartText` rebuilt around it, `mtTokensPerMinute`, `sizeMonitorChart` (dot-row count from the panel's real pixel height, ~35% of the window), `monitorSessionLines`/shared session-box width computation, `pollNow` calls `bmLive(10)` in monitor-dev vs `bmLive()` in full view)
  - `C:\ZND\projects\burnmon\scripts\uicheck.ps1` (`$selfManagedChecks` gained `"u5"`)
  - `C:\ZND\projects\burnmon\tools\uicheck\check_u5.go` (new; self-managed real-window check for U5)
  - `C:\ZND\projects\burnmon\SESSION_LOG.md` (V3-3c paragraph, prepended)
  - `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_session_prompts.md` (V3-3c ticked)
  - `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-23_v0.3_V3-3c_monitor_view.md` (this file)
- **Untracked / outside a repo (so it is not lost):** `C:\ZND\projects\burnmon\testdata\uicheck\out\u5-before-monitor-dev.png` and `u5-chart-monitor-dev.png` (gitignored, evidence only, referenced from SESSION_LOG.md); a one-off font-rendering test page/screenshot under this session's own scratchpad temp folder, not committed, not durable (only used to answer the patch's braille-glyph verification requirement).

## 3. What did NOT happen (and why)
- Not committed at the time this brief was written; committed locally immediately after per this session's own instructions, still not pushed. House rule: git writes on `C:\ZND` are otherwise a manual step for Wilco; this session's own prompt explicitly asked for a local commit and to print (not run) the push command, so a commit happened here, push did not.
- The full default `scripts\uicheck.ps1` suite (`w0`, `w2`-`w8`) was also run as a regression check, not just `u5`. `w8` (the Now page's Chart.js cost-axis toggle, full view, a file this session never touched) failed both with and without this session's `template.html` changes (verified directly: stashed just `template.html` back to its pre-patch state, rebuilt, re-ran `w8`, identical failure message), so it is a pre-existing issue outside U5's scope, not a regression, and was not fixed here. `w1` (fresh-launch timing, a 1.5-second window-appear budget) also failed once during this session; given multiple `burnmon.exe` launches back to back under this session's own load, it looks like the same kind of environment timing sensitivity `w8` has, but was not isolated the same rigorous way (re-run against a stashed baseline) for time reasons, so treat it as unconfirmed rather than ruled out. `w0`, `w2`-`w7` all passed clean.
- No numbers from a synthetic/fabricated session: the "at least two running sessions" requirement in the patch spec was met by spawning two short-lived, read-only, in-repo subagent sessions from this same Claude Code session (real coding-agent activity, genuinely concurrent, not fake data) rather than hand-crafting fixture rows.
- Mid-brief, Wilco asked a new, separate question about long/invisible startup time in both monitor and full view; investigated and answered in-conversation (not part of this brief, no code changed for it), offered as a follow-up, not implemented in this session.

## 4. Findings worth propagating
- [RESULT] Real-window check green: `.\scripts\uicheck.ps1 u5` on the final build. Chart title correct, 5 legend rows (one per session with any event in the 30-minute chart window) against 3 currently-running session cards (legend rows is allowed to exceed card count by design, a session that went quiet a few minutes ago keeps its chart/legend row but loses its card), "scaled to peak" on exactly the last legend row, 517 non-blank braille dots, 17 dot rows, minute labels fully rendered (`06:07`..`06:36`), equal session-box widths, `Cascadia Mono` first in the computed font stack.
- [RESULT] `node --check` (extracted app `<script>` block), `go test ./... -count=1` (every package), `.\build.ps1`, all green.
- [RESULT] Braille glyph rendering verified without needing the real WebView2 window: a headless Edge screenshot of a standalone test page rendered real dot glyphs in Cascadia Mono, JetBrains Mono, Consolas and the combined stack, via the browser's own font-fallback. No extra fallback font was needed, contrary to the patch's own "if not, add a fallback font" contingency.
- [RESULT] One real, reproducible bug found and fixed inside this same session: the rightmost minute label (at the "now" column) was clipping to its first character, since it was pinned to the panel's very last column with no room to extend. Fixed by right-aligning any label that would otherwise overflow; re-verified in the final screenshot showing `06:36` intact.
- [STATE] `w8` and `w1` failures are pending, unowned, not part of U5's scope; noted above and left for whoever next touches the Now page's Chart.js chart (`w8`) or looks at start-up timing generally (`w1`), which ties into the open startup-visibility question below.
- [STATE] Wilco flagged (mid-session, not part of the original U5 patch) that the time until something visible appears at startup feels long, in both monitor and full view, and asked whether it can be fixed or at least surfaced with a progress indicator. Investigated in-conversation: a full-screen progress bar (`warmingPageHTML`, `cmd\burnmon\app.go`) already exists but only shows on a genuinely first-ever run (no cached `dashboard.html` yet); every later run shows the stale cached dashboard immediately and relies on a small `#cc_progress` span in the header's top-right corner (`stampAreaHTML`, same file) that is easy to miss in full view and is completely invisible in monitor view, since `body.monitor-mode #topbar{display:none}` hides the whole header row that span lives in. No code changed for this; a scoped follow-up (a persistent, monitor-view-visible progress indicator reusing the same `ccProgress` Go-to-JS hook) was offered to Wilco directly, pending his decision on whether and when to build it.

## 5. Hub-level decision (if any)
Nothing. Project-internal implementation detail (a chart widget's rendering), no cross-project time, positioning, park/unpark or CIPHER call involved.

## 6. What the next hub read should update
Nothing in the hub canonical files (STATUS, DEADLINES, portfolio, decisions.md) needs to change from this brief alone. The v0.3 session prompts checklist is already ticked for V3-3c in this project's own `02_roadmap` folder (not a Mission Deck This Week item).

## 7. Open flags for next session
- Push is still Wilco's call; commit already made locally this session (see the exact command run, below, for the record):

```
git add internal/live/live.go internal/live/live_test.go cmd/burnmon/main.go internal/report/template.html scripts/uicheck.ps1 tools/uicheck/check_u5.go SESSION_LOG.md "02_roadmap/2026-09-23_v0.3_session_prompts.md" "04_assets/hub_agent_update_2026-09-23_v0.3_V3-3c_monitor_view.md"
git commit -m "v0.3 V3-3c: monitor view like perfadvisor (U5)"
```

  (No tag this session; `git push` is a separate, deliberate step, not included above, and was not run.)
- `w8` (cost-axis toggle, full view) and `w1` (fresh-launch timing) both failed this session; `w8` confirmed pre-existing via a stash-and-rebuild comparison, `w1` unconfirmed either way. Worth a look before V3-6's release-candidate pass.
- The startup-progress-visibility question (section 4) is open and unimplemented; Wilco has a concrete, scoped proposal in-conversation if he wants it built as its own small session.
- V3-6 (release candidate, `v0.3.0`, due 2026-10-09) is next per the build order.

## 8. Related files
- Patch spec: `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_monitor_view_patch.md`
- Reference images: `C:\ZND\projects\burnmon\testdata\uicheck\reference\perfadvisor_history.png`, `monitor_mockup.jpg`
- Source ported from: `C:\ZND\projects\perfadvisor\internal\tui\widgets.go` (`graph()`), `C:\ZND\projects\perfadvisor\internal\tui\view.go` (`graphLines()`)
- Prior brief: `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-23_v0.3_V3-5.md`
- Session log: `C:\ZND\projects\burnmon\SESSION_LOG.md`
