# Station UI pass and backlog, from alpha.10 (2026-10-03)

Source: Wilco's list of 2026-10-03 with four screenshots. Order chosen by Wilco: Station UI
first. Never use em dashes. Views: P (full Station) and O (plan view). Themes: space and site.
Every UI item must hold in all four combinations (P space, P site, O space, O site).

## Decision recorded

BurnMon has no Valona pilot. It is ZND only; developers use it through the public ZND GitHub.
DEADLINES.md line for 2026-10-24 is removed (done 2026-10-03). The decision itself was already in
`C:\ZND\10_holding\03_logs\decisions.md` (2026-09-29), so no second entry was added. Done
2026-10-03: the hub one-pager's two stale Valona lines corrected, and in `2026-09-22_burnmon_plan.md`
the Valona rows struck, assumption A1 marked KILLED, and H1's measure marked OPEN (Wilco sets it at
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
