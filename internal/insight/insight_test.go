package insight

import (
	"fmt"
	"testing"
	"time"

	"burnmon/internal/pricing"
	"burnmon/internal/schema"
)

func testConfig() *pricing.Config {
	cfg := pricing.Defaults()
	return &cfg
}

func ptr(v int64) *int64 { return &v }

func ev(sessionID, requestID, model string, at time.Time, fresh int64, cacheW, cacheR *int64, out int64) schema.Event {
	return schema.Event{
		Vendor: "anthropic", Agent: "claude-code", Surface: "cli",
		SessionID: sessionID, RequestID: requestID, Model: model,
		At: at, Input: fresh, CacheWrite: cacheW, CacheRead: cacheR, Output: out,
	}
}

// TestAnalyze_Reprefill_CompactionCause: a compaction drop on turn 2, then a
// large cache write on turn 3, should be attributed to the compaction, the
// first cause in I2's fixed order.
func TestAnalyze_Reprefill_CompactionCause(t *testing.T) {
	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	events := []schema.Event{
		ev("s1", "r1", "claude-sonnet-5", base, 100_000, ptr(0), ptr(0), 500),
		ev("s1", "r2", "claude-sonnet-5", base.Add(1*time.Minute), 20_000, ptr(0), ptr(0), 500),
		ev("s1", "r3", "claude-sonnet-5", base.Add(2*time.Minute), 30_000, ptr(25_000), ptr(0), 500),
	}
	findings := Analyze(events, testConfig())

	var compactions, reprefills []Finding
	for _, f := range findings {
		switch f.Kind {
		case KindCompaction:
			compactions = append(compactions, f)
		case KindReprefill:
			reprefills = append(reprefills, f)
		}
	}
	if len(compactions) != 1 || compactions[0].Turn != 2 {
		t.Fatalf("want one compaction finding at turn 2, got %+v", compactions)
	}
	if len(reprefills) != 1 || reprefills[0].Turn != 3 {
		t.Fatalf("want one re-prefill finding at turn 3, got %+v", reprefills)
	}
	if reprefills[0].Cause != "compaction just happened" {
		t.Fatalf("want cause 'compaction just happened', got %q", reprefills[0].Cause)
	}
}

// TestAnalyze_Reprefill_ModelChangedCause: no compaction, but the model
// switched between turns 1 and 2, and turn 2 has a large cache write.
func TestAnalyze_Reprefill_ModelChangedCause(t *testing.T) {
	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	events := []schema.Event{
		ev("s1", "r1", "claude-sonnet-5", base, 5_000, ptr(0), ptr(0), 500),
		ev("s1", "r2", "claude-opus-5", base.Add(1*time.Minute), 30_000, ptr(25_000), ptr(0), 500),
	}
	findings := Analyze(events, testConfig())
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Kind != KindReprefill || f.Turn != 2 {
		t.Fatalf("want a re-prefill finding at turn 2, got %+v", f)
	}
	if f.Cause != "model changed since the previous turn" {
		t.Fatalf("want cause 'model changed since the previous turn', got %q", f.Cause)
	}
}

// TestAnalyze_Reprefill_GapCause: same model, no compaction, but the gap
// since the previous turn (90 minutes) exceeds the default 60-minute cache
// TTL, and turn 2 has a large cache write.
func TestAnalyze_Reprefill_GapCause(t *testing.T) {
	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	events := []schema.Event{
		ev("s1", "r1", "claude-sonnet-5", base, 5_000, ptr(0), ptr(0), 500),
		ev("s1", "r2", "claude-sonnet-5", base.Add(90*time.Minute), 30_000, ptr(25_000), ptr(0), 500),
	}
	findings := Analyze(events, testConfig())
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Kind != KindReprefill || f.Turn != 2 {
		t.Fatalf("want a re-prefill finding at turn 2, got %+v", f)
	}
	want := "gap 90 min, exceeds 60 min cache TTL"
	if f.Cause != want {
		t.Fatalf("want cause %q, got %q", want, f.Cause)
	}
}

// TestAnalyze_Reprefill_FirstTurnAfterResumeCause: the very first event we
// see for the session already has a large cache write, with no previous
// turn to compare against.
func TestAnalyze_Reprefill_FirstTurnAfterResumeCause(t *testing.T) {
	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	events := []schema.Event{
		ev("s1", "r1", "claude-sonnet-5", base, 30_000, ptr(25_000), ptr(0), 500),
	}
	findings := Analyze(events, testConfig())
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Kind != KindReprefill || f.Turn != 1 {
		t.Fatalf("want a re-prefill finding at turn 1, got %+v", f)
	}
	if f.Cause != "first turn after resume" {
		t.Fatalf("want cause 'first turn after resume', got %q", f.Cause)
	}
}

// TestAnalyze_Reprefill_UnknownCause: no compaction, same model, gap well
// inside the TTL, but the cache write is still above threshold.
func TestAnalyze_Reprefill_UnknownCause(t *testing.T) {
	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	events := []schema.Event{
		ev("s1", "r1", "claude-sonnet-5", base, 5_000, ptr(0), ptr(0), 500),
		ev("s1", "r2", "claude-sonnet-5", base.Add(1*time.Minute), 30_000, ptr(25_000), ptr(0), 500),
	}
	findings := Analyze(events, testConfig())
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Cause != "unknown" {
		t.Fatalf("want cause 'unknown', got %q", f.Cause)
	}
	if f.Confidence <= 0 || f.Confidence >= 1 {
		t.Fatalf("want a confidence strictly between 0 and 1, got %v", f.Confidence)
	}
}

// TestAnalyze_CompactionThreshold: an 20% context drop does not qualify (the
// I2 threshold is greater than 30%), a 40% drop does.
func TestAnalyze_CompactionThreshold(t *testing.T) {
	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	events := []schema.Event{
		ev("s1", "r1", "claude-sonnet-5", base, 100_000, ptr(0), ptr(0), 500),
		ev("s1", "r2", "claude-sonnet-5", base.Add(1*time.Minute), 80_000, ptr(0), ptr(0), 500), // 20% drop
		ev("s1", "r3", "claude-sonnet-5", base.Add(2*time.Minute), 48_000, ptr(0), ptr(0), 500), // 40% drop from turn 2
	}
	findings := Analyze(events, testConfig())
	var compactions []Finding
	for _, f := range findings {
		if f.Kind == KindCompaction {
			compactions = append(compactions, f)
		}
	}
	if len(compactions) != 1 || compactions[0].Turn != 3 {
		t.Fatalf("want one compaction finding at turn 3, got %+v", compactions)
	}
}

// TestAnalyze_NoFindings: a quiet session, every cache write under
// threshold, context growing steadily, triggers neither rule.
func TestAnalyze_NoFindings(t *testing.T) {
	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	events := []schema.Event{
		ev("s1", "r1", "claude-sonnet-5", base, 2_000, ptr(1_000), ptr(0), 300),
		ev("s1", "r2", "claude-sonnet-5", base.Add(1*time.Minute), 2_500, ptr(0), ptr(3_000), 300),
		ev("s1", "r3", "claude-sonnet-5", base.Add(2*time.Minute), 3_000, ptr(0), ptr(5_500), 300),
		ev("s1", "r4", "claude-sonnet-5", base.Add(3*time.Minute), 3_200, ptr(0), ptr(8_500), 300),
	}
	findings := Analyze(events, testConfig())
	if len(findings) != 0 {
		t.Fatalf("want no findings, got %+v", findings)
	}
}

// TestAnalyze_UnderFiveMillisecondsPerSession: bmLive runs Analyze once per
// running session on every 2-second poll (v0.2 I3), so a single session's
// worth of events must stay well inside the per-poll budget even on a long
// marathon session, not just a short fixture.
func TestAnalyze_UnderFiveMillisecondsPerSession(t *testing.T) {
	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	events := make([]schema.Event, 0, 2000)
	for i := 0; i < 2000; i++ {
		fresh := int64(1000 + i%500)
		cw := ptr(int64(500))
		if i%37 == 0 {
			cw = ptr(int64(25_000)) // occasional re-prefill
		}
		events = append(events, ev("marathon", fmt.Sprintf("r%d", i), "claude-sonnet-5",
			base.Add(time.Duration(i)*time.Minute), fresh, cw, ptr(int64(2000)), 300))
	}
	cfg := testConfig()

	start := time.Now()
	findings := Analyze(events, cfg)
	elapsed := time.Since(start)

	if elapsed > 5*time.Millisecond {
		t.Fatalf("Analyze took %s for %d events, want under 5ms", elapsed, len(events))
	}
	if len(findings) == 0 {
		t.Fatal("want at least one finding from the periodic re-prefills")
	}
}

// TestAnalyze_SyntheticToolOnlyEventsIgnored: a tool-only event (no model,
// no tokens) never counts as a turn, so it neither breaks turn numbering
// nor trips a false compaction drop.
func TestAnalyze_SyntheticToolOnlyEventsIgnored(t *testing.T) {
	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	events := []schema.Event{
		ev("s1", "r1", "claude-sonnet-5", base, 2_000, ptr(1_000), ptr(0), 300),
		{Vendor: "anthropic", Agent: "claude-code", Surface: "cli", SessionID: "s1", RequestID: "tool1", At: base.Add(30 * time.Second)},
		ev("s1", "r2", "claude-sonnet-5", base.Add(1*time.Minute), 2_500, ptr(0), ptr(3_000), 300),
	}
	findings := Analyze(events, testConfig())
	if len(findings) != 0 {
		t.Fatalf("want no findings, got %+v", findings)
	}
}
