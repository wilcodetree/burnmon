package advisor

import (
	"testing"
	"time"

	"burnmon/internal/live"
	"burnmon/internal/pricing"
	"burnmon/internal/sysmon"
)

func testConfig() *pricing.Config {
	cfg := pricing.Defaults()
	cfg.ContextWindows = map[string]int64{"claude-sonnet-5": 200_000}
	return &cfg
}

func turn(vendor, agent, sessionID, model string, turnN int, at time.Time, fresh, cw, cr, out int64) live.TurnEvent {
	return live.TurnEvent{
		Vendor: vendor, Agent: agent, SessionID: sessionID, Model: model,
		Turn: turnN, At: at.UTC().Format(time.RFC3339),
		Fresh: fresh, CacheWrite: cw, CacheRead: cr, Output: out,
	}
}

// sample builds a sysmon.Sample with every PressureScore input explicit
// (cpuQueue, diskLatMs, gpuPct), since the heavy-turn-pressure rule's tests
// need to push the composite score above or below its own threshold, not
// just CPUPct in isolation.
func sample(at time.Time, cpuPct, memUsed, memTotal, hardFaults, cpuQueue, diskLatMs, gpuPct float64) sysmon.Sample {
	return sysmon.Sample{
		Ts: at, CPUPct: cpuPct, Cores: []float64{cpuPct},
		MemUsedMB: memUsed, MemTotalMB: memTotal,
		CPUQueue: cpuQueue, CPUPerfPct: 100, DiskQueueLen: 0, DiskLatMs: diskLatMs,
		HardFaults: hardFaults, GPUPct: gpuPct,
	}
}

func findingIDs(fs []Finding) map[string]int {
	out := map[string]int{}
	for _, f := range fs {
		out[f.RuleID]++
	}
	return out
}

func TestAnalyze_HeavyTurnPressure_Fires(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	in := Input{
		Now:   base,
		Turns: []live.TurnEvent{turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 250_000, 0, 0, 1000)},
		// Every PressureScore component pushed high: composite score ~95.
		SysmonHistory: []sysmon.Sample{sample(base, 90, 15500, 16000, 0, 5, 25, 90)},
	}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	counts := findingIDs(findings)
	if counts["heavy-turn-pressure"] != 1 {
		t.Fatalf("expected 1 heavy-turn-pressure finding, got %d (all: %+v)", counts["heavy-turn-pressure"], findings)
	}
}

func TestAnalyze_HeavyTurnPressure_NoFireBelowTokenThreshold(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	in := Input{
		Now:           base,
		Turns:         []live.TurnEvent{turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 1000, 0, 0, 100)},
		SysmonHistory: []sysmon.Sample{sample(base, 90, 15500, 16000, 0, 5, 25, 90)},
	}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["heavy-turn-pressure"]; c != 0 {
		t.Fatalf("expected no heavy-turn-pressure finding for a small turn, got %d", c)
	}
}

func TestAnalyze_HeavyTurnPressure_NoFireWhenPressureLow(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	in := Input{
		Now:           base,
		Turns:         []live.TurnEvent{turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 250_000, 0, 0, 1000)},
		SysmonHistory: []sysmon.Sample{sample(base, 5, 2000, 16000, 0, 0, 0, 0)},
	}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["heavy-turn-pressure"]; c != 0 {
		t.Fatalf("expected no heavy-turn-pressure finding at low pressure, got %d", c)
	}
}

func TestAnalyze_HeavyTurnPressure_NoFireWhenSampleStale(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	in := Input{
		Now:   base,
		Turns: []live.TurnEvent{turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 250_000, 0, 0, 1000)},
		// Sample is 5 minutes away from the turn: too stale to be "at that moment".
		SysmonHistory: []sysmon.Sample{sample(base.Add(-5*time.Minute), 90, 15500, 16000, 0, 5, 25, 90)},
	}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["heavy-turn-pressure"]; c != 0 {
		t.Fatalf("expected no heavy-turn-pressure finding with a stale sample, got %d", c)
	}
}

// TestAnalyze_HeavyTurnPressure_CacheReadAloneIsNotHeavy guards the review
// fix, 2026-09-24: a turn dominated by cache read (replayed context, not
// new work) must not count as "heavy" on its own, or nearly every turn late
// in a long session would cross the threshold regardless of how much real
// work it did.
func TestAnalyze_HeavyTurnPressure_CacheReadAloneIsNotHeavy(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	in := Input{
		Now: base,
		// 5K fresh, 500K cache read, 1K output: turnTokens would cross
		// 200K, but heavyTurnWeight (fresh+cache_write+output) is only 6K.
		Turns:         []live.TurnEvent{turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 5_000, 0, 500_000, 1_000)},
		SysmonHistory: []sysmon.Sample{sample(base, 90, 15500, 16000, 0, 5, 25, 90)},
	}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["heavy-turn-pressure"]; c != 0 {
		t.Fatalf("expected no heavy-turn-pressure finding for a cache-read-dominated turn, got %d", c)
	}
}

// TestAnalyze_HeavyTurnPressure_CooldownCollapsesSustainedEpisode guards
// the review fix, 2026-09-24: a sustained high-pressure stretch spanning
// many heavy turns in the same session reports once per EpisodeCooldown,
// not once per turn.
func TestAnalyze_HeavyTurnPressure_CooldownCollapsesSustainedEpisode(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	var turns []live.TurnEvent
	var samples []sysmon.Sample
	for i := 0; i < 5; i++ {
		at := base.Add(time.Duration(i) * time.Minute)
		turns = append(turns, turn("anthropic", "claude-code", "s1", "claude-sonnet-5", i+1, at, 250_000, 0, 0, 1000))
		samples = append(samples, sample(at, 90, 15500, 16000, 0, 5, 25, 90))
	}
	in := Input{Now: base.Add(5 * time.Minute), Turns: turns, SysmonHistory: samples}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["heavy-turn-pressure"]; c != 1 {
		t.Fatalf("expected the 5-minute episode (5 turns, 1 minute apart, cooldown 5 minutes) to collapse to 1 finding, got %d", c)
	}
}

// TestAnalyze_HeavyTurnPressure_ReFiresAfterCooldown guards that a second,
// later episode in the same session still reports once the cooldown has
// elapsed, rather than being silently absorbed by the first episode's cooldown forever.
func TestAnalyze_HeavyTurnPressure_ReFiresAfterCooldown(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	turns := []live.TurnEvent{
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 250_000, 0, 0, 1000),
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 2, base.Add(10*time.Minute), 250_000, 0, 0, 1000),
	}
	samples := []sysmon.Sample{
		sample(base, 90, 15500, 16000, 0, 5, 25, 90),
		sample(base.Add(10*time.Minute), 90, 15500, 16000, 0, 5, 25, 90),
	}
	in := Input{Now: base.Add(10 * time.Minute), Turns: turns, SysmonHistory: samples}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["heavy-turn-pressure"]; c != 2 {
		t.Fatalf("expected both episodes (10 minutes apart, past the 5-minute cooldown) to report, got %d", c)
	}
}

func TestAnalyze_HarnessRunaway_FiresWithNoTurns(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	var groups []sysmon.ProcessGroupSample
	for i := 0; i < 5; i++ {
		groups = append(groups, sysmon.ProcessGroupSample{
			Ts: base.Add(time.Duration(i) * time.Second), Harness: sysmon.HarnessCodex, CPUPct: 40, MemMB: 500,
		})
	}
	in := Input{Now: base, ProcessGroups: groups}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["harness-runaway"]; c != 1 {
		t.Fatalf("expected 1 harness-runaway finding, got %d (all: %+v)", c, findings)
	}
}

func TestAnalyze_HarnessRunaway_NoFireWithTurns(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	var groups []sysmon.ProcessGroupSample
	for i := 0; i < 5; i++ {
		groups = append(groups, sysmon.ProcessGroupSample{
			Ts: base.Add(time.Duration(i) * time.Second), Harness: sysmon.HarnessCodex, CPUPct: 40, MemMB: 500,
		})
	}
	in := Input{
		Now:           base,
		Turns:         []live.TurnEvent{turn("openai", "codex", "s1", "gpt-5", 1, base, 1000, 0, 0, 100)},
		ProcessGroups: groups,
	}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["harness-runaway"]; c != 0 {
		t.Fatalf("expected no harness-runaway finding once turns exist for that harness, got %d", c)
	}
}

func TestAnalyze_HarnessRunaway_NoFireBelowCPUThreshold(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	var groups []sysmon.ProcessGroupSample
	for i := 0; i < 5; i++ {
		groups = append(groups, sysmon.ProcessGroupSample{
			Ts: base.Add(time.Duration(i) * time.Second), Harness: sysmon.HarnessCodex, CPUPct: 2, MemMB: 500,
		})
	}
	in := Input{Now: base, ProcessGroups: groups}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["harness-runaway"]; c != 0 {
		t.Fatalf("expected no harness-runaway finding below the CPU threshold, got %d", c)
	}
}

func TestAnalyze_HarnessRunaway_NoFireWithTooFewSamples(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	groups := []sysmon.ProcessGroupSample{
		{Ts: base, Harness: sysmon.HarnessCodex, CPUPct: 90, MemMB: 500},
	}
	in := Input{Now: base, ProcessGroups: groups}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["harness-runaway"]; c != 0 {
		t.Fatalf("expected no harness-runaway finding with only 1 sample, got %d", c)
	}
}

func TestAnalyze_CacheHitDrop_Fires(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	turns := []live.TurnEvent{
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 1000, 0, 9000, 500),
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 2, base.Add(1*time.Minute), 1000, 0, 9000, 500),
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 3, base.Add(2*time.Minute), 9000, 0, 1000, 500),
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 4, base.Add(3*time.Minute), 9000, 0, 1000, 500),
	}
	in := Input{Now: base, Turns: turns}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["cache-hit-drop"]; c != 1 {
		t.Fatalf("expected 1 cache-hit-drop finding, got %d (all: %+v)", c, findings)
	}
}

func TestAnalyze_CacheHitDrop_NoFireWhenStable(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	turns := []live.TurnEvent{
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 1000, 0, 9000, 500),
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 2, base.Add(1*time.Minute), 1000, 0, 9000, 500),
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 3, base.Add(2*time.Minute), 1000, 0, 9000, 500),
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 4, base.Add(3*time.Minute), 1000, 0, 9000, 500),
	}
	in := Input{Now: base, Turns: turns}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["cache-hit-drop"]; c != 0 {
		t.Fatalf("expected no cache-hit-drop finding with a stable hit rate, got %d", c)
	}
}

func TestAnalyze_CacheHitDrop_NoFireWithTooFewTurns(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	turns := []live.TurnEvent{
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 1000, 0, 9000, 500),
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 2, base.Add(1*time.Minute), 9000, 0, 1000, 500),
	}
	in := Input{Now: base, Turns: turns}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["cache-hit-drop"]; c != 0 {
		t.Fatalf("expected no cache-hit-drop finding with only 2 turns, got %d", c)
	}
}

func TestAnalyze_ContextWindow_Fires(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	turns := []live.TurnEvent{
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 170_000, 0, 0, 500),
	}
	in := Input{Now: base, Turns: turns}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["context-window"]; c != 1 {
		t.Fatalf("expected 1 context-window finding (170k/200k = 85%%), got %d (all: %+v)", c, findings)
	}
}

func TestAnalyze_ContextWindow_NoFireBelowThreshold(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	turns := []live.TurnEvent{
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 50_000, 0, 0, 500),
	}
	in := Input{Now: base, Turns: turns}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["context-window"]; c != 0 {
		t.Fatalf("expected no context-window finding at 25%%, got %d", c)
	}
}

func TestAnalyze_ContextWindow_NoFireForUnknownModel(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	turns := []live.TurnEvent{
		turn("openai", "codex", "s1", "some-unknown-model", 1, base, 500_000, 0, 0, 500),
	}
	in := Input{Now: base, Turns: turns}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["context-window"]; c != 0 {
		t.Fatalf("expected no context-window finding for a model with no known window, got %d", c)
	}
}

func TestAnalyze_ContextWindow_UsesLatestTurnPerSession(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	turns := []live.TurnEvent{
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 190_000, 0, 0, 500),
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 2, base.Add(time.Minute), 10_000, 0, 0, 500),
	}
	in := Input{Now: base, Turns: turns}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["context-window"]; c != 0 {
		t.Fatalf("expected no context-window finding: the session's latest turn dropped back to 5%%, got %d", c)
	}
}

func TestAnalyze_HardFaultsMemory_FiresOnHardFaults(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	in := Input{
		Now:           base,
		Turns:         []live.TurnEvent{turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 1000, 0, 0, 500)},
		SysmonHistory: []sysmon.Sample{sample(base, 30, 4000, 16000, 900, 0, 0, 0)},
	}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["hard-faults-memory"]; c != 1 {
		t.Fatalf("expected 1 hard-faults-memory finding, got %d (all: %+v)", c, findings)
	}
}

func TestAnalyze_HardFaultsMemory_FiresOnMemoryPressure(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	in := Input{
		Now:           base,
		Turns:         []live.TurnEvent{turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 1000, 0, 0, 500)},
		SysmonHistory: []sysmon.Sample{sample(base, 30, 15500, 16000, 0, 0, 0, 0)},
	}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["hard-faults-memory"]; c != 1 {
		t.Fatalf("expected 1 hard-faults-memory finding on memory pressure, got %d (all: %+v)", c, findings)
	}
}

func TestAnalyze_HardFaultsMemory_NoFireWhenCalm(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	in := Input{
		Now:           base,
		Turns:         []live.TurnEvent{turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 1000, 0, 0, 500)},
		SysmonHistory: []sysmon.Sample{sample(base, 10, 4000, 16000, 0, 0, 0, 0)},
	}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["hard-faults-memory"]; c != 0 {
		t.Fatalf("expected no hard-faults-memory finding when calm, got %d", c)
	}
}

// TestAnalyze_HardFaultsMemory_CooldownCollapsesSustainedEpisode mirrors
// heavy-turn-pressure's own cooldown test: a sustained memory-pressure
// stretch across many turns in one session reports once per
// EpisodeCooldown, not once per turn.
func TestAnalyze_HardFaultsMemory_CooldownCollapsesSustainedEpisode(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	var turns []live.TurnEvent
	var samples []sysmon.Sample
	for i := 0; i < 5; i++ {
		at := base.Add(time.Duration(i) * time.Minute)
		turns = append(turns, turn("anthropic", "claude-code", "s1", "claude-sonnet-5", i+1, at, 1000, 0, 0, 500))
		samples = append(samples, sample(at, 30, 15500, 16000, 0, 0, 0, 0))
	}
	in := Input{Now: base.Add(5 * time.Minute), Turns: turns, SysmonHistory: samples}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["hard-faults-memory"]; c != 1 {
		t.Fatalf("expected the 5-minute episode to collapse to 1 finding, got %d", c)
	}
}

func TestAnalyze_SelfOverhead_FiresOverCPUBudget(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	var groups []sysmon.ProcessGroupSample
	for i := 0; i < 5; i++ {
		groups = append(groups, sysmon.ProcessGroupSample{
			Ts: base.Add(time.Duration(i) * time.Second), Harness: sysmon.HarnessSelf, CPUPct: 5, MemMB: 100,
		})
	}
	in := Input{Now: base, ProcessGroups: groups}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["self-overhead"]; c != 1 {
		t.Fatalf("expected 1 self-overhead finding, got %d (all: %+v)", c, findings)
	}
}

func TestAnalyze_SelfOverhead_FiresOverMemBudget(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	var groups []sysmon.ProcessGroupSample
	for i := 0; i < 5; i++ {
		groups = append(groups, sysmon.ProcessGroupSample{
			Ts: base.Add(time.Duration(i) * time.Second), Harness: sysmon.HarnessSelf, CPUPct: 0.5, MemMB: 400,
		})
	}
	in := Input{Now: base, ProcessGroups: groups}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["self-overhead"]; c != 1 {
		t.Fatalf("expected 1 self-overhead finding on memory, got %d (all: %+v)", c, findings)
	}
}

// TestAnalyze_SelfOverhead_CombinesGoAndWebviewRows guards a regression a
// fresh review caught (2026-09-26): WS2 item 4 split what used to be one
// summed HarnessSelf process-groups row into HarnessSelf (the Go process)
// and HarnessSelfWebview (its WebView2 host), and evalSelfOverhead still
// only read HarnessSelf, silently dropping WebView2's own share - the
// larger half of this app's real footprint. Neither harness alone crosses
// SelfCPUPct (2%) here, but their sum (2.5%) should.
func TestAnalyze_SelfOverhead_CombinesGoAndWebviewRows(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	var groups []sysmon.ProcessGroupSample
	for i := 0; i < 5; i++ {
		ts := base.Add(time.Duration(i) * time.Second)
		groups = append(groups,
			sysmon.ProcessGroupSample{Ts: ts, Harness: sysmon.HarnessSelf, CPUPct: 1, MemMB: 50},
			sysmon.ProcessGroupSample{Ts: ts, Harness: sysmon.HarnessSelfWebview, CPUPct: 1.5, MemMB: 60},
		)
	}
	in := Input{Now: base, ProcessGroups: groups}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["self-overhead"]; c != 1 {
		t.Fatalf("expected 1 self-overhead finding from the combined Go+WebView2 CPU (1+1.5=2.5%%, over the 2%% budget), got %d (all: %+v)", c, findings)
	}
}

func TestAnalyze_SelfOverhead_NoFireWithinBudget(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	var groups []sysmon.ProcessGroupSample
	for i := 0; i < 5; i++ {
		groups = append(groups, sysmon.ProcessGroupSample{
			Ts: base.Add(time.Duration(i) * time.Second), Harness: sysmon.HarnessSelf, CPUPct: 0.5, MemMB: 100,
		})
	}
	in := Input{Now: base, ProcessGroups: groups}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if c := findingIDs(findings)["self-overhead"]; c != 0 {
		t.Fatalf("expected no self-overhead finding within budget, got %d", c)
	}
}

func TestAnalyze_SortedNewestFirst(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	turns := []live.TurnEvent{
		turn("anthropic", "claude-code", "s1", "claude-sonnet-5", 1, base, 250_000, 0, 0, 1000),
		turn("anthropic", "claude-code", "s2", "claude-sonnet-5", 1, base.Add(10*time.Minute), 250_000, 0, 0, 1000),
	}
	in := Input{
		Now:   base.Add(10 * time.Minute),
		Turns: turns,
		SysmonHistory: []sysmon.Sample{
			sample(base, 90, 15500, 16000, 0, 5, 25, 90),
			sample(base.Add(10*time.Minute), 90, 15500, 16000, 0, 5, 25, 90),
		},
	}
	findings := Analyze(in, testConfig(), DefaultThresholds)
	if len(findings) < 2 {
		t.Fatalf("expected at least 2 findings, got %d", len(findings))
	}
	for i := 1; i < len(findings); i++ {
		if findings[i].At.After(findings[i-1].At) {
			t.Fatalf("findings not sorted newest-first: %v before %v", findings[i-1].At, findings[i].At)
		}
	}
}

func TestAnalyze_EmptyInputNoPanic(t *testing.T) {
	findings := Analyze(Input{Now: time.Now()}, testConfig(), DefaultThresholds)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for empty input, got %d", len(findings))
	}
}
