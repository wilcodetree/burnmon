# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-26 - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** report that the WS2 ticker/headline patch (sections 1-4 of the ticker/headline spec) is code-complete and committed on burnmon-dev, verified against the real running window, not yet released.
**Read order:** this file, then C:\ZND\projects\burnmon\02_roadmap\2026-09-25_ws2_ticker_headline_patch.md, then SESSION_LOG.md top entry in the worktree.
**Supersedes:** C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-25_burn_chart_no_scroll_patch.md (next patch in the same WS2 series; phase 5 there is still the same phase 5 pending here)

## 1. Headline
The ticker/headline patch (turn ticker rows fixed-height and scrollable, process-groups trend column capped to half width, harness-heatmap hover text, headline reformatted and re-tweened, header gap widened) is code-complete and committed on branch burnmon-dev, verified with go vet, go test, a real build, and a real-window uicheck battery (d0-d12, all green, rerun in full after a fresh Opus review caught five real bugs mid-session). Not merged to main, not pushed, not tagged, version deliberately unchanged at 0.4.0-alpha.1.

## 2. What changed on disk
- Committed in the burnmon repo (worktree C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev, branch burnmon-dev): 493a84d "v0.4 ticker/headline patch, sections 1-4" - cmd\burnmon-dev\page.html, tools\uicheck\check_d9.go, tools\uicheck\check_d12.go (new file), and four reference screenshots 04_assets\reference\2026-09-25_ui_review\11_ticker_broken.png, 12_process_groups.png, 13_heatmaps.png, 14_topbar.png.
- Committed, same branch: 516c24e "SESSION_LOG: v0.4 ticker/headline patch (sections 1-4) entry" - SESSION_LOG.md only.
- Files touched (full paths, all inside the worktree above): C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev\cmd\burnmon-dev\page.html, tools\uicheck\check_d9.go, tools\uicheck\check_d12.go, SESSION_LOG.md.
- Untracked, left alone on purpose (pre-existing, not from this session): C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev\04_assets\reference\2026-09-25_ui_review\5_topbar_target.jpg, 6_ticker_target.png, 7_perfadvisor_system.png, 8_current.jpg - the same four reference images the prior brief already flagged as out of scope; still untouched.

## 3. What did NOT happen (and why)
- Not merged to main, not pushed to any remote, not tagged - per house process and explicit instruction for this session.
- Version stays 0.4.0-alpha.1 everywhere it lives - the version bump is scoped to phase 5 (a separate, later session), not this patch, on explicit instruction.
- Phase 5 itself (rebase on main, final review, version bump, release commands, hub brief for the release) has not run; still pending exactly as the prior brief flagged it.
- The heatmap hover token figure is deliberately incomplete, not a bug: it only correlates against burn.turns own 30-minute/50-turn window, narrower than the heatmap own 60-minute CPU history, so an older or busier cell can show CPU with no token figure even when real activity happened. Documented in the code, not fixed, since a full fix (widening the turns source itself) is out of this small patch scope.
- No fresh burn-in / soak test beyond the uicheck battery; no multi-hour real-usage observation yet.
- The known, separate, pre-existing shared live-watch/store memory and handle-growth issue (burnmon.exe and burnmon-dev.exe both climbing toward roughly 900MB-1GB) was not touched; out of this patch scope, same as the prior brief.

## 4. Findings worth propagating
- [RESULT] All uicheck cases pass against the real running window: d0-d8 (header, export, the five viewports), d9 (no visible scrollbar anywhere except the turn ticker, its one deliberate exception), d10 (burn chart markers), d11 (paint-tick timing), and the new d12 (turn ticker rows never shorter than their own text line height, the box itself never collapsed, at least one row present to check, across all five sizes) - all green both before and after the review fixes were applied.
- [RESULT] go vet ./..., go test ./... -count=1 and .\build.ps1 all green after every edit pass, including after the review-caught fixes.
- [RESULT] A fresh, independent, read-only Opus review of the diff (spawned mid-session, no shared context with the implementation pass) caught five real issues before commit, all fixed and re-verified: animateNumber only recorded its in-flight tween value at completion, so a tween cancelled mid-flight (routine once its own duration is close to the tick cadence, exactly the headline new case) restarted from the stale pre-tween value and visibly snapped backward; the harness-heatmap hover text showed a raw per-minute CPU sum mislabeled as a percentage (could read 600% CPU for one busy harness at the default sample rate), now shows a true average; the turn ticker full-innerHTML replace every tick would have drifted a scrolled-down reader position by one row height per new turn, now skips its repaint while scrolled away from the top; a pre-existing dead CSS selector (#processGroupsHead th) that matched nothing, since that id belongs to the panel title div, not the table own thead, cleaned up in the same lines this patch was already touching; and a doc comment overclaiming the headline keeps counting up continuously when the underlying value (vendor_strip own 60-second cache) actually only changes about once a minute, corrected to describe the real behaviour.
- [RESULT] Live DOM verification (not just reading the code) against the real running app confirmed each of the three harness-heatmap hover branches renders correctly: CPU-only cells, CPU-plus-tokens cells (e.g. Claude Code, 00:09: 54% CPU, 975K tok before the average fix, later confirmed sane and bounded after it, e.g. BurnMon Dev, 00:34: 35% CPU), and empty cells (BurnMon Dev, 23:55: no activity).
- [STATE] Display scale in this session sandbox: every d-check requested window size lands at roughly half its CSS-pixel target (e.g. 1280x860 requested to 627x395 real CSS px), consistent across all five sizes and across both sweeps this session ran, pointing to roughly 200% Windows display scaling in this sandboxed box specifically. d7 own fullscreen path still reaches the full 1600x1000 CSS px this session screen actually offers, which is where the fuller-layout screenshots for this session own report were taken. Not checked against Wilco own real hardware, same caveat the prior brief already raised for this same sandboxed display.

## 5. Hub-level decision (if any)
Nothing. This is a project-internal implementation patch, not a cross-project, pricing, positioning, park/unpark or CIPHER-wall call.

## 6. What the next hub read should update
- C:\ZND\10_holding\01_projects\burnmon.md (the project one-pager): note that the ticker/headline patch (sections 1-4) landed on burnmon-dev on top of the burn-chart-no-scroll patch, phase 5 (release) still pending.
- C:\ZND\projects\burnmon\02_roadmap\2026-09-25_ws2_ticker_headline_patch.md itself has no status field to update; SESSION_LOG.md in the worktree is the record of record for this patch own history.
- No This Week / Mission Deck item name was given for this session, so section 6 own done/in_progress/blocked line does not apply here; if BurnMon WS2 is tracked as a named deck item, the hub reader should match it against this brief rather than guessing an item ID here.

## 7. Open flags for next session
- Phase 5 has not run: rebase burnmon-dev on main, a final review, the version bump, release commands handed to Wilco, and a hub brief for the release itself (separate from this one).
- The heatmap hover token-window limitation (section 3) is a known, documented gap, not urgent, but worth deciding in phase 5 whether it needs widening or is acceptable as-is long term.
- Whether the panel-drop / display-scale behaviour flagged in the prior brief needs a follow-up still depends on what Wilco actually sees on his own two screens; unchanged from before, not re-investigated this session.

## 8. Related files
- C:\ZND\projects\burnmon\02_roadmap\2026-09-25_ws2_ticker_headline_patch.md (the spec this patch follows)
- C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev\SESSION_LOG.md (top entry, full narrative of what changed and what the review caught)
- C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-25_burn_chart_no_scroll_patch.md (the immediately prior brief in this same WS2 series, superseded by this one)
- C:\ZND\projects\burnmon\04_assets\2026-09-24_burnmon_dev_design.md (the underlying BurnMon Dev design doc this patch sections build on)
- C:\ZND\projects\burnmon\04_assets\reference\2026-09-25_ui_review\ (in the worktree at ...\.claude\worktrees\burnmon-dev\04_assets\reference\2026-09-25_ui_review\ - the four newly committed screenshots plus the four still-untracked pre-existing ones)
