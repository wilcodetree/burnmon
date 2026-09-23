# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 - **Owner:** Wilco de Tree
**Project:** BurnMon (formerly claudecost)
**Purpose:** 43B (A2, A3) shipped and tagged: Copilot CLI adapter built, live-corrected the VS Code OTel answer from no to yes.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md` (top two entries), then `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` section 2.5
**Supersedes:** `C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.2_43A_markers_ticker_drawer.md` (extends it, does not replace it)

## 1. Headline
43B is shipped, merged to `main` and tagged `v0.2.0-alpha.4`: `internal/adapter/copilotcli`
built and wired in, A2's plan assumption (Copilot CLI totals in `data.db`, written at
session end) confirmed false on a live install, adapter built like Hermes instead by
Wilco's own call. A3 (can VS Code Copilot emit OpenTelemetry to a local file) was first
answered no from a CLI-only test, then corrected to yes the same day once Wilco reloaded
the VS Code window and sent one real chat message. Both committed, pushed, build green.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (branch `main`, pushed to `origin/main`):
  - `876c37e` "feat: Copilot CLI adapter, VS Code OTel check answered no (A2, A3)"
  - `b6caa93` "docs: correct A3 finding to yes, GitHub Copilot VS Code does emit OTel"
- **Tagged:** `v0.2.0-alpha.4` on `876c37e`.
- **Files touched** (full paths):
  `C:\ZND\projects\burnmon\internal\adapter\copilotcli\copilotcli.go` (new),
  `C:\ZND\projects\burnmon\internal\adapter\copilotcli\copilotcli_test.go` (new),
  `C:\ZND\projects\burnmon\testdata\copilot\copilot_fixture.db` (new, three real
  sessions' `sessions` and `assistant_usage_events` rows only, no message content),
  `C:\ZND\projects\burnmon\cmd\burnmon\main.go` (`startCopilotCLIPoll`, same shape as
  the existing Hermes poll),
  `C:\ZND\projects\burnmon\SESSION_LOG.md` (43B entry, then the 2026-09-23 correction
  entry above it),
  `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (43B ticked,
  then re-annotated with the correction).
- **Untracked, not this session's work:** `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1.2_patch_spec.md`
  shows modified in `git status` but predates this session; left alone throughout.

## 3. What did NOT happen (and why)
44A (I3 findings on the Sessions tab, next in the build order) has not started. The
`github.copilot.chat.otel.*` VS Code settings used to test A3 are still enabled at
Wilco's own choice (see section 7): no product-code change was made for A3 either way,
per the spec's own instruction. `%LOCALAPPDATA%\Temp\copilot-otel.jsonl`, the test
outfile, is real local data on Wilco's own machine, not something this brief copies or
quotes beyond the field names already public in the OTel spec (`gen_ai.*` semantic
conventions); it was not moved into the repo or committed anywhere.

## 4. Findings worth propagating
- [RESULT] A2: Copilot CLI's real store is `session-store.db` (WAL mode), not `data.db`,
  and it writes `assistant_usage_events` incrementally per API call throughout a turn,
  not once at session close; no `ended_at`/`status`/`closed` column exists anywhere in
  its schema. Confirmed on Wilco's live install plus three real fixture sessions he
  recorded the same day. Full schema and evidence in `SESSION_LOG.md`'s 43B entry.
- [RESULT] Per Wilco's own call on the open choice this created, the adapter was built
  like Hermes (5-second poll, last-row-wins per session), not the plan's "totals at
  session end" framing, since that framing depended on the killed assumption.
- [RESULT] A3, corrected: GitHub Copilot Chat (VS Code 1.138.0's own built-in, v0.66.0)
  can emit OpenTelemetry to a local file. First test (CLI-only, `code chat "..."`, no
  window reload) produced an empty file and was reported as "no"; second test (window
  reload, one real message sent in the Chat panel) produced 476KB of real spans with
  `gen_ai.usage.input_tokens`, `gen_ai.request.model`, `event.name":
  "copilot_chat.session.start"`. The gap was the missing reload, not the settings, the
  extension, or a fake test.
- [RESULT] `go test ./... -count=1` and `.\build.ps1` both green on the code change (43B);
  the correction itself changed no code, docs only.
- [STATE] 44A (I3 Sessions tab findings) is next per the build order table, week 44,
  not yet started.

## 5. Hub-level decision (if any)
One flagged, not yet made: A3 being "yes" triggers the spec's own stated consequence
("yes means Copilot VS Code enters v0.3"). That is a v0.3 scope call, not something
decided or applied in this session; left for Wilco when v0.3 is planned.

  **Decision:** pending, not yet made.
  **Context:** the v0.2 spec (`2026-09-22_v0.2_spec.md`, section 2.5 A3) states "yes
  means Copilot VS Code enters v0.3, no means the Now page stops promising it." A3 is
  now confirmed yes.
  **Alternatives considered:** none yet, this is a flag for the v0.3 planning session,
  not a decision made here.
  **Consequences:** if adopted, v0.3 gains a Copilot VS Code adapter reading the OTel
  file/db-span-exporter path instead of a store like the other adapters; if declined,
  the "yes" finding stands on record but nothing is scoped in.
  **Links:** `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` section 2.5;
  `C:\ZND\projects\burnmon\SESSION_LOG.md` (both the 43B entry and the 2026-09-23
  correction entry).

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` (hub one-pager): 43B done, `v0.2.0-alpha.4`
  tagged, A3 corrected to yes, next up 44A.
- `C:\ZND\10_holding\02_roadmap\roadmap.md` if it tracks BurnMon by build-order week:
  week 43 (43A and 43B) now closed.
- No DEADLINES.md change expected: the v0.2 release date (2026-11-14) and the
  two-session-a-week cadence are unaffected. A v0.3 scope note (Copilot VS Code
  adapter, pending decision above) may belong in a v0.3 planning doc once one exists.

## 7. Open flags for next session
44A needs the Sessions tab findings column and drawer wiring, per the build order. The
v0.3 scope call in section 5 (Copilot VS Code adapter, yes/no) is open for whenever v0.3
is planned, not urgent now. Wilco chose to keep the four `github.copilot.chat.otel.*`
VS Code settings enabled going forward rather than reverting them after the test; the
outfile grows unbounded while they stay on, worth a periodic check or cleanup, not
tracked by any BurnMon code.

## 8. Related files
`C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_spec.md` (section 2.5, A1-A3),
`C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.2_session_prompts.md` (43B prompt,
checklist line 149),
`C:\ZND\projects\burnmon\04_assets\2026-09-22_token_monitor_sources_and_facts.md`
(Copilot rows, section C),
`C:\ZND\projects\burnmon\SESSION_LOG.md` (43B entry and the 2026-09-23 correction entry,
top of file),
`C:\ZND\projects\burnmon\04_assets\hub_agent_update_2026-09-22_v0.2_43A_markers_ticker_drawer.md`
(the 43A brief this one extends).
