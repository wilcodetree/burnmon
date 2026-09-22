// Package live builds the Now page's snapshot: which sessions are running
// right now, their context fill, and the last 30 minutes of per-minute burn,
// from the same Events the store already holds. No new ingest logic lives
// here; internal/watch keeps the store current, this package only reads it.
package live

import (
	"sort"
	"time"

	"burnmon/internal/pricing"
	"burnmon/internal/schema"
)

// ClassCounts is one session's token classes inside one minute bucket.
type ClassCounts struct {
	Fresh      int64 `json:"fresh"`
	CacheWrite int64 `json:"cache_write"`
	CacheRead  int64 `json:"cache_read"`
	Output     int64 `json:"output"`
}

// MinuteBucket is one minute of the live chart's 30-minute window.
type MinuteBucket struct {
	Minute    string                 `json:"minute"` // RFC3339, truncated to the minute, UTC
	BySession map[string]ClassCounts `json:"by_session"`
	Cost      float64                `json:"cost"`
}

// Session is one running session (or subagent) on the Now page.
type Session struct {
	SessionID     string     `json:"session_id"`
	Agent         string     `json:"agent"`
	Surface       string     `json:"surface"`
	Model         string     `json:"model"`
	Project       string     `json:"project"`
	Start         string     `json:"start"`     // RFC3339
	LastTurn      string     `json:"last_turn"` // RFC3339
	TurnCount     int        `json:"turn_count"`
	Context       int64      `json:"context"`                  // last turn: Input + CacheRead + CacheWrite
	ContextWindow int64      `json:"context_window,omitempty"` // 0 means unknown
	Tokens        int64      `json:"tokens"`                   // whole session
	Cost          float64    `json:"cost"`
	CacheHitRatio float64    `json:"cache_hit_ratio"` // last turn: cache_read / (fresh + cache_read)
	Subagents     []*Session `json:"subagents,omitempty"`
}

// ForecastDay is one day of the Now page's forecast placeholder.
type ForecastDay struct {
	Date   string  `json:"date"` // YYYY-MM-DD
	Cost   float64 `json:"cost"`
	Tokens int64   `json:"tokens"`
}

// Forecast is the Now page's forecast card: history only until a scored week
// exists (v0.1 has none), per the spec's "no forecast without a scored week".
type Forecast struct {
	Message string        `json:"message"`
	Days    []ForecastDay `json:"days"`
}

// Snapshot is what bmLive() and `burnmon-cli.exe live --json` both return.
type Snapshot struct {
	GeneratedAt          string         `json:"generated_at"`
	RunningWindowSeconds float64        `json:"running_window_seconds"`
	Sessions             []*Session     `json:"sessions"`
	Chart                []MinuteBucket `json:"chart"`
	Forecast             Forecast       `json:"forecast"`
}

const chartWindow = 30 * time.Minute

// turnCost returns one event's subscription-share cost, the same model
// buildSession uses: output-driven for Claude, list-price for OpenAI (no
// calibrated Codex invoice exists in v0.1).
func turnCost(e schema.Event, cfg *pricing.Config) float64 {
	if e.Vendor == "openai" {
		cr := int64(0)
		if e.CacheRead != nil {
			cr = *e.CacheRead
		}
		c, _ := cfg.OpenAICallCostUSD(e.Model, e.Input, cr, e.Output)
		return c
	}
	fam := cfg.ModelFamily(e.Model)
	return cfg.CallCostSubUSD(fam, e.Input, e.Output)
}

// isTurn reports whether e is a real API-call turn rather than the claude
// adapter's synthetic tool-only event (see internal/dataset/fromstore.go's
// buildSession, same condition).
func isTurn(e schema.Event) bool {
	return !(e.Model == "" && e.Input == 0 && e.Output == 0)
}

func contextOf(e schema.Event) int64 {
	c := e.Input
	if e.CacheRead != nil {
		c += *e.CacheRead
	}
	if e.CacheWrite != nil {
		c += *e.CacheWrite
	}
	return c
}

// BuildSnapshot groups events by (Vendor, SessionID), keeps the ones whose
// last turn is within cfg's running window of now, nests a running
// subagent (ParentID set) under its running parent when the parent is also
// present, and builds the last 30 minutes of per-minute chart buckets from
// every event in that window, running or not.
func BuildSnapshot(events []schema.Event, cfg *pricing.Config, now time.Time) Snapshot {
	type group struct {
		id     string
		events []schema.Event
	}
	groups := map[string]*group{}
	var order []string
	for _, e := range events {
		key := e.Vendor + "|" + e.SessionID
		g := groups[key]
		if g == nil {
			g = &group{id: e.SessionID}
			groups[key] = g
			order = append(order, key)
		}
		g.events = append(g.events, e)
	}
	sort.Strings(order)

	runningWindow := time.Duration(cfg.RunningWindowSeconds()) * time.Second

	sessions := map[string]*Session{}
	parentOf := map[string]string{}
	var runningOrder []string
	for _, key := range order {
		g := groups[key]
		sort.SliceStable(g.events, func(i, j int) bool { return g.events[i].At.Before(g.events[j].At) })

		var turns []schema.Event
		for _, e := range g.events {
			if isTurn(e) {
				turns = append(turns, e)
			}
		}
		if len(turns) == 0 {
			continue
		}
		last := turns[len(turns)-1]
		if last.At.IsZero() || now.Sub(last.At) > runningWindow {
			continue
		}

		var tokens int64
		var cost float64
		for _, t := range turns {
			cw, cr := int64(0), int64(0)
			if t.CacheWrite != nil {
				cw = *t.CacheWrite
			}
			if t.CacheRead != nil {
				cr = *t.CacheRead
			}
			tokens += t.Input + cw + cr + t.Output
			cost += turnCost(t, cfg)
		}

		lastCtx := contextOf(last)
		fresh := last.Input
		cacheR := int64(0)
		if last.CacheRead != nil {
			cacheR = *last.CacheRead
		}
		var hitRatio float64
		if fresh+cacheR > 0 {
			hitRatio = float64(cacheR) / float64(fresh+cacheR)
		}
		window, _ := cfg.ContextWindow(last.Model)

		s := &Session{
			SessionID:     g.id,
			Agent:         turns[0].Agent,
			Surface:       last.Surface,
			Model:         last.Model,
			Project:       firstNonEmptyProject(turns),
			Start:         turns[0].At.UTC().Format(time.RFC3339),
			LastTurn:      last.At.UTC().Format(time.RFC3339),
			TurnCount:     len(turns),
			Context:       lastCtx,
			ContextWindow: window,
			Tokens:        tokens,
			Cost:          cost,
			CacheHitRatio: hitRatio,
		}
		sessions[g.id] = s
		runningOrder = append(runningOrder, g.id)
		if p := turns[0].ParentID; p != "" {
			parentOf[g.id] = p
		}
	}

	var top []*Session
	for _, id := range runningOrder {
		s := sessions[id]
		if p, ok := parentOf[id]; ok {
			if parent, ok := sessions[p]; ok && parent != s {
				parent.Subagents = append(parent.Subagents, s)
				continue
			}
		}
		top = append(top, s)
	}

	return Snapshot{
		GeneratedAt:          now.UTC().Format(time.RFC3339),
		RunningWindowSeconds: cfg.RunningWindowSeconds(),
		Sessions:             top,
		Chart:                buildChart(events, cfg, now),
		Forecast:             buildForecast(events, cfg, now),
	}
}

func firstNonEmptyProject(events []schema.Event) string {
	for _, e := range events {
		if e.Project != "" {
			return e.Project
		}
	}
	return ""
}

// buildChart buckets every real turn in the last 30 minutes by minute and by
// session, running or not: a session that just fell out of the running
// window a moment ago still belongs on the chart's tail.
func buildChart(events []schema.Event, cfg *pricing.Config, now time.Time) []MinuteBucket {
	from := now.Add(-chartWindow)
	buckets := map[string]*MinuteBucket{}
	var order []string
	for _, e := range events {
		if !isTurn(e) || e.At.IsZero() || e.At.Before(from) || e.At.After(now) {
			continue
		}
		minute := e.At.UTC().Truncate(time.Minute).Format(time.RFC3339)
		b := buckets[minute]
		if b == nil {
			b = &MinuteBucket{Minute: minute, BySession: map[string]ClassCounts{}}
			buckets[minute] = b
			order = append(order, minute)
		}
		cc := b.BySession[e.SessionID]
		cc.Fresh += e.Input
		if e.CacheWrite != nil {
			cc.CacheWrite += *e.CacheWrite
		}
		if e.CacheRead != nil {
			cc.CacheRead += *e.CacheRead
		}
		cc.Output += e.Output
		b.BySession[e.SessionID] = cc
		b.Cost += turnCost(e, cfg)
	}
	sort.Strings(order)
	out := make([]MinuteBucket, 0, len(order))
	for _, m := range order {
		out = append(out, *buckets[m])
	}
	return out
}

// buildForecast sums every real turn's tokens and cost per UTC calendar day
// for the last 7 days, history only: v0.1 has no scored week to project a
// line from (see the spec's Step 3, "cache clock and turn ticker... if the
// week allows" and the features note's forecast rule).
func buildForecast(events []schema.Event, cfg *pricing.Config, now time.Time) Forecast {
	today := now.UTC().Truncate(24 * time.Hour)
	from := today.AddDate(0, 0, -6)
	days := map[string]*ForecastDay{}
	var order []string
	for d := from; !d.After(today); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		days[key] = &ForecastDay{Date: key}
		order = append(order, key)
	}
	for _, e := range events {
		if !isTurn(e) || e.At.IsZero() {
			continue
		}
		key := e.At.UTC().Format("2006-01-02")
		d, ok := days[key]
		if !ok {
			continue
		}
		cw, cr := int64(0), int64(0)
		if e.CacheWrite != nil {
			cw = *e.CacheWrite
		}
		if e.CacheRead != nil {
			cr = *e.CacheRead
		}
		d.Tokens += e.Input + cw + cr + e.Output
		d.Cost += turnCost(e, cfg)
	}
	out := make([]ForecastDay, 0, len(order))
	for _, k := range order {
		out = append(out, *days[k])
	}
	return Forecast{
		Message: "Needs one scored week before a forecast line; showing the last 7 days of history.",
		Days:    out,
	}
}
