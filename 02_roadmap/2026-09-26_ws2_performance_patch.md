# WS2 performance patch: BurnMon Dev's own CPU and memory

Branch `burnmon-dev`, worktree `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev`.
Executor: fresh Claude Code session, Sonnet 5 (Opus for the review agent).
Runs AFTER WS3 (`2026-09-26_ws3_shared_ingest_performance.md`) is committed on `main`.
Never use em dashes anywhere.

Read first: `AGENTS.md`, `C:\ZND\AGENTS.md`, top of `SESSION_LOG.md`, the WS2 phase 5 and WS3
hub briefs in `C:\ZND\projects\burnmon\04_assets\` (their numbers are the baseline).

## Step 0: rebase

Rebase `burnmon-dev` on `main` (WS3, v0.3.2). Shared code conflicts resolve in favour of
`main`. Build, test and uicheck both exes before any change below.

## Items (BurnMon Dev code only)

1. **Cheaper process sampler.** One system-wide snapshot per sample for CPU, RAM and IO of all
   processes (one call, not per-process queries). Read each process's command line only once,
   when its pid first appears, and cache the pid to harness mapping until the pid exits.
   Process walk every 3 s (step 5b of phase 5), independent of `refresh_ms`.
2. **Pause when nobody looks.** Window minimized or hidden: stop painting, slow system sampling
   to 10 s, and set WebView2's memory usage target to low. Restore on show, with one immediate
   fresh tick.
3. **No per-tick DOM rebuilds.** Panels that rebuild their HTML every tick (burn chart, session
   cards, ticker, process groups, vendor strip) update text nodes and canvases in place. Create
   elements once, reuse them.
4. **Honest self row.** Split the process-groups row "BurnMon Dev" into "BurnMon Dev (Go)" and
   "BurnMon Dev (WebView2)", and state in the header of the CPU column whether the percent is of
   one core or of the whole machine. Same unit everywhere.
5. **Local time everywhere (Wilco, 2026-09-26).** WS3 moved main to local-midnight days. Bring
   BurnMon Dev's own code in line: `headline.go` (`headlineDayStart` is UTC today), the
   heatmap, burn chart, turn ticker, To Do and export. Local midnight, local 24-hour times,
   DST tested. Report every `.UTC()` call kept or removed in `cmd\burnmon-dev`.
6. **Headline evidence (left open by phase 5).** Watch 30 s during an active session and
   report the exact number of headline changes out of the number of ticks.

## Done when

- 10-minute measurement, same method as phase 5 and WS3, with `burnmon.exe` also running:
  average and peak CPU, peak RAM (Go and WebView2 separately), handles. Report against the
  targets (under 2 percent average CPU, under 250 MB RAM for the Go process), before and after.
- Minimized for 5 minutes: CPU and RAM reported.
- `go vet ./...`, `go test ./... -count=1`, `.\build.ps1`, `node --check`, uicheck d0 to d12 in
  CSS pixels, screenshots of the process-groups panel.
- Fresh read-only Opus review agent over the diff before each commit.
- Version v0.4.0-alpha.2. README, STATUS, SESSION_LOG. Commit, do not push or tag. Hand Wilco
  the PowerShell commands (branch push needs `--force-with-lease` after the rebase if the
  branch was pushed before).
- One hub brief with the `hub-agent-update` skill in `C:\ZND\projects\burnmon\04_assets\`.
