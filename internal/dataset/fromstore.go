package dataset

import (
	"sort"

	"burnmon/internal/pricing"
	"burnmon/internal/scan"
	"burnmon/internal/schema"
)

// SessionsFromEvents groups events by (Vendor, SessionID) and rebuilds the
// same *scan.Session shape ParseSession used to produce directly, applying
// cfg's pricing fresh every call: events carry no cost, so a pricing.Config
// change never needs a cache invalidation, only a rebuild from the same
// stored events.
func SessionsFromEvents(events []schema.Event, cfg *pricing.Config) []*scan.Session {
	type group struct {
		key    string
		events []schema.Event
	}
	groups := map[string]*group{}
	var order []string
	for _, e := range events {
		key := e.Vendor + "|" + e.SessionID
		g := groups[key]
		if g == nil {
			g = &group{key: key}
			groups[key] = g
			order = append(order, key)
		}
		g.events = append(g.events, e)
	}
	sort.Strings(order)

	sessions := make([]*scan.Session, 0, len(order))
	for _, key := range order {
		if s := buildSession(groups[key].events, cfg); s != nil {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

func round6f(x float64) float64 {
	const scale = 1e6
	return float64(int64(x*scale+0.5)) / scale
}

// buildSession is ParseSession's old tail loop
// (internal/scan/parse.go:342-448 before the v0.1 Step 1 move), unchanged
// in its accumulation logic, fed from a session's stored Events instead of
// a freshly parsed turn list.
func buildSession(events []schema.Event, cfg *pricing.Config) *scan.Session {
	if len(events) == 0 {
		return nil
	}

	var stamps []string
	for _, e := range events {
		if !e.At.IsZero() {
			stamps = append(stamps, e.At.Format("2006-01-02T15:04:05Z"))
		}
	}
	if len(stamps) == 0 {
		return nil
	}
	sort.Strings(stamps)

	title, sessionID, surface := "", events[0].SessionID, events[0].Surface
	for _, e := range events {
		if title == "" && e.Title != "" {
			title = e.Title
		}
	}
	if title == "" {
		title = "(untitled session)"
	}

	perModel := map[string]*scan.PerModel{}
	daily := map[string]*scan.PerModel{}
	tools := map[string]int64{}
	var fresh, cacheW, cacheR, out, callsN int64
	var cost, costSub float64

	for _, e := range events {
		for name, c := range e.Tools {
			tools[name] += c
		}
		// A synthetic tool-only event (see the claude adapter's orphan
		// handling) carries no model and no tokens: it contributes to
		// Tools only, not to call counts or cost.
		if e.Model == "" && e.Input == 0 && e.Output == 0 {
			continue
		}
		callsN++
		var cw, cr int64
		if e.CacheWrite != nil {
			cw = *e.CacheWrite
		}
		if e.CacheRead != nil {
			cr = *e.CacheRead
		}
		fam := cfg.ModelFamily(e.Model)
		c := cfg.CallCostUSD(fam, e.Input, cw, cr, e.Output)
		cs := cfg.CallCostSubUSD(fam, e.Input, e.Output)
		tokens := e.Input + cw + cr + e.Output
		fresh += e.Input
		cacheW += cw
		cacheR += cr
		out += e.Output
		cost += c
		costSub += cs

		label := cfg.Label(fam)
		pm := perModel[label]
		if pm == nil {
			pm = &scan.PerModel{}
			perModel[label] = pm
		}
		pm.Calls++
		pm.Tokens += tokens
		pm.Cost += c
		pm.CostSub += cs

		ts := e.At.Format("2006-01-02T15:04:05Z")
		if e.At.IsZero() {
			ts = stamps[0]
		}
		day := ts
		if len(day) >= 10 {
			day = day[:10]
		}
		d := daily[day]
		if d == nil {
			d = &scan.PerModel{}
			daily[day] = d
		}
		d.Calls++
		d.Tokens += tokens
		d.Cost += c
		d.CostSub += cs
	}
	if callsN == 0 {
		return nil
	}

	for _, pm := range perModel {
		pm.Cost = round6f(pm.Cost)
		pm.CostSub = round6f(pm.CostSub)
	}
	for _, d := range daily {
		d.Cost = round6f(d.Cost)
		d.CostSub = round6f(d.CostSub)
	}

	var toolsOut map[string]int64
	if len(tools) > 0 {
		toolsOut = tools
	}

	cwd := ""
	for _, e := range events {
		if e.Project != "" {
			cwd = e.Project
			break
		}
	}

	return &scan.Session{
		SessionID:      sessionID,
		Title:          title,
		Surface:        surface,
		CWD:            cwd,
		Start:          stamps[0],
		End:            stamps[len(stamps)-1],
		Calls:          callsN,
		Tokens:         fresh + cacheW + cacheR + out,
		Cost:           round6f(cost),
		CostSub:        round6f(costSub),
		CostPerCall:    round6f(cost / float64(callsN)),
		CostSubPerCall: round6f(costSub / float64(callsN)),
		Fresh:          fresh,
		CacheW:         cacheW,
		CacheR:         cacheR,
		Out:            out,
		Models:         perModel,
		Tools:          toolsOut,
		Daily:          daily,
		Long:           callsN > scan.LongSessionCalls,
	}
}
