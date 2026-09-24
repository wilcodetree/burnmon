// Package history builds the History tab's payload (v0.2 P2, cost basis
// upgraded to V3-1's headline function in v0.3 C3/K3): tokens by class,
// sessions, turns and (where a price book covers the vendor) headline cost,
// bucketed by day, week or month, for one filter (period, date range,
// vendor, owner, client), plus K3's per-client rollup. The page never
// aggregates raw events itself; it calls the bound bmHistory(filter)
// function, which runs Build against the store's events.
package history

import (
	"fmt"
	"sort"
	"time"

	"burnmon/internal/dataset"
	"burnmon/internal/pricing"
	"burnmon/internal/schema"
)

// AgentLabel maps a schema.Event.Agent value to its display name, the same
// names P2's vendor filter lists.
var AgentLabel = map[string]string{
	"claude-code":    "Claude Code",
	"cowork":         "Cowork",
	"codex":          "Codex",
	"hermes":         "Hermes",
	"copilot-cli":    "Copilot CLI",
	"copilot-vscode": "Copilot (VS Code)",
}

// Filter is what the History page's controls send bmHistory. From and To are
// "YYYY-MM-DD", inclusive on both ends. Vendor is an agent key (empty means
// every agent); Owner and Client are empty for "all" or when no owner/client
// rules are configured. Client (K3) narrows the totals/rows the same way
// Owner does; the per-client table (Payload.ClientRows) ignores it, so
// clients stay comparable side by side even while one is selected above.
type Filter struct {
	Period string `json:"period"` // "day", "week" or "month"
	From   string `json:"from"`
	To     string `json:"to"`
	Vendor string `json:"vendor"`
	Owner  string `json:"owner"`
	Client string `json:"client"`
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

	// ByVendor is Task 2's per-vendor split of this bucket's own Tokens/
	// CostUSD, keyed by agent (the same keys AgentLabel and Vendors use), so
	// the History chart can stack one segment per harness rather than one
	// bar for the whole period. Absent (nil) when the bucket carries no
	// agent-tagged event at all (a store predating Vendor/Agent on Event).
	ByVendor map[string]*VendorAgg `json:"by_vendor,omitempty"`
}

// VendorAgg is one agent's share of a bucket's tokens and headline cost, the
// History chart's stacked-bar segment.
type VendorAgg struct {
	Tokens  int64    `json:"tokens"`
	CostUSD *float64 `json:"cost_usd,omitempty"`
}

// Row is one period bucket's Totals, keyed the same way agg.Bucket's maps
// are (month "YYYY-MM", week "YYYY-Www", day "YYYY-MM-DD"), so the page can
// reuse its existing monthLabel/weekLabel/dayWithName formatters.
type Row struct {
	Key string `json:"key"`
	Totals
}

// ClientRow is K3's per-client breakdown, one per client seen under Filter's
// period/range/vendor/owner (not Filter.Client itself, so every client stays
// visible for comparison): tokens, headline cost, active time and session
// count, the same "what did this client cost" answer C2's cost function
// gives everywhere else.
type ClientRow struct {
	Client        string   `json:"client"`
	Tokens        int64    `json:"tokens"`
	CostUSD       *float64 `json:"cost_usd,omitempty"`
	CostNote      string   `json:"cost_note,omitempty"`
	ActiveMinutes float64  `json:"active_minutes"`
	Sessions      int64    `json:"sessions"`
}

// Payload is bmHistory's full return value.
type Payload struct {
	Filter  Filter   `json:"filter"`
	Totals  Totals   `json:"totals"`
	Rows    []Row    `json:"rows"`
	Vendors []string `json:"vendors"` // every agent seen in the store, for the filter's dropdown
	Owners  []string `json:"owners"`  // every owner seen, empty when no owner rules are configured
	// Clients is every client seen in the store (K3), for the filter's
	// dropdown, empty when no owner/client rules are configured (same
	// convention as Owners).
	Clients []string `json:"clients,omitempty"`
	// ClientRows is K3's per-client table, business mode only on the page
	// side; empty when Clients is empty.
	ClientRows []ClientRow `json:"client_rows,omitempty"`
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

// headlineCostUSD sums cfg.CostForEvents' Headline basis across every vendor
// present in events (C2's one cost function, the same one Now, Sessions and
// export use): covered is false only when not one event's vendor has any
// price book at all ("tokens only"), replacing v0.2's "tokens only until
// v0.3" placeholder (v0.3 shipped that cost function, so the note is stale)
// and its single-vendor-only gate (a mixed anthropic+openai "All" selection
// now sums both vendors' headline figures, rather than showing nothing).
func headlineCostUSD(events []schema.Event, cfg *pricing.Config) (usd float64, covered bool) {
	for _, vc := range cfg.CostForEvents(events) {
		if vc.Headline != nil {
			usd += vc.Headline.USD
			covered = true
		}
	}
	return usd, covered
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
	clientSeen := map[string]bool{}
	buckets := map[string]*Totals{}
	bucketSessions := map[string]map[string]bool{}
	bucketTurnEvents := map[string][]schema.Event{}
	// bucketVendorTokens/bucketVendorTurnEvents split the same accumulation
	// above one level further, by agent, for ByVendor's stacked-bar segments.
	bucketVendorTokens := map[string]map[string]int64{}
	bucketVendorTurnEvents := map[string]map[string][]schema.Event{}
	var overallTurnEvents []schema.Event
	var overallSessions = map[string]bool{}
	// clientRowEvents groups every event matching period/range/vendor/owner
	// (deliberately not Filter.Client) by client, for ClientRows below: the
	// per-client table stays comparable across every client even while one
	// is selected in the filter above it.
	clientRowEvents := map[string][]schema.Event{}

	var overall Totals
	for _, e := range events {
		if e.Agent != "" {
			vendorSeen[e.Agent] = true
		}
		if e.Owner != "" {
			ownerSeen[e.Owner] = true
		}
		if e.Client != "" {
			clientSeen[e.Client] = true
		}
		day := e.At.UTC().Format("2006-01-02")
		inDateRange := f.From == "" || f.To == "" || inRange(day, f.From, f.To)
		vendorOK := f.Vendor == "" || e.Agent == f.Vendor
		ownerOK := f.Owner == "" || e.Owner == f.Owner

		if vendorOK && ownerOK && inDateRange && e.Client != "" {
			clientRowEvents[e.Client] = append(clientRowEvents[e.Client], e)
		}

		if !(vendorOK && ownerOK && inDateRange) {
			continue
		}
		if f.Client != "" && e.Client != f.Client {
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

		if e.Agent != "" {
			if bucketVendorTokens[key] == nil {
				bucketVendorTokens[key] = map[string]int64{}
			}
			bucketVendorTokens[key][e.Agent] += e.Input + cw + cr + e.Output
		}

		if isTurn(e) {
			b.Turns++
			overall.Turns++
			bucketTurnEvents[key] = append(bucketTurnEvents[key], e)
			overallTurnEvents = append(overallTurnEvents, e)
			if e.Agent != "" {
				if bucketVendorTurnEvents[key] == nil {
					bucketVendorTurnEvents[key] = map[string][]schema.Event{}
				}
				bucketVendorTurnEvents[key][e.Agent] = append(bucketVendorTurnEvents[key][e.Agent], e)
			}
		}
	}

	for key, b := range buckets {
		b.Tokens = b.Fresh + b.CacheW + b.CacheR + b.Out
		b.Sessions = int64(len(bucketSessions[key]))
		if usd, covered := headlineCostUSD(bucketTurnEvents[key], cfg); covered {
			c := usd
			b.CostUSD = &c
		} else {
			b.CostNote = "tokens only"
		}
		if agentTokens := bucketVendorTokens[key]; len(agentTokens) > 0 {
			b.ByVendor = make(map[string]*VendorAgg, len(agentTokens))
			for agent, tokens := range agentTokens {
				va := &VendorAgg{Tokens: tokens}
				if usd, covered := headlineCostUSD(bucketVendorTurnEvents[key][agent], cfg); covered {
					c := usd
					va.CostUSD = &c
				}
				b.ByVendor[agent] = va
			}
		}
	}
	overall.Tokens = overall.Fresh + overall.CacheW + overall.CacheR + overall.Out
	overall.Sessions = int64(len(overallSessions))
	if usd, covered := headlineCostUSD(overallTurnEvents, cfg); covered {
		c := usd
		overall.CostUSD = &c
	} else {
		overall.CostNote = "tokens only"
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
	clients := make([]string, 0, len(clientSeen))
	for c := range clientSeen {
		clients = append(clients, c)
	}
	sort.Strings(clients)

	clientRows := make([]ClientRow, 0, len(clients))
	for _, c := range clients {
		clientRows = append(clientRows, buildClientRow(c, clientRowEvents[c], cfg))
	}

	return Payload{
		Filter:     f,
		Totals:     overall,
		Rows:       rows,
		Vendors:    vendors,
		Owners:     owners,
		Clients:    clients,
		ClientRows: clientRows,
	}
}

// buildClientRow is K3's per-client table row: tokens and headline cost from
// client's own turn events (C2's cost function), sessions and active time
// via dataset.SessionsFromEvents/ActiveMinutesByClient (K2), the same
// grouping the Sessions tab and export will use.
func buildClientRow(client string, events []schema.Event, cfg *pricing.Config) ClientRow {
	row := ClientRow{Client: client}
	var turnEvents []schema.Event
	for _, e := range events {
		if !isTurn(e) {
			continue
		}
		var cw, cr int64
		if e.CacheWrite != nil {
			cw = *e.CacheWrite
		}
		if e.CacheRead != nil {
			cr = *e.CacheRead
		}
		row.Tokens += e.Input + cw + cr + e.Output
		turnEvents = append(turnEvents, e)
	}
	if usd, covered := headlineCostUSD(turnEvents, cfg); covered {
		c := usd
		row.CostUSD = &c
	} else {
		row.CostNote = "tokens only"
	}

	sessions := dataset.SessionsFromEvents(events, cfg)
	row.Sessions = int64(len(sessions))
	byClient := dataset.ActiveMinutesByClient(sessions)
	row.ActiveMinutes = byClient[client]
	return row
}
