# Session prompts: WS2 phase 5, WS3, WS2 performance patch (2026-09-26)

Run in this order, each in a fresh Claude Code session on Sonnet 5, each only after the previous
one has stopped and Wilco has read its report. Never use em dashes anywhere.

## 1. WS2 phase 5 (worktree `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev`)

```text
You are doing phase 5 (verify and release) of WS2 (BurnMon Dev) on branch burnmon-dev, in this worktree, with fixes first. Phases 0 to 4 and three UI patches are done (last commits 493a84d, 516c24e). Keep the laptop session unlocked in mind: if the workstation locks, stop and say so.
Read first: C:\ZND\projects\burnmon\02_roadmap\2026-09-24_ws2_burnmon_dev.md ("Verify and close"); the three patch specs 2026-09-25_ws2_*.md in the same folder; git log --oneline main..burnmon-dev; git log --oneline -3 main; top two SESSION_LOG entries; AGENTS.md and C:\ZND\AGENTS.md. Read-only git except the rebase and commits.
Work, in order:
1. Headline per tick: today it only changes on the 60 s vendor-strip refresh. Make today's total = cached today total + tokens ingested since that cache point, from data the shared tick already fetches, every tick, no extra full-store query. Keep the 2 s linear tween. Unit test across a cache refresh (no jump back, no double count). Watch 30 s during an active session, report how often it changed.
2. uicheck in CSS pixels: read GetDpiForWindow and size windows so "1152x2048" means CSS px, capped at the screen, logging when it cannot. Re-run d0 to d12, report real CSS sizes.
3. Rebase on main (v0.3.1, 1291ef9). Shared code conflicts resolve in favour of main. Then go vet, go test ./... -count=1, .\build.ps1, node --check on both page JS files, uicheck for both exes.
4. Commit every untracked screenshot in 04_assets\reference\2026-09-25_ui_review\.
5. Measure burnmon-dev.exe for 10 min at refresh_ms 2000 with burnmon.exe running: avg and peak CPU (of whole machine), peak RAM, handles. Do not fix shared ingest code (that is WS3).
5b. Split cadences: paint and cheap sources every refresh_ms; process walk every 3 s; persist every 10 s by wall clock. Measure 10 min at refresh_ms 1000. Default becomes 1000 only if under 2 percent avg CPU and less than 0.5 point above the 2000 run. Explain and fix the unit of the "BurnMon Dev 47%" process-groups value.
6. Re-run uicheck d7 (real F11).
7. Confirm 0.4.0-alpha.1 everywhere. README (BurnMon Dev section: what, start, F11, To Do opt-in, export), STATUS, SESSION_LOG.
8. Fresh read-only Opus review over main..burnmon-dev. Fix, commit.
9. One hub brief with the hub-agent-update skill in 04_assets\.
Do not push, tag or merge. End with the PowerShell commands to push the branch (origin/burnmon-dev already holds the pre-rebase UI review commits, so use git push --force-with-lease) and tag v0.4.0-alpha.1, and the steps for Wilco's own Microsoft To Do sign-in. Report: headline change rate, CSS sizes, rebase conflicts, tests, both 10-minute runs, d7, review findings.
```

## 2. WS3 shared ingest performance (repo `C:\ZND\projects\burnmon`, branch `main`)

```text
You are doing WS3 (shared ingest performance) on branch main in C:\ZND\projects\burnmon. WS2 phase 5 has finished; its hub brief in 04_assets\ holds the baseline numbers.
Read first: C:\ZND\projects\burnmon\02_roadmap\2026-09-26_ws3_shared_ingest_performance.md (your spec), AGENTS.md, C:\ZND\AGENTS.md, STATUS.md, the top SESSION_LOG entry, the phase 5 hub brief. Read-only git except commits.
Do step 0 (profile and report) before any change. Then test the four hypotheses in order, fixing only what the profile confirms. Fresh read-only Opus review before each commit. Do not push or tag.
Report: before and after numbers for burnmon.exe alone and with burnmon-dev.exe, what each hypothesis turned out to be, live-ingest latency, test results, review findings, and the PowerShell commands for Wilco.
```

## 3. WS2 performance patch (worktree `C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev`)

```text
You are doing the WS2 performance patch for BurnMon Dev on branch burnmon-dev, in this worktree. WS3 is committed on main.
Read first: C:\ZND\projects\burnmon\02_roadmap\2026-09-26_ws2_performance_patch.md (your spec, absolute path, NOT in this worktree), AGENTS.md, C:\ZND\AGENTS.md, the top SESSION_LOG entry, the phase 5 and WS3 hub briefs in C:\ZND\projects\burnmon\04_assets\. Read-only git except the rebase and commits.
Step 0 is the rebase on main. Then items 1 to 4. Fresh read-only Opus review before each commit. Do not push, tag or merge.
Report: before and after numbers (Go and WebView2 separately, and minimized), test results, screenshots of process groups, review findings, and the PowerShell push commands.
```

## Launch pattern (PowerShell on the laptop)

```powershell
# runs in: PowerShell on the laptop. Set $dir to the folder named in the prompt's heading, paste the prompt text into $prompt.
$dir = 'C:\ZND\projects\burnmon\.claude\worktrees\burnmon-dev'
cd $dir
$prompt = @'
<paste the prompt text here>
'@
claude --model claude-sonnet-5 $prompt
```
