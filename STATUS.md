# BurnMon, status

What is true at this commit (2026-09-24): **v0.3.0**, the release from
`02_roadmap\2026-09-23_v0.3_spec.md`, shipped: price books (C1), per-vendor cost on every
basis a book covers (C2), the dev/business switch (C3), the owner-then-client map with
active time (K1, K2), the business-mode client view (K3), export and merge (K4, K5),
GitHub Copilot in VS Code as a real adapter (A4), macOS and Linux browser-mode builds
(B1), and the Now page fixes, loading states and monitor mode (U1 to U5). Release
candidate Done-when and VERIFY pass: `SESSION_LOG.md`'s newest entry, all seven Done-when
items pass against the built exe and the real local store, no fail carried into the tag.
v0.2.3 (real-window check harness), v0.2.2, v0.2.1 and v0.2.0 shipped before it. Specs:
`02_roadmap\2026-09-22_v0.2_spec.md` through `02_roadmap\2026-09-23_v0.3_spec.md`.

## What it is

BurnMon is the successor of claudecost, a vendor-agnostic token and cost monitor for
your own AI coding usage: a portable Windows app (`burnmon.exe`) plus a single-file CLI
(`burnmon-cli.exe`), reading local session transcripts directly, no account or API key.
It is the token-cost slot in ZeroNonsense.dev's Groundwork Kit, per README's own section.

## Adapters

- **Claude** (`internal\adapter\claude`): Claude Code CLI, Cowork/desktop agent-mode
  sessions.
- **Codex** (`internal\adapter\codex`): Codex CLI/Desktop rollout transcripts, native and
  WSL roots.
- **Hermes** (`internal\adapter\hermes`, A1): local SQLite `messages`, WAL, 5-second poll.
  Default macOS/Linux path fixed this session: `~/.hermes/state.db` (or `$HERMES_HOME`),
  confirmed live against Hermes's own docs, replacing V3-5's guessed
  Application Support/XDG paths, which were wrong.
- **Copilot CLI** (`internal\adapter\copilotcli`, A2): session totals read at shutdown,
  not live.
- **Copilot in VS Code** (`internal\adapter\copilotvsc`, A4, v0.3): tails the OTel file
  the two VS Code settings produce, 5-second poll, last write wins per request. No
  workspace/project attribute exists in the real span data seen so far, so its events
  show client "unassigned" until GitHub adds one.

## Pages

Five tabs, deep-linkable: **Now** (default start page), **History**, **Sessions**,
**Tools**, **About**. English only.

- **Now**: every running session, live, polled every 2 seconds; a smoothed burn chart
  (per-session lines, cost line toggle, euros in business mode); the vendor strip; the
  turn ticker with finding markers; the forecast chart (tokens in dev, euros in
  business). Loading states throughout (U2): every visual shows "Loading..." until its
  first real data, never the saved-report fallback text while the app's bindings are
  still being injected.
- **History**: filters (period, range, vendor, owner, client once client rules exist);
  business mode adds a per-client table (tokens, headline cost, active time, sessions).
- **Sessions**: sortable/searchable table, Findings column, owner and client columns
  once configured.
- **Tools**: `tool_calls` totals as a chart and table.
- **About**: the Claude pricing explainer, plus "what this shows and what it cannot",
  plus the macOS/Linux untested note.
- **Monitor mode** (U3, U4, U5): a second Now view, chart/sessions/vendor strip only, no
  chrome. Dev mode renders it as real monospace text with a braille burn chart (U5,
  perfadvisor-style); business mode shows the normal visuals with euros. Remembered in
  `burnmon.json`'s `"view"` key.

## Store

Versioned additive migrations. Schema version 7 at this commit (migration 7, v0.3 K1,
adds `client` on `events`; the spec's own text calls it "migration 6", stale against
`migrations.go`'s real `Version: 6`, which is the v0.2.1 session_id index). A v0.1 store
opens under this code with no event lost (`TestMigrateRealV01Store`, run against the real
store this session, schema version 7 confirmed, all events retained). `tool_calls` table
fed by the Claude and Codex adapters. `burnmon-cli live` and the app's Now page share the
same windowed query and `SessionTotals` path.

## Cost (C1, C2, C3)

Dated JSON price books built into the binary (`internal\pricing\books`), each with a
`date` and `source`, overridable per model from `burnmon.json`. `CostForEvents` (one Go
function, shared by every page and export) returns every basis a book covers plus the
headline basis per vendor: credits for GitHub Copilot, subscription share once
configured, else API list labelled "upper bound"; a vendor with no book (Hermes) is
"tokens only" everywhere, never a guessed figure. The header's Dev/Business toggle flips
every number on the Now page and switches on History's per-client table; `"mode"` in
`burnmon.json` sets the start state.

## Owner and client map, active time (K1, K2)

`burnmon.json`'s `owners` table now carries optional `client` and `remote` per rule,
alongside the v0.2 `match`/`owner`. Match order per session: a `remote` rule (matched
against the project's git origin, read straight from `.git\config`, no git binary, worktree-
and submodule-aware) first, then a `match` path rule, else owner `"personal"`/client
`"unassigned"`. A v0.2 `burnmon.json` (`owners` only) loads unchanged
(`TestLoadV02ConfigWithOwnersOnlyIsUnchanged`). `burnmon-cli reown` re-applies both to
every stored event. Active time: the sum of gaps between a session's consecutive turns,
any gap over `active_idle_minutes` (default 10) counting zero; per-client active time
sums its sessions'. Spot-checked against three real sessions (V3-2); a proper comparison
against an external time log remains open, see Known gaps.

## Export and merge (K4, K5)

`burnmon-cli export` writes daily rows (day, owner, client, vendor, model, five token
classes, cost on every basis, active minutes); no paths, session ids, prompts or project
names, enforced by a test that scans the output bytes for a path separator or a
session-id pattern. `--owner` is required whenever `owners` rules exist. Verified for
real this session: an export with `--owner ZND` against the real local store held zero
"Valona" occurrences, zero path separators, zero session-id patterns (35 rows). `burnmon-
cli merge` combines any number of exports (no cap) into `merged.json` and a `report.html`
that opens offline (zero `<script>` tags), one column per label, totals by client, vendor
and week; verified for real by merging two exports of the real store under different
labels.

## Platforms (B1)

`burnmon`/`burnmon-cli` cross-compile clean for darwin and linux, amd64 and arm64,
`CGO_ENABLED=0` (verified this session against the current tree). Off Windows, the app
collects once, writes `dashboard.html`, opens it in the OS default browser, then keeps
re-collecting on `-interval` in the background; no bound-JS live path exists there
(WebView2 is Windows-only), so a page there falls back to the same "only available in
the app window" text a saved report already shows. README, About and `STATUS.md` all say
untested: built and cross-compiled, never run on real macOS or Linux hardware.
`.github\workflows\release.yml` builds and attaches all four binaries to a pushed `v*`
tag; this only runs once the tag is actually pushed, see Known gaps.

## Insight (I1, I2, I3)

`internal\insight`, computed on the fly, no store writes. Four finding kinds:
`re-prefill`, `compaction`, `context-runway`, `expensive-turn`. Shown as markers on the
Now chart, lines in the turn ticker, the spike drawer, and per-session on the Sessions
tab.

## Forecast (F1)

Plan line (last four weeks, weekday-aware) and live line (current rate to end of day/
month), error band once a week is scored, gate text until then. Tokens in dev mode;
business mode shows the same forecast in USD/euros on the headline cost basis (v0.3 C3).

## Config

`burnmon.json` is the current name; `claudecost.json` in the same two locations is still
read as a fallback. `burnmon.example.json` documents every key including `owners` (with
one commented client/remote example), `mode`, `view`, `copilot_plan` and
`copilot_vscode_otel_file`.

## Known gaps at this commit

- `internal\report\template.html`'s Now chart cost-axis toggle (dev mode, full view):
  the right-hand cost axis stays hidden after the cost series is shown through the chart
  legend (`scripts\uicheck.ps1 w8`, reproduced live this session against the current
  build). Confirmed pre-existing, not caused by any v0.3 session (V3-3c already isolated
  this against a pre-U5 build with the identical failure); not fixed this session, out of
  scope for a release-candidate pass, needs a real Chart.js debugging session.
- GitHub Copilot Business and Copilot Enterprise: their AI-credit allotments are now
  confirmed live (1,900/user/month and 3,900/user/month respectively, pooled at the
  billing entity, `docs.github.com/en/copilot/concepts/billing-and-usage/organizations-
  and-enterprises/billing`, checked 2026-09-24), but the plans page's "for businesses"
  tab still does not render in a plain page fetch, so no per-seat monthly price is
  confirmed for either, and neither is wired into `copilot_credits.json`'s `plans` block
  (their pooled, org-level billing also does not fit `CopilotCreditsLeft`'s
  single-machine model without further design work).
- A ChatGPT/Codex plan-credit table comparable to GitHub's AI Credits does not exist:
  confirmed live 2026-09-24 (`help.openai.com`'s own credits article) that ChatGPT
  personal plans use their own usage limits plus an optional pay-as-you-go credit
  top-up for overage, not a fixed monthly allotment; nothing to build a book from.
- Plan assumption A8 (active time usable): the spot-check from V3-2 stands, but
  `C:\ZND\10_holding\03_logs\time\` still does not exist on this laptop (checked again
  this session), so the comparison against an external time log remains unverified.
- macOS and Linux release artefacts: the workflow and the cross-compilation both verify
  clean, but no artefact has actually landed on a GitHub release yet, since that only
  happens once the `v0.3.0` tag is pushed; this session tags locally and stops before
  pushing, per house rules.
- The Now chart's per-vendor colour families define a lighter "desktop"/"VS Code" shade
  for Codex and Copilot, but Codex is always agent "codex" and Copilot in VS Code sets
  no distinct agent value either, so those two shades stay unreachable.
- `internal\report\template.html`'s About section still describes only Claude's own
  seat/allowance model, not the other vendors.

## Next

Decision 2026-12-19: continue to a paid team line, keep free, or stop. Out of scope,
named in the v0.3 spec's section 3: a local OTLP receiver, a merge cap and paid team
line, the BurnRate time-log join, a native window off Windows, signed builds,
project-file tags, a team server. Plan: `02_roadmap\roadmap.md`.
