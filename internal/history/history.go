// Package history builds the History tab's payload (v0.2 P2): tokens by
// class, sessions, turns and (where the price book covers the vendor) cost,
// bucketed by day, week or month, for one filter (period, date range,
// vendor, owner). The page never aggregates raw events itself; it calls the
// bound bmHistory(filter) function, which runs Build against the store's
// events.
package history

import (
	"fmt"
	"sort"
	"time"

	"burnmon/internal/pricing"
	"burnmon/internal/schema"
)

// AgentLabel maps a schema.Event.Agent value to its display name, the same
// five names P2's vendor filter lists.
var AgentLabel = map[string]string{
	"claude-code": "Claude Code",
	"cowork":      "Cowork",
	"codex":       "Codex",
	"hermes":      "Hermes",
	"copilot-cli": "Copilot CLI",
}

// coveredVendor reports whether vendor (schema.Event.Vendor, not Agent) has
// list prices in the price book at all: "anthropic" (Claude Code, Cowork)
// and "openai" (Codex) do, "github" (Copilot CLI) and "nous" (Hermes) do
// not, per v0.2 scope (per-vendor cost is v0.3). Decided in the P2 grill:
// the totals block only ever shows a cost figure when the vendor filter is
// narrowed to one covered vendor; "All" always shows tokens only.
func coveredVendor(vendor string) bool {
	return vendor == "anthropic" || vendor == "openai"
}

// Filter is what the History page's controls send bmHistory. From and To are
// "YYYY-MM-DD", inclusive on both ends. Vendor is an agent key (empty means
// every agent); Owner is empty for "all owners" or when no owner rules are
// configured.
type Filter struct {
	Period string `json:"period"` // "day", "week" or "month"
	From   string `json:"from"`
	To     string `json:"to"`
	Vendor string `json:"vendor"`
	Owner  string `json:"owner"`
}

// Totals is one bucket's (or the whole filter's) rolled-up numbers.
type Totals struct {
	Fresh    int64 `json:"fresh"`
	CacheW   int64 `json:"cache_w"`
	CacheR   int64 `json:"cache_r"`
	Out      int64 `json:"out"`
	Tokens   int64 `json:"tokens"`
	Sessions int64 `json:"sessions"`
	Turns    int64 `json:"turns"`
	// CostUSD is nil unless Filter.Vendor names one covered vendor; see
	// coveredVendor. CostNote carries the "tokens only until v0.3" text in
	// that case, for the page to show in place of a cost figure.
	CostUSD  *float64 `json:"cost_usd,omitempty"`
	CostNote string   `json:"cost_note,omitempty"`
}

// Row is one period bucket's Totals, keyed the same way agg.Bucket's maps
// are (month "YYYY-MM", week "YYYY-Www", day "YYYY-MM-DD"), so the page can
// reuse its existing monthLabel/weekLabel/dayWithName formatters.
type Row struct {
	Key string `json:"key"`
	Totals
}

// Payload is bmHistory's full return value.
type Payload struct {
	Filter  Filter   `json:"filter"`
	Totals  Totals   `json:"totals"`
	Rows    []Row    `json:"rows"`
	Vendors []string `json:"vendors"` // every agent seen in the store, for the filter's dropdown
	Owners  []string `json:"owners"`  // every owner seen, empty when no owner rules are configured
}

func bucketKey(period string, d time.Time) string {
	switch period {
	case "month":
		return d.Format("2006-01")
	case "week":
		y, w := d.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", y, w) // matches agg.weekKey's shape
	default: // "day"
		return d.Format("2006-01-02")
	}
}

// eventCostUSD is the full API list price of one event, the same formula
// buildSession uses (internal/dataset/fromstore.go), minus the subscription
// (cost_sub) side: History reports plain list-price cost only, gated on
// vendor coverage rather than blended with an uncalibrated figure.
func eventCostUSD(e schema.Event, cfg *pricing.Config) float64 {
	var cw, cr int64
	if e.CacheWrite != nil {
		cw = *e.CacheWrite
	}
	if e.CacheRead != nil {
		cr = *e.CacheRead
	}
	if e.Vendor == "openai" {
		cost, _ := cfg.OpenAICallCostUSD(e.Model, e.Input, cr, e.Output)
		return cost
	}
	fam := cfg.ModelFamily(e.Model)
	return cfg.CallCostUSD(fam, e.Input, cw, cr, e.Output)
}

// isTurn reports whether e is a real call rather than the claude adapter's
// synthetic tool-only event (model and every token count zero), matching
// buildSession's own callsN filter.
func isTurn(e schema.Event) bool {
	return !(e.Model == "" && e.Input == 0 && e.Output == 0)
}

func inRange(day, from, to string) bool {
	return day >= from && day <= to
}

// sessionKey identifies one session across vendors: a bare SessionID is not
// guaranteed unique between adapters (schema.Event's own doc comment).
func sessionKey(e schema.Event) string { return e.Vendor + "|" + e.SessionID }

// Build aggregates events into f's bucketed payload. events is normally the
// store's whole history (AllEvents): History is queried on demand, not
// polled, so loading everything and filtering here costs one pass over data
// that is at most a few hundred thousand rows (the same scale the CLI's
// one-shot report already handles).
func Build(events []schema.Event, cfg *pricing.Config, f Filter) Payload {
	if f.Period != "week" && f.Period != "month" {
		f.Period = "day"
	}

	vendorSeen := map[string]bool{}
	ownerSeen := map[string]bool{}
	buckets := map[string]*Totals{}
	bucketSessions := map[string]map[string]bool{}
	var totalCostUSD float64
	haveCost := f.Vendor != "" && coveredVendorForAgent(f.Vendor)
	var overallSessions = map[string]bool{}

	var overall Totals
	for _, e := range events {
		if e.Agent != "" {
			vendorSeen[e.Agent] = true
		}
		if e.Owner != "" {
			ownerSeen[e.Owner] = true
		}
		if f.Vendor != "" && e.Agent != f.Vendor {
			continue
		}
		if f.Owner != "" && e.Owner != f.Owner {
			continue
		}
		day := e.At.UTC().Format("2006-01-02")
		if f.From != "" && f.To != "" && !inRange(day, f.From, f.To) {
			continue
		}

		key := bucketKey(f.Period, e.At.UTC())
		b := buckets[key]
		if b == nil {
			b = &Totals{}
			buckets[key] = b
			bucketSessions[key] = map[string]bool{}
		}

		sk := sessionKey(e)
		bucketSessions[key][sk] = true
		overallSessions[sk] = true

		var cw, cr int64
		if e.CacheWrite != nil {
			cw = *e.CacheWrite
		}
		if e.CacheRead != nil {
			cr = *e.CacheRead
		}
		b.Fresh += e.Input
		b.CacheW += cw
		b.CacheR += cr
		b.Out += e.Output
		overall.Fresh += e.Input
		overall.CacheW += cw
		overall.CacheR += cr
		overall.Out += e.Output

		if isTurn(e) {
			b.Turns++
			overall.Turns++
			if haveCost {
				c := eventCostUSD(e, cfg)
				totalCostUSD += c
				if b.CostUSD == nil {
					z := 0.0
					b.CostUSD = &z
				}
				*b.CostUSD += c
			}
		}
	}

	for key, b := range buckets {
		b.Tokens = b.Fresh + b.CacheW + b.CacheR + b.Out
		b.Sessions = int64(len(bucketSessions[key]))
	}
	overall.Tokens = overall.Fresh + overall.CacheW + overall.CacheR + overall.Out
	overall.Sessions = int64(len(overallSessions))
	if haveCost {
		c := totalCostUSD
		overall.CostUSD = &c
	} else {
		overall.CostNote = "tokens only until v0.3"
	}
	for _, b := range buckets {
		if !haveCost {
			b.CostNote = "tokens only until v0.3"
		}
	}

	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([]Row, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, Row{Key: k, Totals: *buckets[k]})
	}

	vendors := make([]string, 0, len(vendorSeen))
	for v := range vendorSeen {
		vendors = append(vendors, v)
	}
	sort.Strings(vendors)
	owners := make([]string, 0, len(ownerSeen))
	for o := range ownerSeen {
		owners = append(owners, o)
	}
	sort.Strings(owners)

	return Payload{
		Filter:  f,
		Totals:  overall,
		Rows:    rows,
		Vendors: vendors,
		Owners:  owners,
	}
}

// coveredVendorForAgent reports whether agent's vendor has any price-book
// coverage at all (see coveredVendor); it maps the History filter's agent
// key to the vendor family that agent's events always carry.
func coveredVendorForAgent(agent string) bool {
	switch agent {
	case "claude-code", "cowork":
		return coveredVendor("anthropic")
	case "codex":
		return coveredVendor("openai")
	default: // "hermes", "copilot-cli", or an agent not yet in AgentLabel
		return false
	}
}
