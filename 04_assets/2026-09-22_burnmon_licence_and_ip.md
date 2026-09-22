---
type: analysis
title: BurnMon, licence and IP pass
created: 2026-09-22
tags: [burnmon, licence, ip, mit]
status: decided 2026-09-22
---

# BurnMon: licence and IP pass

Decision 6 of the grill: MIT core, paid team line later. This note says what that means in
practice and what is deliberately not done.

## The honest boundary

Anyone can copy everything in the MIT core, adapters included, and CodeBurn or ccusage can
merge our adapters the day they appear. That is accepted: the adapters are commodity (five
competitors have them), and the parsing knowledge is public in vendor docs. What a copier
cannot take: the client map and hours join as a working habit inside a firm, the written
opinion behind the numbers (talks, posts, advice), the EU entity behind the tool, and the
track record of the forecast on a specific team's data. The product is the opinion; the
exe is the evidence.

## Four layers

**Distribution.** Public MIT repo `github.com/wilcodetree/burnmon`, portable exe per OS, no
installer, no signing in v1. Copyright line "(c) 2026 Wilco de Tree", as claudecost. The
name BurnMon is not trademarked; registering the domain (A6, VERIFY) is the only reservation.

**Containment.** The paid team line, when it exists, is a separate module (`merge` with more
than N inputs, client reports, support). It ships as a separate binary or a licence file
the core checks locally, never a server call. Until a firm asks, nothing is contained:
`merge` is MIT in v0.3 and counted as a giveaway that feeds H3.

**Detection.** None in v1. A team running `merge` on 50 exports without paying is a lead,
not a breach. The only counter is the count of inputs to `merge`, computed locally from
data the team already has; over the cap the report prints a line with the price and
continues. No phone-home, no kill switch: that would contradict the zero-telemetry claim
that is half the pitch.

**Contract.** Individuals: MIT, no contract. Teams later: a one-page order form under Dutch
law from ZeroNonsense.dev (eenmanszaak), price per firm per month like Siteoffice
(Starter EUR 49, Team EUR 149, Company from EUR 490 are the existing anchors; BurnMon's
own price is OPEN: waits on the first ask, decision date 2026-12-19). Support: one thread,
no SLA. Data: stays on the customer's laptops; an export is the customer's file.

## How caps are counted

From data the customer already has: the number of export files fed to one `merge` run and
the number of distinct developer ids inside them. Nothing else is measured. No usage of the
core is ever counted.

## Over the cap

The report is still produced, with one printed line: the cap, the count, the price, the
address. Nothing is withheld and nothing is degraded. A second run prints the same line.

## Deliberately not done

No CLA (contributors keep their copyright, MIT covers the grant). No dual licence. No
"source-available" clause. No trademark filing before revenue. No telemetry, no licence
server, no online activation. No contributor agreement with Valona: the pilot uses the
public build and Valona contributes nothing but feedback, so no Valona IP enters the repo.
Anything a pilot developer proposes as code goes through a public PR under MIT, never
through a Valona channel (wall `valona`).

## Open calls (asked in phase 5 or carried)

- BurnMon team price: OPEN, waits on the first firm asking, decided by 2026-12-19.
- Domain: VERIFY at a registrar this week (A6).
- Whether the Talon Groundwork Kit counts as "free tier" or as a paid Siteoffice inclusion:
  ASSUMED free, as claudecost is today; confirm when the Kit is re-issued at v0.3.
