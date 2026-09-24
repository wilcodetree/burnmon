package dataset

import (
	"math"
	"sort"
	"time"

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
	return math.Round(x*1e6) / 1e6
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
	var times []time.Time
	for _, e := range events {
		if !e.At.IsZero() {
			stamps = append(stamps, e.At.Format("2006-01-02T15:04:05.000Z"))
			times = append(times, e.At)
		}
	}
	if len(stamps) == 0 {
		return nil
	}
	sort.Strings(stamps)
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	title, owner, client, sessionID, surface := "", "", "", events[0].SessionID, events[0].Surface
	vendor, agent := events[0].Vendor, ""
	for _, e := range events {
		if title == "" && e.Title != "" {
			title = e.Title
		}
		if owner == "" && e.Owner != "" {
			owner = e.Owner
		}
		if client == "" && e.Client != "" {
			client = e.Client
		}
		if agent == "" && e.Agent != "" {
			agent = e.Agent
		}
	}
	if title == "" {
		title = "(untitled session)"
	}

	perModel := map[string]*scan.PerModel{}
	daily := map[string]*scan.PerModel{}
	tools := map[string]int64{}
	var fresh, cacheW, cacheR, out, callsN, unpriced int64
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
		tokens := e.Input + cw + cr + e.Output

		// V3.1 cleanup: price every vendor through its own book
		// (pricing.Config.EventCost), not just OpenAI. Before this fix, any
		// non-OpenAI vendor (Copilot, Hermes) fell through to Claude's
		// family-generic price table and was priced as if it were a Claude
		// call; a vendor with no book entry for this exact model now prices
		// at 0 and counts toward Unpriced ("no price"), never a guessed
		// Claude figure. Side effect, not just the Copilot/Hermes fix: an
		// anthropic event is now also priced from AnthropicBook's exact model
		// id (matching Now/History/export, C2's "one function ... so the
		// numbers agree everywhere") instead of the family-generic table this
		// package used to use on its own, so Sessions' own per-call figures
		// for Claude shift too (e.g. Sonnet's family price was $3/$15, the
		// book's claude-sonnet-5 is $2/$10), and a Claude model id absent
		// from the book (an older id, or a new one the book has not caught
		// up with yet) now shows "no price" here rather than a guessed
		// Sonnet figure.
		c, cs, label, unpricedCall := cfg.EventCost(e)
		if unpricedCall {
			unpriced += tokens
		}
		fresh += e.Input
		cacheW += cw
		cacheR += cr
		out += e.Output
		cost += c
		costSub += cs

		pm := perModel[label]
		if pm == nil {
			pm = &scan.PerModel{}
			perModel[label] = pm
		}
		pm.Calls++
		pm.Tokens += tokens
		pm.Cost += c
		pm.CostSub += cs

		ts := e.At.Format("2006-01-02T15:04:05.000Z")
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

	activeMinutes := activeTimeMinutes(times, cfg.ActiveIdleMinutesOrDefault())

	return &scan.Session{
		SessionID:      sessionID,
		Title:          title,
		Owner:          owner,
		Client:         client,
		Vendor:         vendor,
		Agent:          agent,
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
		Unpriced:       unpriced,
		ActiveMinutes:  activeMinutes,
	}
}

// activeTimeMinutes is K2's active time: the sum of gaps between
// consecutive, sorted timestamps, any gap strictly greater than
// idleCutoffMinutes counting 0 rather than the real gap (spec: "any gap
// above active_idle_minutes counts zero"; exactly at the cutoff still
// counts). A session with 0 or 1 timestamps has no gaps, so 0.
func activeTimeMinutes(sorted []time.Time, idleCutoffMinutes float64) float64 {
	if len(sorted) < 2 {
		return 0
	}
	cutoff := time.Duration(idleCutoffMinutes * float64(time.Minute))
	var total time.Duration
	for i := 1; i < len(sorted); i++ {
		gap := sorted[i].Sub(sorted[i-1])
		if gap <= cutoff {
			total += gap
		}
	}
	return total.Minutes()
}

// ActiveTimeMinutes is K2's active time computed directly off events, for
// callers that group events into buckets finer than a whole session (K4
// export's per-row active minutes, one row per day/owner/client/vendor/model
// rather than per session).
func ActiveTimeMinutes(events []schema.Event, cfg *pricing.Config) float64 {
	var times []time.Time
	for _, e := range events {
		if !e.At.IsZero() {
			times = append(times, e.At)
		}
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	return activeTimeMinutes(times, cfg.ActiveIdleMinutesOrDefault())
}

// ActiveMinutesByClient sums each session's ActiveMinutes into its Client
// (K2: "per client as the sum of its sessions"). A session with Client ==
// "" (a store not yet reowned since K1 landed) is omitted rather than
// bucketed under "": callers that want an "unknown" bucket can check for it
// separately via the sessions list itself.
func ActiveMinutesByClient(sessions []*scan.Session) map[string]float64 {
	out := map[string]float64{}
	for _, s := range sessions {
		if s.Client == "" {
			continue
		}
		out[s.Client] += s.ActiveMinutes
	}
	return out
}
