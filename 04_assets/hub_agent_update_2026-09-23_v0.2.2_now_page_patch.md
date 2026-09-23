# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 (v0.2.2 Now page patch session) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** Report the v0.2.2 patch: six fixes from Wilco's own live review of the Now page
(three screenshots, 2026-09-23 around 12:00), worked N1 to N6 in order.
**Read order:** this file, `C:\ZND\projects\burnmon\STATUS.md`, `C:\ZND\projects\burnmon\SESSION_LOG.md`
**Supersedes:** nothing

## 1. Headline
Six items, all fixed and gate-checked (`node --check`, `go test ./... -count=1`,
`.\build.ps1`, all green after every item): the turn drawer's close bug traced to the same
UI-thread mechanism v0.2.1 already fixed elsewhere, not a new bug; the Live burn chart
rebuilt from smoothed lines to clustered per-minute bars; a spacing fix; the vendor strip
links de-styled; the context-window book corrected after a live check (Sonnet 5, Opus 5.5
and Fable 5.1 run a native 1M-token window, not 200K, reversing a dated assumption from one
day earlier); the Codex "(model unknown)" card traced to a header-rescan gap and fixed the
same way an earlier session fixed the equivalent surface bug. One capability gap flagged
openly: this session cannot open or click the native WebView2 window, so every item was
verified by proxy (rendered-logic review, `node --check`, `go test`, `burnmon-cli.exe live
--json` against the real store) rather than by watching the running app; a Claude Code
session was live throughout (this session's own), no Codex CLI session happened to be
running concurrently. Committed locally, not pushed.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (local commit, not pushed): all six fixes,
  STATUS/roadmap/SESSION_LOG updates, this brief, version bump to `0.2.2`.
- **Files touched** (full paths):
  - `C:\ZND\projects\burnmon\cmd\burnmon\main.go` (N1: `bmTurn` binding now returns
    immediately and resolves through a goroutine plus `asyncResolveJS`, matching
    `bmSessionInsight`/`bmHistory`'s existing v0.2.1 pattern; version to `0.2.2`)
  - `C:\ZND\projects\burnmon\internal\report\template.html` (N1: `TURN_DETAIL_PENDING`,
    `__bmTurnResolve`, an `Escape`-key close handler; N2: `drawNowChart` rewritten to
    clustered per-session-per-minute bars, `VENDOR_COLOR_FAMILIES`, `shadeHex`,
    `sessionSeriesColor`, `nowChartOnClick` rewritten for the new bar-click-to-drawer
    behaviour, finding-marker point layer removed; N3: `.now-cards` gets a 28px top
    margin; N4: `#t_vendorstrip td a` de-styled plus a `title` tooltip)
  - `C:\ZND\projects\burnmon\internal\live\live.go` (N2: `BucketSeconds` 10 to 60, doc
    comments updated; the window-truncation logic needed no other change)
  - `C:\ZND\projects\burnmon\internal\live\live_test.go` (N5: `TestBuildSnapshot_RunningVsStale`'s
    context-window assertion moved from 200,000 to 1,000,000)
  - `C:\ZND\projects\burnmon\internal\pricing\pricing.go` (N5: `ContextWindows` table,
    Opus 5.5/Sonnet 5/Fable 5.1 to 1,000,000 tokens, Haiku 4.5 unchanged at 200,000; the
    Opus key itself renamed `claude-opus-5` to `claude-opus-5-5` to match what the real
    store actually reports; `ContextWindowBookDate` to 2026-09-23)
  - `C:\ZND\projects\burnmon\internal\insight\insight_test.go` (N5:
    `TestAnalyze_ContextRunway_ExpectedTurnCount` switched to `claude-haiku-4-5-20251001`,
    the one model still booked at 200,000, to keep its hand-computed turn-count math valid)
  - `C:\ZND\projects\burnmon\internal\adapter\codex\codex.go` (N6: `scanHeaderMeta` now
    also tracks the most recent `turn_context.model` across its existing 64KB header
    window, seeded into `Parse`'s `model` for incremental reads)
  - `C:\ZND\projects\burnmon\internal\adapter\codex\codex_test.go` (N6:
    `TestParseIncrementalReadKeepsSurface` extended with a `Model` assertion)
  - `C:\ZND\projects\burnmon\cmd\burnmon-cli\main.go` (version to `0.2.2`)
  - `C:\ZND\projects\burnmon\STATUS.md`, `C:\ZND\projects\burnmon\02_roadmap\roadmap.md`,
    `C:\ZND\projects\burnmon\SESSION_LOG.md` (v0.2.2 recorded as item 5, v0.3 renumbered
    to item 6, decision to item 7)
  - `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-23_v0.2.2_now_page_patch.md`
    (this file)
- **On a branch, not merged:** none; all work is on `main`, committed locally.
- **Untracked / outside this session's scope:** `04_assets\_grill_state.md` and
  `DEADLINES.md` were already modified, and `02_roadmap\2026-09-23_v0.3_session_prompts.md`
  / `02_roadmap\2026-09-23_v0.3_spec.md` already untracked, at session start from earlier
  work; left untouched and not committed by this session.

## 3. What did NOT happen (and why)
- Not pushed to `origin/main`. Tag/push commands handed to Wilco at the end of this
  session's own reply, per the session prompt's own instruction.
- The native WebView2 window was never opened or clicked by this session: no windowing or
  screenshot tool is available in this environment. Every item was verified by proxy instead
  (see Headline and SESSION_LOG.md's own paragraph for exactly which proxy per item).
  Wilco should still do one pass in the real window before tagging, given this gap.
- N6 rests on a real rollout file read plus a unit test, not a live concurrent Codex CLI
  session: none happened to be running during this session, and this session cannot start
  one as a separate interactive process.
- The Now chart's colour table (N2) defines a lighter shade for "Codex Desktop" and
  "Copilot in VS Code" that no session can reach yet, since neither adapter distinguishes
  that surface from its CLI counterpart today (Copilot in VS Code is not an adapter at all,
  v0.3 scope per STATUS.md). Left in as forward-compatible definitions, not implemented as
  a fresh adapter distinction, and named in STATUS.md's Known gaps rather than silently
  dropped from N2's own colour spec.

## 4. Findings worth propagating
- [RESULT] N1's actual root cause was the same UI-thread mechanism the v0.2.1 hang patch
  already diagnosed and fixed for two other bindings (`bmSessionInsight`, `bmHistory`):
  `bmTurn` was the one binding v0.2.1 missed, still running synchronously on the WebView2
  UI thread, so opening the drawer on the real store's scale could make the whole window,
  including its own Close button, unresponsive for long enough to read as "cannot close".
- [RESULT] N5's live check (Perplexity, citing platform.claude.com/docs) found the
  project's own context-window book was stale by one day: Sonnet 5, Opus 5.5 and Fable 5.1
  all default to a native 1,000,000-token window now, no beta header needed, contradicting
  the prior session's dated comment that assumed a beta-only 1M tier. The same live check
  against the real store also caught a second, independent bug: the book's Opus entry was
  keyed `claude-opus-5`, a string no real session has ever produced; the real Cowork session
  observed reports `claude-opus-5-5`.
- [RESULT] N6's root cause, read directly off a real rollout file (868 lines, 7
  `turn_context` lines against 114 `token_count` lines): `scanHeaderMeta` already re-read
  `session_meta` for cwd/surface on every incremental parse (F2's own fix) but never looked
  at `turn_context` at all, so most live polls landed in the gap between two `turn_context`
  lines and fell back to an empty model. Same class of bug as F2, same fix shape.
- [RESULT] `go test ./... -count=1` green across every package after every item; `node
  --check` on both extracted script blocks passed; `.\build.ps1` green (both exes built).
- [STATE] Local commit exists on `main`, not pushed.

## 5. Hub-level decision (if any)
None needed this session. Every open item is project-internal, recorded in BurnMon's own
STATUS.md Known gaps, not a hub-level call.

## 6. What the next hub read should update
`10_holding\01_projects\burnmon.md` may want a one-line note that v0.2.2 shipped without a
real-window pass (no windowing tool in this environment), so a quick manual check before
the tag is pushed is still worth doing, not a surprise gap to rediscover later.

## 7. Open flags for next session
- Push and tag `v0.2.2` to `origin/main`, Wilco's call; commands below.
- A real-window pass over all six items (drawer close/Esc, chart bars and colours, chart
  spacing, vendor strip link hover, corrected context windows on the Claude Code card, the
  Codex card's model label) before or shortly after tagging, since this session could not
  do that pass itself.
- v0.3 scope (per-vendor cost, dev/business switch, full client map, macOS/Linux builds,
  dated 2026-10-09) unchanged, still the next real work after this patch.

## 8. Related files
- `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.2.2_now_page_patch.md` (this session's spec/prompt)
- `C:\ZND\projects\burnmon\STATUS.md`, `C:\ZND\projects\burnmon\SESSION_LOG.md`,
  `C:\ZND\projects\burnmon\02_roadmap\roadmap.md`
