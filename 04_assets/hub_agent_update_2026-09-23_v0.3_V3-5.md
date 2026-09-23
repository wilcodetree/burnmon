# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** V3-5 (Copilot in VS Code, A4; macOS/Linux builds, B1) shipped and committed.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md`'s newest entry
(2026-09-23, v0.3 V3-5) for the full detail.
**Supersedes:** nothing

## 1. Headline
V3-5 is done and committed (`7545d63`, branch `main`, not pushed): GitHub Copilot Chat in
VS Code now feeds BurnMon via its own OTel-file export, and `burnmon`/`burnmon-cli` now
cross-compile for macOS and Linux (amd64 and arm64, `CGO_ENABLED=0`), both labelled
untested. Two items left in the v0.3 build order (V3-6, release candidate, due 2026-10-09).

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (repo root): `7545d63` "v0.3 V3-5: Copilot in
  VS Code (A4) and macOS/Linux builds (B1)". 17 files changed.
- **Not merged/pushed:** this is a local commit on `main`; per the session prompt's own
  rule, the session stops before tagging or pushing. Tag command to run when Wilco is
  ready: `git tag v0.3.0-beta.1` then `git push origin main --tags`.
- **Files touched** (full paths): `C:\ZND\projects\burnmon\internal\adapter\copilotvsc\`
  (new package), `C:\ZND\projects\burnmon\testdata\copilotvsc\copilotvsc_fixture.jsonl`
  (new fixture), `C:\ZND\projects\burnmon\cmd\burnmon\app.go` (new, shared app logic),
  `C:\ZND\projects\burnmon\cmd\burnmon\main_other.go` (new, darwin/linux entry point),
  `C:\ZND\projects\burnmon\cmd\burnmon\main.go` (trimmed to Windows-only WebView2 code),
  `C:\ZND\projects\burnmon\.github\workflows\release.yml` (new release workflow),
  `C:\ZND\projects\burnmon\internal\adapter\hermes\hermes.go`,
  `C:\ZND\projects\burnmon\internal\scan\wsl_other.go`,
  `C:\ZND\projects\burnmon\internal\vendorstrip\vendorstrip.go`,
  `C:\ZND\projects\burnmon\internal\history\history.go`,
  `C:\ZND\projects\burnmon\internal\pricing\pricing.go`,
  `C:\ZND\projects\burnmon\internal\report\template.html`,
  `C:\ZND\projects\burnmon\README.md`,
  `C:\ZND\projects\burnmon\burnmon.example.json`,
  `C:\ZND\projects\burnmon\SESSION_LOG.md`,
  `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_session_prompts.md` (V3-5 ticked).
- **Untracked, left alone (not this session's work, concurrent with Wilco's own):**
  `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_spec.md` (a U5 addition appeared
  mid-session, not made by this session) and a new
  `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_monitor_view_patch.md`; also
  `C:\ZND\projects\burnmon\04_assets\_grill_state.md` and
  `C:\ZND\projects\burnmon\DEADLINES.md` were already modified before this session started
  and were left as found, not folded into this commit.

## 3. What did NOT happen (and why)
Not tagged, not pushed (house rule: stop before tagging, print the commands, Wilco's own
step). V3-6 (release-candidate Done-when pass, version bump to 0.3.0, README/STATUS
rewrite) not started; the README still carries some pre-v0.3 wording that is explicitly
flagged in-file as V3-6's job, not silently fixed here. The macOS/Linux builds are
compile-verified and cross-compiled for real (all four combinations), never run on actual
macOS or Linux hardware; every place that matters says so ("untested"). No CI run has
actually exercised `.github\workflows\release.yml` yet (it only fires on a `v*` tag push,
which has not happened).

## 4. Findings worth propagating
- **[RESULT]** Copilot Chat in VS Code's real OTel span file (Wilco's own laptop, 1,793
  lines, 14.7MB) carries no prompt text and no workspace/folder attribute anywhere,
  confirmed by scanning every attribute value on every line. Client shows "unassigned" for
  every Copilot-VS-Code event until GitHub adds a workspace attribute to the export.
- **[RESULT]** Real end-to-end verification, not just the fixture: pointed a scratch
  config at the real live OTel file, ran `burnmon.exe` for real, and read the actual
  running window's DOM: the vendor strip showed a real "Copilot (VS Code)" row,
  0/412K/412K tokens (today/week/month), confirming config through the store to the page
  works against real data.
- **[RESULT]** `burnmon` and `burnmon-cli` cross-compile cleanly (`CGO_ENABLED=0`) for
  darwin/amd64, darwin/arm64, linux/amd64, linux/arm64: all eight binaries built with zero
  errors this session.
- **[STATE]** V3-6 (release candidate) is the one remaining v0.3 session before the
  2026-10-09 tag; nothing else in the build order is open.

## 5. Hub-level decision (if any)
Nothing. This session's one judgement call (using the two settings the spec names,
`github.copilot.chat.otel.enabled` and `github.copilot.chat.otel.outfile`, in the README
rather than all four settings the earlier A3 correction verified live) is project-internal
documentation wording, not a hub-level call.

## 6. What the next hub read should update
- `C:\ZND\10_holding\01_projects\burnmon.md` (project one-pager): V3-5 done, V3-6 next.
- `C:\ZND\10_holding\02_roadmap\roadmap.md`: no change to dates (2026-10-09 holds).
- This Week plan: no named A/B item covers BurnMon sessions directly; nothing to flip here
  unless the hub's own tracking names V3-5 explicitly.

## 7. Open flags for next session
V3-6 needs the real Done-when pass against the built exe and real store (not fixtures),
every VERIFY item resolved or explicitly left "still unverified" in the UI, version bump
to 0.3.0, and the README/STATUS rewrite this session's README edits explicitly deferred to
it. Tag command once Wilco is ready: `git tag v0.3.0-beta.1 && git push origin main --tags`
(the spec calls for this tag after V3-5; not run this session, per the stop-before-tagging
rule).

## 8. Related files
`C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_spec.md` (sections 2.3 A4, 2.4 B1),
`C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_session_prompts.md` (V3-5 prompt and
checklist), `C:\ZND\projects\burnmon\SESSION_LOG.md` (2026-09-23, v0.3 V3-5 entry, full
detail).
