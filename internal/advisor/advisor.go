// Package advisor is BurnMon Dev's "TODAY'S READ" rule engine (design doc
// section 3, phase 4): a fixed table of rules, each combining burn data
// (live.TurnEvent, already carrying its own insight.Finding) with system
// data (internal/sysmon samples and process-group samples) into one
// Finding with a time, evidence numbers and one concrete suggestion.
//
// Analyze is a pure function: every rule reads only its Input argument plus
// cfg and Thresholds, no store or sysmon access happens in this package. The
// binding (cmd/burnmon-dev/main.go's bdevAdvisorNow) and the export bundle
// (internal/devexport) are the only callers that touch a real store, off
// the UI thread, and both build one Input and call Analyze.
package advisor

import (
	"fmt"
	"math"
	"sort"
	"time"

	"burnmon/internal/live"
	"burnmon/internal/pricing"
	"burnmon/internal/sysmon"
)

// Finding is one thing advisor noticed, combining burn and system data.
type Finding struct {
	RuleID     string             `json:"rule_id"`
	At         time.Time          `json:"at"`
	Message    string             `json:"message"`
	Evidence   map[string]float64 `json:"evidence,omitempty"`
	Suggestion string             `json:"suggestion"`
}

// Thresholds holds every rule's tunable numbers in one place, so a later
// session recalibrating one rule edits one struct, not six functions.
type Thresholds struct {
	// heavy-turn-pressure: a turn whose own token total is at least this
	// many tokens, landing while the nearest system sample's pressure score
	// is at least HeavyTurnPressure.
	HeavyTurnTokens   int64
	HeavyTurnPressure int

	// harness-runaway: a harness's process-group samples averaging at least
	// this much CPU percent, with at least RunawayMinSamples samples, and
	// zero turns from that harness's own agent(s) in the same Input.
	// CPUPct here is gopsutil's own Percent, summed across every process in
	// the group (sysmon.ProcessSampler.Tick), so 100 means one full core
	// saturated, not "the whole machine": on an 8-core box a harness with
	// several busy child processes can read well over 100.
	RunawayCPUPct     float64
	RunawayMinSamples int

	// cache-hit-drop: a session's own cache-hit ratio (cache_read / (fresh +
	// cache_read), per turn) falling by at least this many percentage
	// points between its first half of turns and its second half. Needs at
	// least CacheHitMinTurns turns to split into two meaningful halves.
	CacheHitDropPoints float64
	CacheHitMinTurns   int

	// context-window: a session's latest turn's context (fresh + cache
	// write + cache read) at or above this fraction of its model's context
	// window.
	ContextWindowRatio float64

	// hard-faults-memory: a turn landing while the nearest system sample
	// shows hard faults (Memory\Pages Input/sec) at or above
	// HardFaultsPerSec, or memory used at or above MemPressureRatio of
	// total.
	HardFaultsPerSec float64
	MemPressureRatio float64

	// self-overhead: BurnMon Dev's own process group (sysmon.HarnessSelf)
	// averaging CPU or RAM at or above these numbers, over at least
	// RunawayMinSamples samples (shared with harness-runaway: both are a
	// "sustained load" read over the same process-group sample table).
	// SelfCPUPct is the same summed-across-processes percent RunawayCPUPct
	// is, not a single-core reading.
	SelfCPUPct float64
	SelfMemMB  float64

	// nearestSampleMaxStaleness bounds how far a turn's timestamp may sit
	// from the nearest system sample before that sample is not treated as
	// "at that moment" (matches page.html's own CPU_LINE_MAX_STALENESS_MS).
	NearestSampleMaxStaleness time.Duration

	// EpisodeCooldown bounds how often heavy-turn-pressure and
	// hard-faults-memory re-fire for the same session: a sustained
	// pressure or memory episode spanning many turns reports once per
	// cooldown window, not once per matching turn (found by review,
	// 2026-09-24: without this, one 10-minute pressure spike during a busy
	// session could fill the whole panel with near-duplicate findings).
	EpisodeCooldown time.Duration
}

// DefaultThresholds is what the live binding and the export bundle both use
// unless a caller (a test) overrides them.
var DefaultThresholds = Thresholds{
	HeavyTurnTokens:           200_000,
	HeavyTurnPressure:         70,
	RunawayCPUPct:             20,
	RunawayMinSamples:         3,
	CacheHitDropPoints:        25,
	CacheHitMinTurns:          4,
	ContextWindowRatio:        0.8,
	HardFaultsPerSec:          500,
	MemPressureRatio:          0.9,
	SelfCPUPct:                2,
	SelfMemMB:                 250,
	NearestSampleMaxStaleness: 60 * time.Second,
	EpisodeCooldown:           5 * time.Minute,
}

// Input is everything one Analyze call needs, already loaded by the caller.
type Input struct {
	Now           time.Time
	Turns         []live.TurnEvent
	SysmonHistory []sysmon.Sample
	ProcessGroups []sysmon.ProcessGroupSample
}

type ruleFunc func(Input, *pricing.Config, Thresholds) []Finding

type rule struct {
	ID   string
	Eval ruleFunc
}

// rules is the table: one row per rule, in the order Analyze reports ties.
// Every threshold a rule needs lives in Thresholds above, not here, so this
// table stays a pure list of (id, function) pairs.
var rules = []rule{
	{"heavy-turn-pressure", evalHeavyTurnPressure},
	{"harness-runaway", evalHarnessRunaway},
	{"cache-hit-drop", evalCacheHitDrop},
	{"context-window", evalContextWindow},
	{"hard-faults-memory", evalHardFaultsMemory},
	{"self-overhead", evalSelfOverhead},
}

// Analyze runs every rule over in and returns every Finding, newest first.
func Analyze(in Input, cfg *pricing.Config, th Thresholds) []Finding {
	var out []Finding
	for _, r := range rules {
		out = append(out, r.Eval(in, cfg, th)...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}

// turnTokens is one turn's whole token count across every class.
func turnTokens(t live.TurnEvent) int64 {
	return t.Fresh + t.CacheWrite + t.CacheRead + t.Output
}

// heavyTurnWeight is heavy-turn-pressure's own measure of "heavy": fresh
// input, cache write and output, deliberately excluding cache read. A
// cache read replays context the model already holds; late in almost any
// real session it alone can exceed 200K tokens, which made turnTokens
// (including cache read) cross HeavyTurnTokens on nearly every turn once a
// session ran long, regardless of how much new work that turn actually did
// (found by review, 2026-09-24).
func heavyTurnWeight(t live.TurnEvent) int64 {
	return t.Fresh + t.CacheWrite + t.Output
}

// sortedByTime returns a copy of turns ordered oldest-first: in.Turns
// itself (from live.BuildTurns) is newest-first, but the per-turn rules'
// own cooldown collapsing (evalHeavyTurnPressure, evalHardFaultsMemory)
// needs to walk a session's turns forward in time to tell a fresh episode
// from a continuation of the last one it already reported.
func sortedByTime(turns []live.TurnEvent) []live.TurnEvent {
	out := append([]live.TurnEvent(nil), turns...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

// nearestSample returns the sysmon.Sample in history closest to at, or
// ok=false when history is empty or the closest sample is farther away than
// maxStaleness (an empty history just after startup, or a gap after the
// app was suspended, is not a real reading for that moment).
func nearestSample(history []sysmon.Sample, at time.Time, maxStaleness time.Duration) (sysmon.Sample, bool) {
	var best sysmon.Sample
	bestDiff := time.Duration(math.MaxInt64)
	found := false
	for _, s := range history {
		diff := at.Sub(s.Ts)
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			bestDiff = diff
			best = s
			found = true
		}
	}
	if !found || bestDiff > maxStaleness {
		return sysmon.Sample{}, false
	}
	return best, true
}

// fmtTok renders a token count the same compact way page.html's fmtTokens
// does, so a Finding's Message reads consistently with the rest of the UI.
func fmtTok(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.0fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// ---------------------------------------------------------------------------
// Rule 1: heavy-turn-pressure
// ---------------------------------------------------------------------------

func evalHeavyTurnPressure(in Input, _ *pricing.Config, th Thresholds) []Finding {
	turns := sortedByTime(in.Turns)
	lastFireAt := map[string]time.Time{} // session key -> last finding's turn time
	var out []Finding
	for _, t := range turns {
		if heavyTurnWeight(t) < th.HeavyTurnTokens {
			continue
		}
		at, err := time.Parse(time.RFC3339, t.At)
		if err != nil {
			continue
		}
		key := t.Vendor + "|" + t.SessionID
		if last, ok := lastFireAt[key]; ok && at.Sub(last) < th.EpisodeCooldown {
			continue
		}
		sample, ok := nearestSample(in.SysmonHistory, at, th.NearestSampleMaxStaleness)
		if !ok {
			continue
		}
		pressure := sysmon.PressureScore(sample)
		if pressure < th.HeavyTurnPressure {
			continue
		}
		lastFireAt[key] = at
		tokens := turnTokens(t)
		out = append(out, Finding{
			RuleID: "heavy-turn-pressure",
			At:     at,
			Message: fmt.Sprintf("%s turn %d at %s: %s tokens while system pressure was %d",
				t.Agent, t.Turn, at.Format("15:04:05"), fmtTok(tokens), pressure),
			Evidence: map[string]float64{
				"tokens":             float64(tokens),
				"fresh_write_output": float64(heavyTurnWeight(t)),
				"pressure_score":     float64(pressure),
				"cpu_pct":            sample.CPUPct,
			},
			Suggestion: "wait for system pressure to drop before starting another heavy turn on this harness, or move it to a quieter window",
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// Rule 2: harness-runaway
// ---------------------------------------------------------------------------

// harnessAgents maps a sysmon.Harness to the schema.Event.Agent value(s)
// (via live.TurnEvent.Agent) its turns would carry, so this rule can tell
// "sustained CPU, but this harness's own agent had zero turns in the same
// window" apart from a harness that is simply busy running real turns.
var harnessAgents = map[sysmon.Harness][]string{
	sysmon.HarnessClaude:        {"claude-code"},
	sysmon.HarnessClaudeDesktop: {"cowork"},
	sysmon.HarnessCodex:         {"codex"},
	sysmon.HarnessCopilotCLI:    {"copilot-cli"},
	sysmon.HarnessCopilotVSCode: {"copilot-vscode"},
	sysmon.HarnessHermes:        {"hermes"},
}

// runawayHarnessOrder is harnessAgents' keys in a fixed order, so ties (two
// harnesses both idle-but-busy at the same avg CPU) report deterministically
// rather than following Go's randomized map iteration.
var runawayHarnessOrder = []sysmon.Harness{
	sysmon.HarnessClaude, sysmon.HarnessClaudeDesktop, sysmon.HarnessCodex,
	sysmon.HarnessCopilotCLI, sysmon.HarnessCopilotVSCode, sysmon.HarnessHermes,
}

func evalHarnessRunaway(in Input, _ *pricing.Config, th Thresholds) []Finding {
	byHarness := map[sysmon.Harness][]sysmon.ProcessGroupSample{}
	for _, s := range in.ProcessGroups {
		byHarness[s.Harness] = append(byHarness[s.Harness], s)
	}

	turnAgents := map[string]bool{}
	for _, t := range in.Turns {
		turnAgents[t.Agent] = true
	}

	var out []Finding
	for _, h := range runawayHarnessOrder {
		samples := byHarness[h]
		if len(samples) < th.RunawayMinSamples {
			continue
		}
		agents := harnessAgents[h]
		hasTurns := false
		for _, a := range agents {
			if turnAgents[a] {
				hasTurns = true
				break
			}
		}
		if hasTurns {
			continue
		}

		var sumCPU, sumMem float64
		var last time.Time
		for _, s := range samples {
			sumCPU += s.CPUPct
			sumMem += s.MemMB
			if s.Ts.After(last) {
				last = s.Ts
			}
		}
		avgCPU := sumCPU / float64(len(samples))
		if avgCPU < th.RunawayCPUPct {
			continue
		}
		avgMem := sumMem / float64(len(samples))
		out = append(out, Finding{
			RuleID: "harness-runaway",
			At:     last,
			Message: fmt.Sprintf("%s used %.0f%% CPU on average across %d samples with no turns recorded in the same window",
				h, avgCPU, len(samples)),
			Evidence: map[string]float64{
				"avg_cpu_pct": avgCPU,
				"avg_mem_mb":  avgMem,
				"samples":     float64(len(samples)),
			},
			Suggestion: fmt.Sprintf("check what %s's own process is doing: a stuck subprocess or background indexer left running is a likely idle-cost cause, not billed tokens", h),
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// Rule 3: cache-hit-drop
// ---------------------------------------------------------------------------

func evalCacheHitDrop(in Input, _ *pricing.Config, th Thresholds) []Finding {
	bySession := map[string][]live.TurnEvent{}
	var order []string
	for _, t := range in.Turns {
		key := t.Vendor + "|" + t.SessionID
		if _, ok := bySession[key]; !ok {
			order = append(order, key)
		}
		bySession[key] = append(bySession[key], t)
	}

	var out []Finding
	for _, key := range order {
		turns := bySession[key]
		if len(turns) < th.CacheHitMinTurns {
			continue
		}
		sort.SliceStable(turns, func(i, j int) bool { return turns[i].At < turns[j].At })

		hitRatio := func(t live.TurnEvent) (float64, bool) {
			denom := t.Fresh + t.CacheRead
			if denom <= 0 {
				return 0, false
			}
			return float64(t.CacheRead) / float64(denom), true
		}

		half := len(turns) / 2
		firstAvg, firstN := 0.0, 0
		for _, t := range turns[:half] {
			if r, ok := hitRatio(t); ok {
				firstAvg += r
				firstN++
			}
		}
		secondAvg, secondN := 0.0, 0
		for _, t := range turns[half:] {
			if r, ok := hitRatio(t); ok {
				secondAvg += r
				secondN++
			}
		}
		if firstN == 0 || secondN == 0 {
			continue
		}
		firstAvg /= float64(firstN)
		secondAvg /= float64(secondN)
		dropPoints := (firstAvg - secondAvg) * 100
		if dropPoints < th.CacheHitDropPoints {
			continue
		}

		last := turns[len(turns)-1]
		at, err := time.Parse(time.RFC3339, last.At)
		if err != nil {
			at = in.Now
		}
		out = append(out, Finding{
			RuleID: "cache-hit-drop",
			At:     at,
			Message: fmt.Sprintf("%s session's cache hit rate dropped from %.0f%% to %.0f%% over %d turns",
				last.Agent, firstAvg*100, secondAvg*100, len(turns)),
			Evidence: map[string]float64{
				"first_half_hit_rate":  firstAvg,
				"second_half_hit_rate": secondAvg,
				"drop_points":          dropPoints,
				"turns":                float64(len(turns)),
			},
			Suggestion: "a dropping cache hit rate usually means the session's prompt prefix keeps changing (a growing system prompt, a resorted tool list); keep the prefix stable across turns to keep cache reads cheap",
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// Rule 4: context-window
// ---------------------------------------------------------------------------

func evalContextWindow(in Input, cfg *pricing.Config, th Thresholds) []Finding {
	type latest struct {
		turn live.TurnEvent
		at   time.Time
	}
	bySession := map[string]latest{}
	var order []string
	for _, t := range in.Turns {
		at, err := time.Parse(time.RFC3339, t.At)
		if err != nil {
			continue
		}
		key := t.Vendor + "|" + t.SessionID
		cur, ok := bySession[key]
		if !ok {
			order = append(order, key)
		}
		if !ok || at.After(cur.at) {
			bySession[key] = latest{turn: t, at: at}
		}
	}

	var out []Finding
	for _, key := range order {
		l := bySession[key]
		if cfg == nil {
			continue
		}
		window, ok := cfg.ContextWindow(l.turn.Model)
		if !ok || window <= 0 {
			continue
		}
		context := l.turn.Fresh + l.turn.CacheWrite + l.turn.CacheRead
		ratio := float64(context) / float64(window)
		if ratio < th.ContextWindowRatio {
			continue
		}
		out = append(out, Finding{
			RuleID: "context-window",
			At:     l.at,
			Message: fmt.Sprintf("%s session on %s is at %.0f%% of its %s context window (%s of %s tokens)",
				l.turn.Agent, l.turn.Model, ratio*100, l.turn.Model, fmtTok(context), fmtTok(window)),
			Evidence: map[string]float64{
				"context":        float64(context),
				"context_window": float64(window),
				"ratio":          ratio,
			},
			Suggestion: "start a new session (or trigger a compaction) soon: a session this close to its context window re-sends a growing prefix every turn, so cost per turn keeps climbing",
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// Rule 5: hard-faults-memory
// ---------------------------------------------------------------------------

func evalHardFaultsMemory(in Input, _ *pricing.Config, th Thresholds) []Finding {
	turns := sortedByTime(in.Turns)
	lastFireAt := map[string]time.Time{} // session key -> last finding's turn time
	var out []Finding
	for _, t := range turns {
		at, err := time.Parse(time.RFC3339, t.At)
		if err != nil {
			continue
		}
		key := t.Vendor + "|" + t.SessionID
		if last, ok := lastFireAt[key]; ok && at.Sub(last) < th.EpisodeCooldown {
			continue
		}
		sample, ok := nearestSample(in.SysmonHistory, at, th.NearestSampleMaxStaleness)
		if !ok {
			continue
		}
		hardFaulting := sample.HardFaults >= 0 && sample.HardFaults >= th.HardFaultsPerSec
		var memRatio float64
		memPressured := false
		if sample.MemTotalMB > 0 {
			memRatio = sample.MemUsedMB / sample.MemTotalMB
			memPressured = memRatio >= th.MemPressureRatio
		}
		if !hardFaulting && !memPressured {
			continue
		}
		lastFireAt[key] = at
		var reasons []string
		if hardFaulting {
			reasons = append(reasons, fmt.Sprintf("%.0f hard faults/sec", sample.HardFaults))
		}
		if memPressured {
			reasons = append(reasons, fmt.Sprintf("memory at %.0f%%", memRatio*100))
		}
		out = append(out, Finding{
			RuleID: "hard-faults-memory",
			At:     at,
			Message: fmt.Sprintf("%s turn %d at %s ran while the system was under memory pressure: %s",
				t.Agent, t.Turn, at.Format("15:04:05"), joinReasons(reasons)),
			Evidence: map[string]float64{
				"hard_faults_per_sec": sample.HardFaults,
				"mem_used_mb":         sample.MemUsedMB,
				"mem_total_mb":        sample.MemTotalMB,
				"mem_ratio":           memRatio,
			},
			Suggestion: "close unused harness windows or browser tabs before the next heavy turn; a turn issued under real memory pressure runs slower and can stall the harness's own tool calls",
		})
	}
	return out
}

func joinReasons(reasons []string) string {
	out := ""
	for i, r := range reasons {
		if i > 0 {
			out += ", "
		}
		out += r
	}
	return out
}

// ---------------------------------------------------------------------------
// Rule 6: self-overhead
// ---------------------------------------------------------------------------

func evalSelfOverhead(in Input, _ *pricing.Config, th Thresholds) []Finding {
	var samples []sysmon.ProcessGroupSample
	for _, s := range in.ProcessGroups {
		if s.Harness == sysmon.HarnessSelf {
			samples = append(samples, s)
		}
	}
	if len(samples) < th.RunawayMinSamples {
		return nil
	}
	var sumCPU, sumMem float64
	var last time.Time
	for _, s := range samples {
		sumCPU += s.CPUPct
		sumMem += s.MemMB
		if s.Ts.After(last) {
			last = s.Ts
		}
	}
	avgCPU := sumCPU / float64(len(samples))
	avgMem := sumMem / float64(len(samples))
	if avgCPU < th.SelfCPUPct && avgMem < th.SelfMemMB {
		return nil
	}
	return []Finding{{
		RuleID: "self-overhead",
		At:     last,
		Message: fmt.Sprintf("BurnMon Dev itself is averaging %.1f%% CPU and %.0f MB RAM across %d samples, over its own %.0f%%/%.0f MB budget",
			avgCPU, avgMem, len(samples), th.SelfCPUPct, th.SelfMemMB),
		Evidence: map[string]float64{
			"avg_cpu_pct": avgCPU,
			"avg_mem_mb":  avgMem,
			"samples":     float64(len(samples)),
		},
		Suggestion: "restart BurnMon Dev to release accumulated memory; if this keeps recurring, it belongs in the shared ingest bug already tracked on main, not this session's own findings",
	}}
}
