# Station UI pass and backlog, from alpha.10 (2026-10-03)

Source: Wilco's list of 2026-10-03 with four screenshots. Order chosen by Wilco: Station UI
first. Never use em dashes. Views: P (full Station) and O (plan view). Themes: space and site.
Every UI item must hold in all four combinations (P space, P site, O space, O site).

## Decision recorded

BurnMon has no ClientA pilot. It is ZND only; developers use it through the public ZND GitHub.
DEADLINES.md line for 2026-10-24 is removed (done 2026-10-03). The decision itself was already in
`C:\ZND\10_holding\03_logs\decisions.md` (2026-09-29), so no second entry was added. Done
2026-10-03: the hub one-pager's two stale ClientA lines corrected, and in `2026-09-22_burnmon_plan.md`
the ClientA rows struck, assumption A1 marked KILLED, and H1's measure marked OPEN (Wilco sets it at
the 2026-12-19 scoring).

## Step 1: Station UI (five items)

1. Room names: centred at the bottom of every room; Lounge and Core further right (their bottom
   centre is covered by agents and the tok/min label). Source: `THEME_DEF`, label drawing in
   `cmd\burnmon-dev\station\station.js`, plus the O view's own labels.
2. One robot or tractor only: delete the small orange box beside the Lounge agent in both
   views and themes (Wilco's answer, 2026-10-03). Find its source first: it is not in the Lounge
   `props` list, so it is probably a per-agent sprite. Do not guess; locate, then remove.
3. Uplink: add a chair at the middle table (the large dish at tile 3,2). Proposed tile 3,4 with a
   `desk_chair` sprite, blocks 0, keep it off the slots [2,3] and [4,3]. Site theme needs its own
   mapping in `SITE_MAP`; `check_themes.js` must still pass (one shared blocked grid).
4. Plan table room: explain the objects on top of the table. Read from `TEMPLATES.plan`: the
   table is nine `platform_low_SE` tiles at 2..4,2..4; the "L" shapes along its top edge are
   inferred to be rail pieces of that sprite (UNVERIFIED, check in the atlas). Report back, then
   Wilco decides keep, hide or replace.
5. Site theme builder cannot sit like the space astronaut: add a seated pose (or a sprite swap
   to a seated worker) for chair slots, in P and O. Needs a sprite; check CC0 kit options first.

Done when: uicheck d23 and d24 pass, `check_themes.js` passes, four screenshots (P and O, space
and site) show items 1 to 3 and 5; item 4 answered in writing.

## Step 2: bugs

- Station CPU climb (cause open, STATUS.md alpha.10 "Known, not fixed"). Measure in 5-minute
  windows with the Station open; the climb is in Go and WebView2.
- uicheck d19 and d20 fail on the single 1600x1000 screen (To Do fit-dropped, process labels
  0 px wide).

## Step 3: parked items and known gaps

To Do due dates live check; d9 at 1920x1080; solo w1 re-run; watcher buffer overflow triggers an
immediate rescan; History whole-table load (about 3 s per filter change); `at` bounds compare
text, fix via `julianday()` or fixed-width binding; legacy `forecast_scores` UTC week start; w8
chart-axis flake (re-run, then close or reopen); About text covers only Claude pricing; Copilot
Business and Enterprise prices; remove or use `copilot_credits_left` and `business_cost`;
History client table live click-through. Parked list: `2026-09-26_parked_after_alpha2.md`.

## Step 4: plan-limits panel

No brief yet. Idea source github.com/Javis603/token-monitor (MIT). Write the brief first.

## Mechanics

No git writes on this mount. A Windows build, uicheck and the CPU measurements need a Windows
session (Claude Code on the laptop). Commits, tags and pushes are Wilco's.

## Added 2026-10-05: Copilot in VS Code always shows Compacting (bug)

Reported by Wilco with screenshots: a running GitHub Copilot session sits in the Recycler,
stage Compacting, all the time. Turn 60 in the drawer shows fresh 253, cache read 0, output 74
on gpt-4o-mini, a tiny request.
Likely cause (INFERRED from code, not yet proven on real data): `internal\insight\insight.go`
fires a compaction finding when `contextOf` (input plus cache read plus cache write) drops more
than 30 percent from the previous turn. Copilot's OTel spans are one row per request, cache
fields are empty, and small side requests follow large ones, so the rule fires on almost every
turn. `internal\live\live.go` line 753 then takes the newest such finding as `CompactedAt`, and
`internal\stage\stage.go` line 88 returns Compacting while it is inside CompactWindow.
Fix: failing test first with a Copilot-shaped event sequence; then restrict the rule (same model
as the previous turn, and a vendor whose context is a running total), and re-check Claude Code
and Codex compaction findings still fire. Prove on the real store before and after.

## Added 2026-10-05, from Wilco's screenshot 12

A. Click popup too big (Wilco, screenshots 13 and 14). The hover card (screenshot 14) is the
   right size. The popup you get on clicking an agent for "the latest turn" (screenshot 13)
   spans the full window width, runs a long Cowork cwd path on one line, and is clipped on the
   left. Make it the same compact card as the hover card: fixed max width near the agent, long
   values wrap (overflow-wrap:anywhere), nothing clipped. In P, O and the new I view, both
   themes. Test first if the card geometry can be checked headless; add a uicheck assertion.
B. New key I: replaces the whole burn zone with the 3D (isometric) Station, the same way O
   replaces it with the plan view. Esc closes it (after a turn popup, before fullscreen, same
   order as O). Decided key: I (Wilco, 2026-10-05). No README mention while the Station stays
   undocumented. Add a d-check to uicheck for it, both themes.
C. Cowork session shows Waiting while it is busy (Wilco: "this session is busy, but something
   else"). INFERRED, unproven: Cowork writes events at turn end, so a long turn reads as
   Waiting. Same family as the Copilot Waiting symptom. Prove on a copy of the store first.

Status 2026-10-05 (uncommitted): A done (`d25`). B done (`d23`; Esc now closes O too, it did not
before). C disproved on the store: Cowork rows land about 0.2 s after their stamp, and long gaps after
a tool call are as rare as in Claude Code (0.52 against 0.79 percent over 180 s). The one live case
Wilco saw is not reproduced: send the time or a screenshot to look at that session.

D. Space theme, P view: the blue "engine glow" under the deck has a hard straight top edge
   (Wilco, screenshot 15). Cause, read from code (INFERRED, not yet seen live):
   `cmd\burnmon-dev\station\station.js` lines 577 to 579 draw a radial gradient of radius
   260*s centred at B[1] + d*2, but fill only `fillRect(B[0]-300*s, B[1], 600*s, 320*s)`. The
   rect starts at the deck's bottom corner, so the upper part of the glow is clipped flat.
   Fix: fill a square that covers the whole gradient, `fillRect(cx - r, cy - r, 2*r, 2*r)` with
   cx = B[0], cy = B[1] + d*2, r = 260*s. Space theme only; site theme has no glow.
   Check at several zoom levels and pan positions in P, and that O and the site theme are
   unchanged.

E. Site theme sky: clouds should look like clouds (Wilco, screenshot 16). Cause, read from code
   (INFERRED, not yet seen live): `buildSky` in `cmd\burnmon-dev\station\station.js` (lines 492
   to 496) draws each cloud as four flat ellipses at 50 percent white in a row, so the overlaps
   double up and read as stacked discs. Fix: draw each cloud on its own small offscreen canvas as
   a cumulus (5 to 8 round puffs of varied radius in a dome, flat base, soft radial-gradient
   edges, white top and a pale grey-blue underside), then composite it onto the sky at one
   alpha so overlaps never show. Keep the seeded `rng(5)` so the sky stays stable, keep the
   sun glow, and keep the sky baked once (no per-frame cost). Site theme only; check the light
   page theme and a resized window.

## Decisions 2026-10-05 (Wilco)

- Esc closes every Station mode, O included: KEPT.
- H1 measure without ClientA: Wilco's own daily use plus public GitHub signals (stars, release
  downloads, issues on the public ZND repo, counted over a fixed window before the 2026-12-19
  decision). No direct developer outreach. Still to do: write this into the H1 row of
  `2026-09-22_burnmon_plan.md` and set the window dates (OPEN: window start and the thresholds
  that count as a yes, waits on Wilco).

F. Hover card too wide and jumps left (Wilco, screenshots 17 to 20, Cowork agent). The long Cowork
   cwd path makes the hover card as wide as the window, and a small mouse move shifts it left.
   Cause, read from code (INFERRED, not yet seen live): `showTip` in `cmd\burnmon-dev\station\
   station.js` (about line 1752) sets display:block, reads `tip.offsetWidth`, then sets left to
   `clamp(mx + 14, 4, W - tw - 4)`. The card is absolutely positioned, so its shrink-to-fit width
   depends on the left it had last time; an unbreakable path then flips between widths as the
   cursor moves. Fix: give the hover card a max width (about 320 px, the same as the click card),
   wrap long values (overflow-wrap:anywhere), and reset left to 0 before measuring so the width
   no longer depends on the last position. Cover Project, Model, Tool and any other long field.
   Test: move the pointer a few px across an agent with a 200 character project and assert the
   card width stays constant and inside the window (uicheck, both themes, P and O and I).
