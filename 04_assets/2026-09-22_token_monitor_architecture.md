---
type: analysis
title: Vendor-agnostic token and cost monitor, architecture note
created: 2026-09-22
tags: [claudecost, burnrate, token-monitor, product, grill]
status: decided 2026-09-22
---

# Vendor-agnostic token and cost monitor: architecture note, Part 1

Companion files: `_grill_state.md`, `2026-09-22_token_monitor_sources_and_facts.md` (all in
`C:\ZND\10_holding\04_assets\`). Marks: RELAYED, ASSUMED, VERIFY as defined in the grill skill.

## 0. TLDR

Build the successor of claudecost as one portable executable that reads the local session
trails of Claude Code, Codex, Copilot CLI and Hermes (and Cowork, which claudecost already
reads), normalises them into one five-class token schema in its own SQLite store, prices
them three ways (API-equivalent, plan credits, subscription share), and adds the two things
none of the crowded competitors sell: a month-end forecast from the developer's own
history, and per-project attribution. Local only, no account, no server, no cloud sync.
Windows first, macOS and Linux from the same Go codebase, WSL roots discovered from Windows.
Recommended: a new name, the claudecost internals carried over, claudecost itself retired
into it. Pilot: Wilco's laptop first, then the Valona Amsterdam team (ASSUMED, not agreed),
then Talon's five laptops through the existing Groundwork Kit slot. No dates or money are
fixed; the plan (phase 6) proposes them.

## 1. Decided so far (phase 0, not reopened)

- Kind: product. Owner: ZeroNonsense.dev. Public voice, MIT lineage of claudecost.
- Scope v1: per developer, local only. Teams and Talon later.
- Platforms: Windows, macOS, Linux, WSL.
- Agents in scope: Claude Code (CLI and VS Code), OpenAI Codex (CLI and VS Code), GitHub
  Copilot (VS Code and CLI), Hermes Agent. Cowork stays because claudecost already has it.
- Output language English. No Valona material into `C:\ZND`.

## 2. The two facts that shape the whole design

**Fact 1. Every agent leaves a per-session trail on disk, but no two look alike.**
(facts list sections A to D, 2026-09-22, primary docs for Claude, Codex, Hermes; Copilot
VERIFY.) Claude: JSONL with four token classes per assistant line, deleted after 30 days.
Codex: JSONL rollouts with cumulative `token_count` events and `rate_limits`, no cache-write
class, files up to 2 GB. Copilot CLI: SQLite plus a layout that changed three times this
year, synced to GitHub by default. Hermes: SQLite with per-model rows and recorded real
cost. This rules out a single parser and rules in an adapter per vendor behind one schema,
and it rules out reading from the source at query time: the monitor needs its own store
because Claude's trail evaporates and Codex's is too big to rescan.

**Fact 2. The field is crowded, and the leader already ships forecast and attribution.**
(facts list section H; external scan of CodeBurn, 2026-09-22, all HELD.) CodeBurn (11,148
stars, MIT, desktop v0.9.25 of 2026-09-21) reads 41 agents including all of ours, forecasts
the month total, attributes by project, branch and PR, shows quota rings, and has a signed
macOS app, a Windows Store build and a Linux package. Part 1 first claimed nobody sold
forecast and attribution; that was wrong and is corrected here (see Part 2, section 9).
What nobody sells, verified against CodeBurn, tokscale and tokenuse docs: a client or
engagement dimension, a link from tokens to billable hours, a durable archive that outlives
each tool's own log retention, zero telemetry of any kind, and a legal entity in the EU to
contract with. Coverage is a commodity; the money question per client is not.

## 3. The structure

One Go module, standard library plus two dependencies (SQLite driver, webview). Boundary
rule: adapters know vendors, everything downstream knows only the schema.

```
cmd/<name>          the app (one window) and the CLI (one file), same as claudecost
internal/adapter/   one package per vendor: claude, codex, copilot, hermes, cowork
                    each returns []Event in the common schema, nothing else
internal/schema/    Event: vendor, agent surface (cli|vscode|desktop), session id,
                    request id, timestamp, model, project path, five token classes
                    (input, cache_write, cache_read, output, reasoning; null allowed),
                    vendor-reported cost (Hermes only), plan-window sample (Codex only)
internal/store/     SQLite (pure Go driver), append-only events, ingest cursors per file
internal/pricing/   three price books: API list per vendor and model, plan credits
                    (Copilot table, ChatGPT credit table), subscription share (config)
internal/forecast/  month-end projection and plan-window burn-down from the store
internal/attrib/    project and client attribution rules (path prefix, git remote, tag)
internal/report/    the HTML dashboard, one template, Chart.js vendored inline
```

## 4. The hard choices

### 4.1 Relation to claudecost

| Option | Pros | Cons | Failure it invites |
|---|---|---|---|
| A. Evolve claudecost in place, keep the name | zero migration, Talon already has it | name says Claude, GitHub repo and Talon config are Claude-shaped | product is dismissed as a Claude tool by Codex and Copilot users |
| B. New name, fork claudecost internals, retire claudecost | keeps the working scan, dedup, pricing and app shell; honest name | one migration for Talon and Wilco; two repos for a while | half-finished migration leaves two tools alive |
| C. New tool from scratch, claudecost untouched | clean design | throws away tested WSL discovery, dedup and app recipe | months lost re-learning what claudecost already knows |

Recommendation: **B**. The claudecost adapter becomes one of five; the README's "Coverage is
Cowork and Claude Code only" line becomes the changelog entry that closes claudecost.

### 4.2 The differentiator

| Option | Pros | Cons | Failure it invites |
|---|---|---|---|
| A. Coverage parity plus Windows-native | fastest to ship | ccusage and tokscale already cover Windows paths | "why not ccusage" on day one |
| B. Forecast: month-end cost and window exhaustion from history | nobody sells it; fits BurnRate's findings and the talks track | needs 4 to 8 weeks of history per developer before it is credible | a forecast that is wrong once is never trusted again |
| C. Attribution: tokens per project or client | turns a curiosity into an invoiceable number; Talon and advisory need it | path-to-client mapping is manual; VS Code Copilot has no project path without OTel | attribution table nobody maintains |
| D. Quota windows (5h, weekly, credits) in a tray | what developers watch hourly | needs undocumented endpoints for Claude and Copilot; CodexBar owns it on macOS | breaks every time a vendor changes an internal endpoint |

Recommendation (revised after the CodeBurn scan, Part 2 section 9): **C first, B second**,
with A as the floor. C means client and engagement, not only project: a client map, hours
from the developer's time log, and an export that fits an invoice line or a PoC report. B
adds the forecast with its error band; CodeBurn has a forecast, ours must show its own
track record or stay hidden. D only where the vendor writes the data to disk (Codex
`rate_limits`), never via undocumented endpoints. The product sentence: "What did each
client cost you in tokens and hours this month, and what will it be by the 30th."

### 4.3 Pricing basis

| Option | Pros | Cons | Failure it invites |
|---|---|---|---|
| A. API-equivalent list price only (ccusage default) | one number, every competitor shows it | BurnRate Finding 6: up to 8.5x above what a Team plan actually pays | the monitor cries wolf and is ignored |
| B. Subscription share (claudecost model) | matches the invoice | needs config per org; meaningless for a solo Pro user | wrong config, wrong number, nobody notices |
| C. Plan credits where the vendor publishes a table (Copilot AI credits, ChatGPT credits) | exact for Copilot since 2026-06-01 | Anthropic publishes no such table for Pro/Max | mixed exactness across vendors confuses |
| D. All three, labelled, side by side | honest; each reader picks the one that matches their bill | busier dashboard | three numbers, no opinion |

Recommendation: **D with an opinion**: the headline is the one that matches the developer's
configured plan (credits for Copilot, subscription share when configured, API-equivalent
labelled "upper bound" otherwise). The other two sit one click away.

### 4.4 How the data gets in

| Option | Pros | Cons | Failure it invites |
|---|---|---|---|
| A. File and SQLite readers, polled every N minutes | works with zero developer configuration; what claudecost does | "live" means minutes, not seconds; Copilot VS Code invisible without OTel | a Copilot user sees nothing and uninstalls |
| B. Local OTLP receiver on localhost, developer enables OTel per tool | true live, per request, includes VS Code Copilot | each developer edits three configs; Hermes has no OTel | half the team never configures it |
| C. Hooks (Claude, Codex, Copilot) writing to the store | per-turn, cheap | three hook formats, Hermes none, hooks conflict with the developer's own hooks | silent hook overwrite |
| D. A plus B: readers by default, receiver as opt-in "live mode" | full coverage without config; live for those who want it | two ingest paths to keep consistent (dedup by request id) | double counting |

Recommendation: **D**, readers first (v1), receiver in v1.1. Dedup by vendor plus request id
on both paths, the claudecost rule (largest output wins) carried over.

### 4.5 Team and Talon aggregation

| Option | Pros | Cons | Failure it invites |
|---|---|---|---|
| A. Strictly personal, no export (claudecost design) | no personnel-data question | Talon PoC number must be hand-collected | manual spreadsheet |
| B. Developer-initiated export file (JSON, totals only, no prompts) | developer stays in control; Talon gets a number by mailing five files | someone still merges five files | export nobody runs |
| C. Shared folder or server aggregation | real team view | a server, an account, a privacy review; competitors with accounts are exactly what we avoid | scope creep into a SaaS |

Recommendation: **B** for v1, plus a `merge` CLI command that folds N export files into one
report. C is explicitly out until a paying team asks.

### 4.6 App shell

| Option | Pros | Cons |
|---|---|---|
| A. Go + `jchv/go-webview2` (claudecost today) | proven, one exe | Windows only |
| B. Go + `webview/webview_go` | Windows, macOS, Linux from one codebase; still one exe | needs cgo on macOS and Linux; WebKitGTK on Linux |
| C. Wails or Tauri | polished, signed installers | new toolchain; Tauri means Rust; heavier than the recipe |

Recommendation: **B** on macOS and Linux, keep A's WebView2 path on Windows behind a build tag,
browser fallback everywhere (claudecost already has it).

### 4.7 Storage

SQLite via a pure-Go driver (`modernc.org/sqlite`, no cgo) replaces the gob parse cache.
Reason: Claude deletes trails after 30 days and the forecast needs at least 8 weeks. Events
append-only, cursors per source file so a 2 GB Codex rollout is read once. Out: any
cloud store.

## 5. Sections of the design

**Forecast.** Two methods, both shown with their error band: month-to-date linear projection
weighted by weekday, and a plan-window burn-down (Codex writes `used_percent` and
`resets_at` to disk; for Claude and Copilot the window is derived from the developer's own
event rate against the plan's published allowance). Accuracy is measured against the next
month's actual and displayed; a forecast without its own track record is not shown.

**Attribution.** Rule order: explicit tag in a project's config file, then git remote, then
path prefix, then "unassigned". A client map lives in the monitor's config, never in the
transcript folders. VS Code Copilot gets a project only via OTel (v1.1). Hours join from the
BurnRate time log (`03_logs\time\YYYY-MM.ndjson`) by date and project, so a client row
shows tokens, API-equivalent cost and hours side by side (added in Part 2).

**Dashboard.** claudecost's page plus a vendor column, a forecast card, an attribution tab,
a plan-window card where data exists. Same refresh model: on start, every 15 minutes, on
demand.

**CLI.** `scan`, `report`, `export`, `merge`, `forecast`, `price-check` (prints the price book
with its dates so a stale table is visible).

**Distribution.** Portable exe per OS, unsigned on Windows (SmartScreen warning, as
claudecost), winget and Homebrew manifests later. Public MIT repo under github.com/wilcodetree.

## 6. What v1 does not do

No account, no cloud, no server, no telemetry back to us. No undocumented vendor endpoints.
No VS Code Copilot tokens unless the developer enables OTel. No org APIs (Anthropic Admin,
OpenAI Admin, GitHub metrics): admin-only and they skip subscriptions. No Gemini, Cursor,
OpenCode: adapters can come later, the schema allows it. No per-person comparison inside a
team. No price scraping: price books ship in the binary and in a JSON the developer can
override, dated.

## 7. What it costs, and what it does not

Costs: Wilco's build hours (ASSUMED 6 to 10 sessions to v1 given the claudecost base), one
laptop per OS for testing (macOS is the gap; ASSUMED none available), and a recurring
15-minute price-book check per vendor per month. Does not cost: infrastructure, licences,
signing certificates (deferred), any Valona budget (the Valona pilot uses the public build
on personal initiative; RELAYED nothing, confirm with the team lead).

## 8. Sources

Primary, 2026-09-22: code.claude.com/docs (claude-directory, costs, monitoring-usage, hooks,
statusline); developers.openai.com/codex (config-advanced, hooks, pricing);
github.com/openai/codex issues #20165, #21660, #24948; docs.github.com Copilot CLI reference,
Chronicle, billing; github/docs `copilot.yml` and `models-and-pricing.yml`;
github.com/NousResearch/hermes-agent; platform.claude.com pricing and usage-cost API; GitHub
API for competitor stars and releases. Internal: claudecost README and design doc, BurnRate
AGENTS.md and findings.md, portfolio.md, decisions.md, Talon PoC plan.
Secondary, VERIFY: Copilot CLI `data.db` token layout (tokenuse docs, ccusage issue #1174);
Hermes native Windows path; Claude Pro/Max plan prices (aggregators); Codex `.jsonl.zst`
compression; the non-interactive Claude credit pool (lowcode.agency).


# Part 2, after Wilco's remarks (2026-09-22)

## 9. Remark: "agree with all your recommendations", and one correction of my own

Wilco accepted every recommendation in section 4. Before recording that, one fact in Part 1
turned out wrong and the acceptance needs to survive it. Section 2, Fact 2, said no
competitor sells a forecast or attribution. The phase 4 scan of CodeBurn
(`2026-09-22_external_scan_codeburn.md`) shows it ships both, plus quota rings, a Windows
tray and a signed macOS app, all free and MIT. So 4.2 option B ("forecast") and option C as
written ("attribution by project") are table stakes, not differentiators.

What survives the scan, verified against CodeBurn, tokscale and tokenuse docs: none has a
client or engagement dimension, none links tokens to hours or an invoice, only tokenuse
keeps a durable archive, CodeBurn's desktop app phones home by default outside the EU, and
none is a legal entity you can contract with. That is exactly the ground ZeroNonsense.dev
already stands on: BurnRate logs time per node, Siteoffice is the company-as-a-site, Talon
needs a closing number per PoC, advisory work bills per client.

Revised 4.2 recommendation: C first, redefined as client and engagement attribution with
hours; B second, the forecast with its own error band and track record; A as the floor.
Section 2 and 4.2 in Part 1 are edited in place.

## 10. Remark (mid-turn): name candidates BurnMon, TokenMon, TokenControl

Quick collision check, 2026-09-22, via the GitHub search API, the npm and PyPI registries and
a plain HTTPS probe (000 means no answer from the sandbox, not proof of a free domain; VERIFY
with a registrar):

| Name | GitHub | npm / PyPI | .com | .dev / .app | Note |
|---|---|---|---|---|---|
| BurnMon | 6 unrelated repos, max 3 stars | free / free | 000 | 000 / 000 | fits the BurnRate family |
| TokenMon | 88 hits incl. tokenmonster (627 stars) | free / free | 200, taken | 000 / 000 | reads as "token monster" |
| TokenControl | 8 unrelated repos | free / free | 301, taken | 000 / 000 | promises control it does not have |

My pick: BurnMon. It sits next to BurnRate, says "burn" (the thing developers feel) and
"monitor" (what it does), collides with nothing, and the .com answered nothing. TokenMon
shares a stem with a 627-star tokenizer. TokenControl claims a verb the tool does not do;
it watches, it does not control.

## What changed in Part 1 because of Part 2

- Section 2, Fact 2 rewritten: CodeBurn ships forecast and attribution; the gap is client,
  hours, archive, zero telemetry, legal entity.
- Section 4.2 recommendation rewritten: C (client and engagement attribution with hours)
  first, B (forecast with track record) second, A floor.
- Section 5, Attribution: gains the client map and the hours join (BurnRate time log now,
  Siteoffice later). Applied below in section 5 as a one-line addition.

## Decisions, taken 2026-09-22

1. Differentiator: all three in v1, in this order: coverage parity Windows-first as the
   floor, then the forecast with its error band, then client attribution. (Wilco's order;
   my pick had client first.)
2. Name: BurnMon. Domain availability VERIFY at a registrar.
3. Hours in v1: session duration from the transcripts, labelled "active time", never
   "billable". The BurnRate time-log join moves to v1.1.
4. Valona pilot: the Amsterdam team from the first build. Needs a team-lead yes; ASSUMED
   until then. Nothing leaves a laptop; the build is the public MIT one.
5. macOS: ship untested from a GitHub Actions macOS runner, README says so, until a Mac user
   in the pilot reports.
6. Licence and money: MIT core; `merge` and client reports become the paid team line when a
   team asks. Phase L note: `2026-09-22_burnmon_licence_and_ip.md`.
7. Roadmap slot: start now, one session a week beside Siteoffice. v0.1 target 2026-10-17.
