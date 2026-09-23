// Package vendorstrip builds the Now page's vendor strip (v0.2 P3): one row
// per vendor seen in the store, tokens today/this week/this month, a total
// row last. Refreshed once a minute from its own timer, not the Now page's
// 2-second poll (bmLive), so it runs one SQL aggregate against the whole
// events table rather than reusing bmLive's windowed read.
package vendorstrip

import (
	"sort"
	"time"

	"burnmon/internal/pricing"
	"burnmon/internal/store"
)

// AgentLabel mirrors internal/history.AgentLabel: the same five display
// names, since the strip's rows and History's vendor filter must read as
// the same vocabulary.
var AgentLabel = map[string]string{
	"claude-code": "Claude Code",
	"cowork":      "Cowork",
	"codex":       "Codex",
	"hermes":      "Hermes",
	"copilot-cli": "Copilot CLI",
}

// Row is one vendor's (or, as Total, every vendor's) token totals.
type Row struct {
	Agent      string `json:"agent"` // "" on the Total row
	AgentLabel string `json:"agent_label"`
	Today      int64  `json:"today"`
	Week       int64  `json:"week"`
	Month      int64  `json:"month"`
}

// CreditsLeft is C3/K3's account-wide "credits left" figure, present only
// when Config.CopilotPlan names a plan the credit book covers.
type CreditsLeft struct {
	Plan           string  `json:"plan"`
	MonthlyCredits float64 `json:"monthly_credits"`
	Left           float64 `json:"left"`
}

// Payload is bmVendorStrip's return value.
type Payload struct {
	GeneratedAt string `json:"generated_at"`
	Rows        []Row  `json:"rows"`
	Total       Row    `json:"total"`
	// CopilotCreditsLeft is refreshed on the same 1-minute cadence as the
	// rest of this payload (rather than bmLive's 2-second poll, or a fourth
	// backend call the business-mode card would otherwise need): an
	// account-wide plan balance does not need sub-minute freshness. nil when
	// Config.CopilotPlan is unset or unknown.
	CopilotCreditsLeft *CreditsLeft `json:"copilot_credits_left,omitempty"`
}

// dayStart, weekStart and monthStart are UTC calendar boundaries: dayStart is
// midnight today, weekStart is Monday 00:00 of the current ISO week (same
// week convention as internal/agg's weekKey and internal/history's "week"
// period), monthStart is the 1st of the current month 00:00, all matching
// the UTC timestamps every stored event already carries.
func dayStart(now time.Time) time.Time {
	now = now.UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func weekStart(now time.Time) time.Time {
	d := dayStart(now)
	// time.Weekday: Sunday=0 ... Saturday=6; ISO weeks start Monday.
	offset := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -offset)
}

func monthStart(now time.Time) time.Time {
	now = now.UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// Build runs store.VendorStripTotals (one SQL query, grouped by agent) and
// shapes it into Payload, sorted by AgentLabel and with the total row summed
// across every vendor. cfg (added C3/K3) is only used for CopilotCreditsLeft
// below; the token totals themselves are unaffected by it.
func Build(st *store.Store, cfg *pricing.Config, now time.Time) (Payload, error) {
	totals, err := st.VendorStripTotals(dayStart(now), weekStart(now), monthStart(now))
	if err != nil {
		return Payload{}, err
	}

	rows := make([]Row, 0, len(totals))
	var total Row
	for _, t := range totals {
		label := AgentLabel[t.Agent]
		if label == "" {
			label = t.Agent
		}
		rows = append(rows, Row{Agent: t.Agent, AgentLabel: label, Today: t.Today, Week: t.Week, Month: t.Month})
		total.Today += t.Today
		total.Week += t.Week
		total.Month += t.Month
	}
	total.AgentLabel = "Total"

	sort.Slice(rows, func(i, j int) bool { return rows[i].AgentLabel < rows[j].AgentLabel })

	p := Payload{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Rows:        rows,
		Total:       total,
	}
	if cl, ok := creditsLeftForMonth(st, cfg, now); ok {
		p.CopilotCreditsLeft = &cl
	}
	return p, nil
}

// creditsLeftForMonth re-reads this calendar month's events (a second query
// beyond VendorStripTotals: the credits calculation needs cache-write/read
// splits per event, which the token-sum SQL aggregate above does not carry)
// and runs cfg.CopilotCreditsLeft over them. Only called at all when
// Config.CopilotPlan is set, so a burnmon.json with no Copilot plan
// configured never pays this extra query.
func creditsLeftForMonth(st *store.Store, cfg *pricing.Config, now time.Time) (CreditsLeft, bool) {
	if cfg == nil || cfg.CopilotPlan == "" {
		return CreditsLeft{}, false
	}
	events, err := st.EventsSince(monthStart(now))
	if err != nil {
		return CreditsLeft{}, false
	}
	left, plan, ok := cfg.CopilotCreditsLeft(events)
	if !ok {
		return CreditsLeft{}, false
	}
	return CreditsLeft{Plan: cfg.CopilotPlan, MonthlyCredits: plan.MonthlyCredits, Left: left}, true
}
