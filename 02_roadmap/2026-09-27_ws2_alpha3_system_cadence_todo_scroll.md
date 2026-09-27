# WS2 alpha.3 patch: one 1 s cadence for System, equal core bars, To Do scroll

Owner: Wilco. Executor: fresh Claude Code session, Sonnet 5 (Opus for the review agent).
Branch: `main` in `C:\ZND\projects\burnmon` (BurnMon Dev was merged in `f7f1c26`). Version
v0.4.0-alpha.3 for `burnmon-dev.exe` only. Never use em dashes anywhere.

Read first: `AGENTS.md`, `C:\ZND\AGENTS.md`, top of `SESSION_LOG.md`,
`C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-26_ws2_performance_patch.md` (its
numbers are the baseline), and the reference screenshot
`C:\ZND\projects\burnmon\04_assets\reference\2026-09-27_system_panel\system_panel_core_bars.png`
(Wilco's red boxes mark the unequal core bars).

## Wilco's requests (2026-09-27)

### 1. To Do panel scrolls vertically

Today `.todobody` in `cmd\burnmon-dev\page.html` is a flex column with `overflow:hidden`, and
`.todorow` can shrink, so rows are squeezed into the panel height and overlap until nothing is
readable. Fix: rows keep their natural height (`flex:0 0 auto`), `.todobody` gets
`overflow-y:auto`, `overflow-x:hidden`. Every row readable, the panel scrolls.
This is Wilco's second exception to "no scrollbars except the turn ticker": update the comment
block at the top of `page.html` and `tools\uicheck\check_d9.go` so the To Do body is allowed a
vertical scrollbar and nothing else is.
Privacy stays as decided: never put To Do content in a committed screenshot. uicheck runs with the
panel off or with fake rows.

### 2. All 20 core bars the same length

In `.coregrid` each `.coreitem` is label, bar, value; the value text ("0%", "20%", "100%") has
its own width, so each bar has a different length. Fix: fixed slots on both sides, three
characters for the numbers: left slot fits "C19", right slot fits "100%", both
`font-variant-numeric:tabular-nums`, value right-aligned. Every bar then has the identical track
length at every window size. Prove it in uicheck: measure all 20 `.corebar` widths, report
min and max, they must be equal.

### 3. One 1 s cadence for the whole System zone, like perfadvisor

Everything in the System zone updates on the same 1 s tick and moves together: the CPU total
bar, the 20 core bars, the main chart (cpu, ram, disk, net, gpu), the process-groups rows and
their trend sparklines, and the Memory, Disks and Network boxes. Today the process walk runs on
its own `processWalkInterval = 3 * time.Second` (`cmd\burnmon-dev\app.go` line ~429), so process
groups lag the rest.
- Read how perfadvisor does it first (`C:\ZND\projects\perfadvisor`, its sampler and its page)
  and follow the same model: one sample per second, one paint per sample.
- One sampler tick per second produces one snapshot for all of the above; the page paints that
  snapshot once. No panel on its own timer. Disk and network rates are per-second deltas of that
  same tick.
- The burn zone and headline keep `refresh_ms` (default 1000, so in practice the same beat).
  Persistence stays at 10 s wall clock.
- Hidden or minimized: everything, including the process walk, drops to 10 s, restore with one
  immediate tick (as today).
- Cost guard: the process walk is now one `NtQuerySystemInformation` snapshot, so 1 s should be
  cheap, but measure it. Go process under 2 percent average CPU of the whole machine and under
  250 MB peak RAM over 10 minutes with `burnmon.exe` running. If 1 s breaks either target, stop
  and report the numbers, do not ship a slower cadence silently.
- Prove the shared beat in uicheck (extend `check_d11.go`): over 30 s, the process-groups panel
  and the main chart change on the same ticks, report both counts.

## Done when

- Items 1 to 3 above, each with its own proof.
- 10-minute measurement active (Go and WebView2 separately) and 5 minutes minimized, same method
  as the performance patch, before and after.
- `go vet ./...`, `go test ./... -count=1`, `.\build.ps1`, `node --check` on both page scripts,
  uicheck d0 to d12 in CSS pixels (full list with sizes), `.\scripts\uicheck.ps1`.
- Fresh read-only Opus review over the diff before each commit; name the model in the report.
- Version v0.4.0-alpha.3 in `cmd\burnmon-dev`; README, STATUS, SESSION_LOG. Commit on `main`, do
  not push or tag. Hand Wilco the PowerShell commands.
- One hub brief with the `hub-agent-update` skill in `C:\ZND\projects\burnmon\04_assets\`.
