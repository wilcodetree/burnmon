// Package live builds the Now page's snapshot: which sessions are running
// right now, their context fill, and the last 30 minutes of per-minute burn,
// from the same Events the store already holds. No new ingest logic lives
// here; internal/watch keeps the store current, this package only reads it.
package live

import (
	"fmt"
	"sort"
	"time"

	"burnmon/internal/insight"
	"burnmon/internal/pricing"
	"burnmon/internal/schema"
	"burnmon/internal/store"
)

// ClassCounts is one session's token classes inside one minute bucket.
type ClassCounts struct {
	Fresh      int64 `json:"fresh"`
	CacheWrite int64 `json:"cache_write"`
	CacheRead  int64 `json:"cache_read"`
	Output     int64 `json:"output"`
}

// Bucket is one fixed whole-minute slot of the Now page's 30-minute sliding
// chart (F3, widened from 10 seconds to 60 by N2, v0.2.2, SESSION_LOG.md):
// always present in Snapshot.Chart, zero-valued when no turn landed in it,
// so the frontend never has to derive a dense timeline from a sparse one.
// At is UTC; the frontend renders it in the viewer's local time (F3's fix
// for the axis showing UTC while local read differently).
type Bucket struct {
	At        string                 `json:"at"`
	BySession map[string]ClassCounts `json:"by_session,omitempty"`
	Cost      float64                `json:"cost"`
}

// Session is one running session (or subagent) on the Now page.
type Session struct {
	Vendor        string     `json:"vendor"`
	SessionID     string     `json:"session_id"`
	Agent         string     `json:"agent"`
	Surface       string     `json:"surface"`
	Model         string     `json:"model"`
	Project       string     `json:"project"`
	// Client is K1's client map result for this session's project, "" when
	// no owner/client rules are configured (mirrors scan.Session.Client;
	// C3's business-mode card shows it, dev mode does not).
	Client        string     `json:"client,omitempty"`
	Start         string     `json:"start"`     // RFC3339
	LastTurn      string     `json:"last_turn"` // RFC3339
	TurnCount     int        `json:"turn_count"`
	Context       int64      `json:"context"`                  // last turn: Input + CacheRead + CacheWrite
	ContextWindow int64      `json:"context_window,omitempty"` // 0 means unknown
	Tokens        int64      `json:"tokens"`                   // whole session
	Cost          float64    `json:"cost"`
	// BusinessCost is C3's headline-basis figure for this session's own
	// events (pricing.Config.CostForEvents' Headline for this vendor): the
	// euro figure the business-mode card shows, with the basis named next to
	// it. nil for a vendor with no price book at all ("tokens only", e.g.
	// Hermes).
	BusinessCost  *pricing.BasisCost `json:"business_cost,omitempty"`
	CacheHitRatio float64    `json:"cache_hit_ratio"` // last turn: cache_read / (fresh + cache_read)
	Subagents     []*Session `json:"subagents,omitempty"`
	// Findings is I3's marker source: insight.Analyze run on this session's
	// own turns from the same windowed events BuildSnapshot already
	// grouped, so a running session's re-prefill and compaction findings
	// are free of any extra store read.
	Findings []insight.Finding `json:"findings,omitempty"`
	// Runway is I2's context-runway rule rendered as the Now card's one-line
	// gauge text (42B): "about N turns to 80%", or "runway unknown" when
	// insight found no context-runway finding for this session. The marker,
	// ticker and drawer for every other finding kind stay 43A's job.
	Runway string `json:"runway"`
}

// TurnEvent is one real API-call turn inside the Now page's 30-minute
// window, running session or not: the I3 turn ticker's one-line-per-turn
// source, and the marker overlay's per-finding placement, both need every
// individual turn rather than BuildSnapshot's per-10-second, per-session
// aggregation (Bucket.BySession sums several turns together when more than
// one lands in the same slot).
type TurnEvent struct {
	Vendor    string  `json:"vendor"`
	Agent     string  `json:"agent"`
	SessionID string  `json:"session_id"`
	Model     string  `json:"model"`
	// Turn is 1-based over this session's own real API-call turns, the same
	// numbering insight.Finding.Turn and BuildTurnDetail's turn argument use,
	// so a ticker line's turn number is what bmTurn(sessionID, turn) expects
	// back.
	Turn       int     `json:"turn"`
	At         string  `json:"at"` // RFC3339 UTC
	Fresh      int64   `json:"fresh"`
	CacheWrite int64   `json:"cache_write"`
	CacheRead  int64   `json:"cache_read"`
	Output     int64   `json:"output"`
	// Finding is this turn's own finding, when insight.Analyze found exactly
	// this Turn among the session's findings; nil on an unremarkable turn.
	// A turn with more than one finding kind (rare) carries only the first,
	// in Kind order: the ticker line and marker both show one finding per
	// turn, the drawer (bmTurn) is where every finding at that turn appears.
	Finding *insight.Finding `json:"finding,omitempty"`
}

// Snapshot is what bmLive() and `burnmon-cli.exe live --json` both return.
type Snapshot struct {
	GeneratedAt          string     `json:"generated_at"`
	RunningWindowSeconds float64    `json:"running_window_seconds"`
	Sessions             []*Session `json:"sessions"`
	// WindowStart is the chart's oldest (leftmost) slot, RFC3339 UTC.
	// BucketSeconds is each Chart slot's width (F3: 10). The frontend must
	// derive every slot's position from these two plus len(Chart), never
	// from the events themselves: Chart is already dense.
	WindowStart   string   `json:"window_start"`
	BucketSeconds int      `json:"bucket_seconds"`
	Chart         []Bucket `json:"chart"`
	// Turns is I3's turn ticker source: every real turn across every
	// session in the chart window, newest first, capped at turnTickerCap.
	Turns []TurnEvent `json:"turns"`
}

// turnTickerCap is I3's fixed cap on the turn ticker: "capped at 50 lines".
const turnTickerCap = 50

// ChartWindow is how far back both the chart and BuildSnapshot's own
// "running" detection look. Exported so main.go's bmLive binding can pull
// exactly this much history via store.EventsSince instead of the whole
// table (F1): safe because pricing.Config.RunningWindowSeconds defaults to
// 10 minutes, well inside this 30-minute window, so no running session's
// last turn ever falls outside what EventsSince(now-ChartWindow) returns.
const ChartWindow = 30 * time.Minute

// BucketSeconds is the width of one Now-page chart slot: a fixed 30-minute,
// 60-second-bucket, 30-slot running chart (N2, v0.2.2: widened from F3's
// 10-second buckets so each slot is a whole minute, the last 30 closed
// minutes plus the current one, which grows as more of it elapses, matching
// windowEnd's own truncation to this width below).
const BucketSeconds = 60

// chartSlots is ChartWindow's slot count at BucketSeconds width (30).
const chartSlots = int(ChartWindow / (BucketSeconds * time.Second))

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
			Vendor:        turns[0].Vendor,
			SessionID:     g.id,
			Agent:         turns[0].Agent,
			Surface:       last.Surface,
			Model:         last.Model,
			Project:       firstNonEmptyProject(turns),
			Client:        firstNonEmptyClient(turns),
			Start:         turns[0].At.UTC().Format(time.RFC3339),
			LastTurn:      last.At.UTC().Format(time.RFC3339),
			TurnCount:     len(turns),
			Context:       lastCtx,
			ContextWindow: window,
			Tokens:        tokens,
			Cost:          cost,
			BusinessCost:  headlineForEvents(turns, cfg),
			CacheHitRatio: hitRatio,
		}
		s.Findings = insight.Analyze(turns, cfg)
		s.Runway = insight.RunwayText(s.Findings)
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

	// windowEnd anchors the chart's right edge at or after now (never
	// before): truncating windowStart itself down from now-ChartWindow, as
	// an earlier version did, always left a 0-10s gap between the last
	// bucket and now, and silently dropped any event landing in that gap
	// (idx computed >= chartSlots) from every bucket, including the very
	// turn a live poll just picked up.
	windowEnd := now.Truncate(BucketSeconds * time.Second)
	if windowEnd.Before(now) {
		windowEnd = windowEnd.Add(BucketSeconds * time.Second)
	}
	windowStart := windowEnd.Add(-ChartWindow)
	return Snapshot{
		GeneratedAt:          now.UTC().Format(time.RFC3339),
		RunningWindowSeconds: cfg.RunningWindowSeconds(),
		Sessions:             top,
		WindowStart:          windowStart.UTC().Format(time.RFC3339),
		BucketSeconds:        BucketSeconds,
		Chart:                buildChart(events, cfg, windowStart, now),
		Turns:                buildTurns(events, cfg, windowStart, now),
	}
}

// buildTurns implements I3's turn ticker and marker source: every real turn
// across every (vendor, session_id) group with at least one turn inside
// [windowStart, now], newest first, capped at turnTickerCap. Groups its own
// events independently of BuildSnapshot's sessions map (which only keeps
// still-running groups) so a session that stopped a few minutes ago but is
// still inside the 30-minute chart window still contributes its turns and
// findings here.
func buildTurns(events []schema.Event, cfg *pricing.Config, windowStart, now time.Time) []TurnEvent {
	type group struct {
		vendor, sessionID string
		events            []schema.Event
	}
	groups := map[string]*group{}
	var order []string
	for _, e := range events {
		if !isTurn(e) || e.At.IsZero() || e.At.Before(windowStart) || e.At.After(now) {
			continue
		}
		key := e.Vendor + "|" + e.SessionID
		g := groups[key]
		if g == nil {
			g = &group{vendor: e.Vendor, sessionID: e.SessionID}
			groups[key] = g
			order = append(order, key)
		}
		g.events = append(g.events, e)
	}

	var out []TurnEvent
	for _, key := range order {
		g := groups[key]
		sort.SliceStable(g.events, func(i, j int) bool { return g.events[i].At.Before(g.events[j].At) })
		findings := insight.Analyze(g.events, cfg)
		findingByTurn := map[int]*insight.Finding{}
		for i := range findings {
			f := findings[i]
			if findingByTurn[f.Turn] == nil {
				findingByTurn[f.Turn] = &f
			}
		}
		for i, e := range g.events {
			turn := i + 1
			te := TurnEvent{
				Vendor: g.vendor, Agent: e.Agent, SessionID: g.sessionID, Model: e.Model,
				Turn: turn, At: e.At.UTC().Format(time.RFC3339),
				Fresh: e.Input, Output: e.Output,
			}
			if e.CacheWrite != nil {
				te.CacheWrite = *e.CacheWrite
			}
			if e.CacheRead != nil {
				te.CacheRead = *e.CacheRead
			}
			te.Finding = findingByTurn[turn]
			out = append(out, te)
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].At > out[j].At })
	if len(out) > turnTickerCap {
		out = out[:turnTickerCap]
	}
	return out
}

// TurnDetail is bmTurn(sessionID, turn)'s payload: the I3 "explain this
// spike" drawer's whole content in one struct, so the template's drawer is
// one branch fed by one bound function per the spec.
type TurnDetail struct {
	SessionID  string             `json:"session_id"`
	Vendor     string             `json:"vendor"`
	Agent      string             `json:"agent"`
	Model      string             `json:"model"`
	Turn       int                `json:"turn"`
	At         string             `json:"at"` // RFC3339 UTC
	Fresh      int64              `json:"fresh"`
	CacheWrite int64              `json:"cache_write"`
	CacheRead  int64              `json:"cache_read"`
	Output     int64              `json:"output"`
	// HasGap is false for a session's own first turn: there is no previous
	// turn to measure a gap against.
	HasGap     bool               `json:"has_gap"`
	GapSeconds float64            `json:"gap_seconds,omitempty"`
	ToolCalls  []schema.ToolCall  `json:"tool_calls,omitempty"`
	// Files is ToolCalls' own Path values, deduplicated, first-seen order:
	// "files read where the trail has them" (I3).
	Files    []string          `json:"files,omitempty"`
	Findings []insight.Finding `json:"findings,omitempty"`
}

// ErrTurnNotFound is BuildTurnDetail's error when sessionID has no turn
// numbered turn (an out-of-range click, or a session bmTurn's caller no
// longer has events for).
var ErrTurnNotFound = fmt.Errorf("live: turn not found")

// BuildTurnDetail is bmTurn's implementation: sessionID's every stored event
// (any vendor, EventsForSession), re-derives that session's own turns and
// insight.Analyze findings exactly as buildTurns and BuildSnapshot already
// do, then reports the one at 1-based turn, its tool calls (S2's tool_calls,
// looked up by the turn's own RequestID as its Turn key, the join
// internal/store.ToolCallsForTurn documents) and every file path they
// carried.
func BuildTurnDetail(st *store.Store, cfg *pricing.Config, sessionID string, turn int) (TurnDetail, error) {
	events, err := st.EventsForSession(sessionID)
	if err != nil {
		return TurnDetail{}, err
	}
	var turns []schema.Event
	for _, e := range events {
		if isTurn(e) {
			turns = append(turns, e)
		}
	}
	sort.SliceStable(turns, func(i, j int) bool { return turns[i].At.Before(turns[j].At) })
	if turn < 1 || turn > len(turns) {
		return TurnDetail{}, ErrTurnNotFound
	}
	e := turns[turn-1]

	d := TurnDetail{
		SessionID: sessionID, Vendor: e.Vendor, Agent: e.Agent, Model: e.Model,
		Turn: turn, At: e.At.UTC().Format(time.RFC3339),
		Fresh: e.Input, Output: e.Output,
	}
	if e.CacheWrite != nil {
		d.CacheWrite = *e.CacheWrite
	}
	if e.CacheRead != nil {
		d.CacheRead = *e.CacheRead
	}
	if turn > 1 {
		d.HasGap = true
		d.GapSeconds = e.At.Sub(turns[turn-2].At).Seconds()
	}

	findings := insight.Analyze(turns, cfg)
	for _, f := range findings {
		if f.Turn == turn {
			d.Findings = append(d.Findings, f)
		}
	}

	calls, err := st.ToolCallsForTurn(e.Vendor, sessionID, e.RequestID)
	if err != nil {
		return TurnDetail{}, err
	}
	d.ToolCalls = calls
	seen := map[string]bool{}
	for _, c := range calls {
		if c.Path == "" || seen[c.Path] {
			continue
		}
		seen[c.Path] = true
		d.Files = append(d.Files, c.Path)
	}
	return d, nil
}

// BuildSessionInsight is bmSessionInsight's implementation (I3, Sessions
// tab): sessionID's every stored event, re-derived into turns and run
// through insight.Analyze exactly as BuildTurnDetail does for one turn, but
// returning every finding for the whole session. Computed fresh on every
// call, per the spec ("not stored"); the frontend is the one that caches it,
// in page memory for the tab's lifetime.
func BuildSessionInsight(st *store.Store, cfg *pricing.Config, sessionID string) ([]insight.Finding, error) {
	events, err := st.EventsForSession(sessionID)
	if err != nil {
		return nil, err
	}
	var turns []schema.Event
	for _, e := range events {
		if isTurn(e) {
			turns = append(turns, e)
		}
	}
	sort.SliceStable(turns, func(i, j int) bool { return turns[i].At.Before(turns[j].At) })
	return insight.Analyze(turns, cfg), nil
}

func firstNonEmptyProject(events []schema.Event) string {
	for _, e := range events {
		if e.Project != "" {
			return e.Project
		}
	}
	return ""
}

func firstNonEmptyClient(events []schema.Event) string {
	for _, e := range events {
		if e.Client != "" {
			return e.Client
		}
	}
	return ""
}

// headlineForEvents runs cfg.CostForEvents over events (all one vendor,
// since callers group by vendor+session first) and returns that one
// vendor's Headline, or nil for a vendor with no price book at all.
func headlineForEvents(events []schema.Event, cfg *pricing.Config) *pricing.BasisCost {
	vcs := cfg.CostForEvents(events)
	if len(vcs) == 0 {
		return nil
	}
	return vcs[0].Headline
}

// buildChart returns chartSlots (180) dense 10-second buckets covering
// [windowStart, windowStart+ChartWindow), every slot present and
// zero-valued when no turn landed in it (F3): the frontend must never
// derive a dense timeline from a sparse one, so this does that work once,
// here, rather than emitting only the minutes that had a turn. Buckets
// every real turn in the window by slot and by session, running or not: a
// session that just fell out of the running window a moment ago still
// belongs on the chart's tail.
func buildChart(events []schema.Event, cfg *pricing.Config, windowStart, now time.Time) []Bucket {
	slots := make([]Bucket, chartSlots)
	for i := range slots {
		slots[i].At = windowStart.Add(time.Duration(i) * BucketSeconds * time.Second).UTC().Format(time.RFC3339)
	}
	for _, e := range events {
		if !isTurn(e) || e.At.IsZero() || e.At.Before(windowStart) || e.At.After(now) {
			continue
		}
		idx := int(e.At.Sub(windowStart) / (BucketSeconds * time.Second))
		if idx < 0 || idx >= chartSlots {
			continue
		}
		b := &slots[idx]
		if b.BySession == nil {
			b.BySession = map[string]ClassCounts{}
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
	return slots
}

// ApplySessionTotals overwrites each running session's Start, TurnCount,
// Tokens and Cost with the store's SQL-aggregated lifetime totals (F1):
// BuildSnapshot itself only ever sees EventsSince(now-ChartWindow), so a
// session that has been running longer than ChartWindow would otherwise
// under-report its own tokens and cost. Recurses into Subagents. A session
// with no matching rows (should not happen for one BuildSnapshot just
// found running, but store access can race a session's very first turn) is
// left as BuildSnapshot computed it.
func ApplySessionTotals(sessions []*Session, st *store.Store, cfg *pricing.Config) error {
	keys := sessionKeys(sessions)
	if len(keys) == 0 {
		return nil
	}
	rows, err := st.SessionTotals(keys)
	if err != nil {
		return err
	}
	byID := map[string][]store.SessionModelTotal{}
	for _, r := range rows {
		k := r.Vendor + "|" + r.SessionID
		byID[k] = append(byID[k], r)
	}
	var apply func(s *Session)
	apply = func(s *Session) {
		if grp, ok := byID[s.Vendor+"|"+s.SessionID]; ok {
			var tokens, turns int64
			var cost float64
			var start time.Time
			// synth rebuilds one schema.Event per (vendor, session, model)
			// group from the SQL-aggregated totals, so BusinessCost below can
			// run the same cfg.CostForEvents every other page uses instead of
			// a second, dev-only cost formula: CostForEvents' per-basis
			// arithmetic is linear in each token class, so summing rates over
			// these aggregated totals gives the same lifetime figure as
			// summing over the individual events would.
			var synth []schema.Event
			for _, r := range grp {
				tokens += r.Input + r.CacheWrite + r.CacheRead + r.Output
				turns += r.Count
				if start.IsZero() || r.MinAt.Before(start) {
					start = r.MinAt
				}
				if r.Vendor == "openai" {
					c, _ := cfg.OpenAICallCostUSD(r.Model, r.Input, r.CacheRead, r.Output)
					cost += c
				} else {
					cost += cfg.CallCostSubUSD(cfg.ModelFamily(r.Model), r.Input, r.Output)
				}
				cw, cr := r.CacheWrite, r.CacheRead
				synth = append(synth, schema.Event{
					Vendor: r.Vendor, Model: r.Model,
					Input: r.Input, CacheWrite: &cw, CacheRead: &cr, Output: r.Output,
				})
			}
			s.Tokens = tokens
			s.Cost = cost
			s.BusinessCost = headlineForEvents(synth, cfg)
			s.TurnCount = int(turns)
			if !start.IsZero() {
				s.Start = start.UTC().Format(time.RFC3339)
			}
		}
		for _, sub := range s.Subagents {
			apply(sub)
		}
	}
	for _, s := range sessions {
		apply(s)
	}
	return nil
}

// sessionKeys collects every (vendor, session_id) pair in sessions,
// including subagents. A bare session_id is not unique across vendors, so
// SessionTotals is looked up by this vendor-qualified pair.
func sessionKeys(sessions []*Session) []store.SessionKey {
	var out []store.SessionKey
	var walk func(s *Session)
	walk = func(s *Session) {
		out = append(out, store.SessionKey{Vendor: s.Vendor, SessionID: s.SessionID})
		for _, sub := range s.Subagents {
			walk(sub)
		}
	}
	for _, s := range sessions {
		walk(s)
	}
	return out
}
