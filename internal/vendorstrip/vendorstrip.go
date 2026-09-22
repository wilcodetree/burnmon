// Package vendorstrip builds the Now page's vendor strip (v0.2 P3): one row
// per vendor seen in the store, tokens today/this week/this month, a total
// row last. Refreshed once a minute from its own timer, not the Now page's
// 2-second poll (bmLive), so it runs one SQL aggregate against the whole
// events table rather than reusing bmLive's windowed read.
package vendorstrip

import (
	"sort"
	"time"

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

// Payload is bmVendorStrip's return value.
type Payload struct {
	GeneratedAt string `json:"generated_at"`
	Rows        []Row  `json:"rows"`
	Total       Row    `json:"total"`
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
// across every vendor.
func Build(st *store.Store, now time.Time) (Payload, error) {
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

	return Payload{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Rows:        rows,
		Total:       total,
	}, nil
}
