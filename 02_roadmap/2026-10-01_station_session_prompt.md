# Session prompt: Station integration (v0.4.0-alpha.9)

Paste this into Claude Code with cwd `C:\ZND\50_projects\burnmon`.

```text
Read C:\ZND\50_projects\burnmon\AGENTS.md, STATUS.md and C:\ZND\50_projects\burnmon\02_roadmap\2026-10-01_station_secret_screen.md.
Build "What is left", steps 1 to 8, in order. internal/stage, cmd/burnmon-dev/station/*.js and the
atlas already exist and are tested; do not rewrite them, only fix a real bug you can show.
Open 04_assets/2026-10-01_station_preview.html first to see the target behaviour.
Rules: test first for the store query and the live fields; one store call per tick for the
latest tool calls; nothing runs while the Station is closed or the window is hidden; no em dashes.
Verify with go vet, go test ./... -count=1, .\build.ps1, node --check on the extracted page script,
.\scripts\uicheck.ps1 d0 through d23 (d23 is new), then the 10+10 minute measurement from step 7.
Run with live Claude Code and Codex sessions and confirm rooms follow tools. Bump to 0.4.0-alpha.9,
update STATUS.md and SESSION_LOG.md, write the hub_agent_update brief. Do not push; give me the
exact git commands.
Model map: spec=opus, plan=opus, dev=sonnet, test=sonnet, verify=opus, iterate=sonnet, report=sonnet.
```
