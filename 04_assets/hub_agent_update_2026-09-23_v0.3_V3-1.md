# Hub Agent Update - ZeroNonsense.dev

**Date:** 2026-09-23 (v0.3 session V3-1) - **Owner:** Wilco de Tree
**Project:** BurnMon
**Purpose:** tell the hub that BurnMon v0.3's first build session (price books, cost
function) is code-complete and committed locally, not yet tagged or pushed.
**Read order:** this file, then `C:\ZND\projects\burnmon\SESSION_LOG.md`'s top entry for
the full technical trail.
**Supersedes:** nothing.

## 1. Headline
BurnMon v0.3 session V3-1 (C1 price books, C2 cost function) is committed locally on
`main`. Three dated JSON price books now live in the binary, keyed by exact model id, and
one new Go function prices any set of events on every basis its vendor's book covers. No UI
work: that starts V3-3.

## 2. What changed on disk
- **Committed** in `C:\ZND\projects\burnmon` (git, `main`): `5fe54f6` "v0.3 V3-1: dated
  price books (C1), CostForEvents (C2)".
- **On a branch, not merged:** nothing; this is a direct commit on `main`, not pushed.
- **Files touched** (full paths):
  - `C:\ZND\projects\burnmon\internal\pricing\books.go` (new)
  - `C:\ZND\projects\burnmon\internal\pricing\books\anthropic_api.json` (new)
  - `C:\ZND\projects\burnmon\internal\pricing\books\openai_api.json` (new)
  - `C:\ZND\projects\burnmon\internal\pricing\books\copilot_credits.json` (new)
  - `C:\ZND\projects\burnmon\internal\pricing\cost.go` (new)
  - `C:\ZND\projects\burnmon\internal\pricing\cost_test.go` (new)
  - `C:\ZND\projects\burnmon\internal\pricing\books_test.go` (new)
  - `C:\ZND\projects\burnmon\internal\pricing\pricing.go` (edited: new Config fields,
    `Subscription.Source`)
  - `C:\ZND\projects\burnmon\cmd\burnmon-cli\main.go` (edited: `price-check` prints the
    new books)
  - `C:\ZND\projects\burnmon\burnmon.example.json` (edited: documents the new override
    block)
  - `C:\ZND\projects\burnmon\SESSION_LOG.md` (edited: session paragraph prepended)
  - `C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_session_prompts.md` (edited: V3-1
    ticked)
- **Untracked / outside a repo:** nothing.

## 3. What did NOT happen (and why)
Not tagged, not pushed: the session prompt rules say stop before tagging (this session is
not the alpha/beta/release tag point anyway; `v0.3.0-alpha.1` is due after V3-2). No UI
change of any kind: the spec explicitly scoped V3-1 to price books and the cost function
only, with "cost on every page" left for V3-3. The old family-generic price table
(`Config.Prices`, `CallCostUSD`, `CallCostSubUSD`) was left untouched and still backs the
v0.2 UI paths (Now/History/Sessions/dataset) unchanged; nothing there was rewired to the
new per-model books yet, that rewiring is also V3-3's job. No spot-check against a real
invoice was possible (Subscription stays at Defaults' illustrative example this session, as
before). Copilot Business/Enterprise plan-credit numbers and any ChatGPT/Codex credit table
could not be confirmed from a primary source today and were deliberately left out rather
than guessed.

## 4. Findings worth propagating
- **[RESULT]** Anthropic now prices Claude Opus 5.5 apart from Claude Opus 5: $4/$20 per
  MTok in/out for 5.5, vs $5/$25 for 5, each with its own cache-read multiplier (Opus 5.5:
  0.05x; Claude Fable 5.1: 0.025x; everyone else: the standard 0.1x). Source:
  `platform.claude.com/docs/en/about-claude/pricing`, live-fetched 2026-09-23. This is why
  the new price books are keyed by exact model id rather than the old family bucket
  ("opus" could not represent two different current prices at once).
- **[RESULT]** GitHub Copilot's individual-plan AI credit allotments, confirmed live from
  `github.com/features/copilot/plans` 2026-09-23: Free $0/0 credits, Pro $10/mo for $15 of
  monthly credits, Pro+ $39/mo for $70, Max $100/mo for $200 (1 credit = $0.01).
- **[STATE]** VERIFY, unresolved, listed in `SESSION_LOG.md`'s top entry: a ChatGPT or
  Codex plan-credit table; GitHub Copilot Business/Enterprise seat credit allotments (the
  page's "for businesses" tab did not render on a plain fetch); `gpt-5.6-terra` and
  `gpt-5.6-luna`, no longer listed on OpenAI's own current pricing page.
- **[RESULT]** `go test ./... -count=1` and `.\build.ps1` both green after the change,
  confirmed this session, not a prior run.

## 5. Hub-level decision (if any)
Nothing. This is a project-internal build step, not a cross-project, pricing/positioning,
park/unpark or CIPHER-wall call.

## 6. What the next hub read should update
`C:\ZND\10_holding\01_projects\burnmon.md` (project one-pager) can note "v0.3 build under
way, V3-1 of 7 done" once the hub next reads BurnMon. No `DEADLINES.md` or `roadmap.md`
change is due yet: the v0.3.0 date (Fri 2026-10-09) and the alpha/beta tag points are
unchanged by this session.

## 7. Open flags for next session
V3-2 (client map K1, active time K2) is next, unrelated to this session's files. The three
VERIFY items above (section 4) stay open until a later session confirms them or the v0.3
spec's own VERIFY-carried list (section 6) closes them at V3-6.

## 8. Related files
`C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_spec.md` (sections 1, 2.1 C1/C2),
`C:\ZND\projects\burnmon\02_roadmap\2026-09-23_v0.3_session_prompts.md` (V3-1 prompt, now
ticked), `C:\ZND\projects\burnmon\04_assets\2026-09-22_token_monitor_architecture.md`
(section 4.3, the pricing-basis decision this session implements), `C:\ZND\projects\burnmon\SESSION_LOG.md`.
