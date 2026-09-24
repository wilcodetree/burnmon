package live

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"burnmon/internal/adapter/claude"
	"burnmon/internal/pricing"
	"burnmon/internal/schema"
)

func testConfig() *pricing.Config {
	cfg := pricing.Defaults()
	return &cfg
}

func ptr(v int64) *int64 { return &v }

func TestBuildSnapshot_RunningVsStale(t *testing.T) {
	now := time.Now().UTC()
	events := []schema.Event{
		{
			Vendor: "anthropic", Agent: "claude-code", Surface: "cli",
			SessionID: "running", RequestID: "r1", Model: "claude-sonnet-5",
			At: now.Add(-30 * time.Second), Input: 1000, CacheRead: ptr(500), Output: 200,
		},
		{
			Vendor: "anthropic", Agent: "claude-code", Surface: "cli",
			SessionID: "stale", RequestID: "r2", Model: "claude-sonnet-5",
			At: now.Add(-1 * time.Hour), Input: 500, Output: 100,
		},
	}

	snap := BuildSnapshot(events, testConfig(), now)
	if len(snap.Sessions) != 1 {
		t.Fatalf("want 1 running session, got %d: %+v", len(snap.Sessions), snap.Sessions)
	}
	s := snap.Sessions[0]
	if s.SessionID != "running" {
		t.Fatalf("want session 'running', got %q", s.SessionID)
	}
	if s.Context != 1500 {
		t.Fatalf("want context 1500 (input+cache_read), got %d", s.Context)
	}
	// N5 (v0.2.2, SESSION_LOG.md): claude-sonnet-5's book entry moved from
	// 200,000 to its native 1,000,000-token window, re-checked live.
	if s.ContextWindow != 1_000_000 {
		t.Fatalf("want context window 1000000 for claude-sonnet-5, got %d", s.ContextWindow)
	}
	if s.CacheHitRatio <= 0 {
		t.Fatalf("want a positive cache hit ratio, got %v", s.CacheHitRatio)
	}
}

// TestBuildSnapshot_ClientAndBusinessCost guards C3/K3's two new Session
// fields: Client (K1's client map result, carried on the event) and
// BusinessCost (the vendor's headline basis from cfg.CostForEvents, the same
// function every other page uses).
func TestBuildSnapshot_ClientAndBusinessCost(t *testing.T) {
	now := time.Now().UTC()
	events := []schema.Event{
		{
			Vendor: "anthropic", Agent: "claude-code", Surface: "cli",
			SessionID: "running", RequestID: "r1", Model: "claude-sonnet-5", Client: "Talon",
			At: now.Add(-30 * time.Second), Input: 1_000_000, Output: 1_000_000,
		},
	}
	snap := BuildSnapshot(events, testConfig(), now)
	if len(snap.Sessions) != 1 {
		t.Fatalf("want 1 running session, got %d", len(snap.Sessions))
	}
	s := snap.Sessions[0]
	if s.Client != "Talon" {
		t.Fatalf("Client = %q, want %q", s.Client, "Talon")
	}
	if s.BusinessCost == nil {
		t.Fatal("BusinessCost = nil, want a headline basis for anthropic")
	}
	// claude-sonnet-5, no subscription configured: headline is API list
	// (upper bound), 1M input at $2/MTok + 1M output at $10/MTok = 12.
	want := 12.0
	if diff := s.BusinessCost.USD - want; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("BusinessCost.USD = %v, want %v", s.BusinessCost.USD, want)
	}
	if s.BusinessCost.Basis != pricing.BasisAPIList {
		t.Fatalf("BusinessCost.Basis = %v, want %v", s.BusinessCost.Basis, pricing.BasisAPIList)
	}
}

// TestBuildSnapshot_BusinessCostNilForNoBookVendor guards Hermes ("nous")
// and any other vendor with no price book: BusinessCost stays nil, "tokens
// only", exactly like CostForEvents' own VendorCost.Headline.
func TestBuildSnapshot_BusinessCostNilForNoBookVendor(t *testing.T) {
	now := time.Now().UTC()
	events := []schema.Event{
		{
			Vendor: "nous", Agent: "hermes", Surface: "unknown",
			SessionID: "running", RequestID: "r1", Model: "hermes-1",
			At: now.Add(-30 * time.Second), Input: 1000, Output: 1000,
		},
	}
	snap := BuildSnapshot(events, testConfig(), now)
	if len(snap.Sessions) != 1 {
		t.Fatalf("want 1 running session, got %d", len(snap.Sessions))
	}
	if snap.Sessions[0].BusinessCost != nil {
		t.Fatalf("BusinessCost = %+v, want nil for a no-book vendor", snap.Sessions[0].BusinessCost)
	}
}

func TestBuildSnapshot_UnknownModelHasNoWindow(t *testing.T) {
	now := time.Now().UTC()
	events := []schema.Event{
		{
			Vendor: "openai", Agent: "codex", Surface: "cli",
			SessionID: "s1", RequestID: "s1:0", Model: "codex-auto-review",
			At: now, Input: 100, Output: 10,
		},
	}
	snap := BuildSnapshot(events, testConfig(), now)
	if len(snap.Sessions) != 1 {
		t.Fatalf("want 1 running session, got %d", len(snap.Sessions))
	}
	if snap.Sessions[0].ContextWindow != 0 {
		t.Fatalf("want context window 0 (unknown) for codex-auto-review, got %d", snap.Sessions[0].ContextWindow)
	}
}

func TestBuildSnapshot_SubagentNestsUnderRunningParent(t *testing.T) {
	now := time.Now().UTC()
	events := []schema.Event{
		{
			Vendor: "anthropic", Agent: "claude-code", Surface: "cli",
			SessionID: "parent", RequestID: "p1", Model: "claude-sonnet-5",
			At: now.Add(-10 * time.Second), Input: 100, Output: 10,
		},
		{
			Vendor: "anthropic", Agent: "claude-code", Surface: "cli",
			SessionID: "child", ParentID: "parent", RequestID: "c1", Model: "claude-haiku-4-5-20251001",
			At: now.Add(-5 * time.Second), Input: 50, Output: 5,
		},
	}
	snap := BuildSnapshot(events, testConfig(), now)
	if len(snap.Sessions) != 1 {
		t.Fatalf("want 1 top-level session, got %d: %+v", len(snap.Sessions), snap.Sessions)
	}
	if got := len(snap.Sessions[0].Subagents); got != 1 {
		t.Fatalf("want 1 subagent nested under the parent, got %d", got)
	}
	if snap.Sessions[0].Subagents[0].SessionID != "child" {
		t.Fatalf("want subagent 'child', got %q", snap.Sessions[0].Subagents[0].SessionID)
	}
}

// TestBuildSnapshot_ChartIsDense guards F3: the chart is always 180 slots
// (30 minutes at 10-second buckets), every slot present whether or not a
// turn landed in it, rather than only the minutes that had a turn.
func TestBuildSnapshot_ChartIsDense(t *testing.T) {
	now := time.Now().UTC()
	events := []schema.Event{
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s", RequestID: "r1",
			Model: "claude-sonnet-5", At: now.Add(-2 * time.Minute), Input: 10, Output: 5},
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s", RequestID: "r2",
			Model: "claude-sonnet-5", At: now.Add(-40 * time.Minute), Input: 10, Output: 5}, // outside the window
	}
	snap := BuildSnapshot(events, testConfig(), now)
	if len(snap.Chart) != chartSlots {
		t.Fatalf("want %d dense slots, got %d", chartSlots, len(snap.Chart))
	}
	if snap.BucketSeconds != BucketSeconds {
		t.Fatalf("BucketSeconds = %d, want %d", snap.BucketSeconds, BucketSeconds)
	}
	if snap.WindowStart == "" {
		t.Fatal("WindowStart must be set")
	}

	var withTurn, occupiedSlots int
	for _, b := range snap.Chart {
		if len(b.BySession) > 0 {
			occupiedSlots++
			if cc, ok := b.BySession["s"]; ok && cc.Fresh == 10 {
				withTurn++
			}
		}
	}
	if occupiedSlots != 1 {
		t.Fatalf("want exactly 1 occupied slot (only the -2min turn is inside the window), got %d", occupiedSlots)
	}
	if withTurn != 1 {
		t.Fatalf("want the -2min turn's tokens in exactly 1 slot, got %d", withTurn)
	}
}

// TestBuildSnapshot_CustomBucketSeconds guards U5 (v0.3,
// 2026-09-23_v0.3_monitor_view_patch.md): monitor view's braille chart calls
// BuildSnapshot with an explicit 10-second bucket for finer columns, still
// 30 minutes wide, so it must come back as 180 dense slots, not the 30
// default callers get.
func TestBuildSnapshot_CustomBucketSeconds(t *testing.T) {
	now := time.Now().UTC()
	events := []schema.Event{
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s", RequestID: "r1",
			Model: "claude-sonnet-5", At: now.Add(-30 * time.Second), Input: 10, Output: 5},
	}
	snap := BuildSnapshot(events, testConfig(), now, 10)
	if snap.BucketSeconds != 10 {
		t.Fatalf("BucketSeconds = %d, want 10", snap.BucketSeconds)
	}
	if len(snap.Chart) != 180 {
		t.Fatalf("want 180 dense slots at a 10-second bucket, got %d", len(snap.Chart))
	}
}

// TestBuildSnapshot_Turns checks I3's ticker source: one TurnEvent per real
// turn in the chart window (stale sessions included, unlike snap.Sessions),
// newest first, with a re-prefill finding attached to its own turn only.
func TestBuildSnapshot_Turns(t *testing.T) {
	now := time.Now().UTC()
	events := []schema.Event{
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s", RequestID: "r1",
			Model: "claude-sonnet-5", At: now.Add(-10 * time.Minute), Input: 100, Output: 50},
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s", RequestID: "r2",
			Model: "claude-sonnet-5", At: now.Add(-9 * time.Minute), Input: 100, CacheWrite: ptr(25_000), Output: 50},
		// Stale: outside the running window, but inside the 30-minute chart
		// window, so it must still appear (unlike snap.Sessions).
		{Vendor: "openai", Agent: "codex", SessionID: "old", RequestID: "old:0",
			Model: "gpt-5-codex", At: now.Add(-20 * time.Minute), Input: 40, Output: 10},
	}
	snap := BuildSnapshot(events, testConfig(), now)
	if len(snap.Turns) != 3 {
		t.Fatalf("want 3 turns, got %d: %+v", len(snap.Turns), snap.Turns)
	}
	for i := 1; i < len(snap.Turns); i++ {
		if snap.Turns[i-1].At < snap.Turns[i].At {
			t.Fatalf("Turns not newest-first at index %d: %q before %q", i, snap.Turns[i-1].At, snap.Turns[i].At)
		}
	}
	var reprefill *TurnEvent
	for i := range snap.Turns {
		if snap.Turns[i].SessionID == "s" && snap.Turns[i].Turn == 2 {
			reprefill = &snap.Turns[i]
		}
	}
	if reprefill == nil {
		t.Fatal("missing session s turn 2")
	}
	if reprefill.Finding == nil {
		t.Fatalf("turn 2's Finding = %+v, want a finding (it both re-prefills and starts a context-runway fit)", reprefill.Finding)
	}
	for _, te := range snap.Turns {
		if te.SessionID == "s" && te.Turn == 1 && te.Finding != nil {
			t.Fatalf("turn 1 should carry no finding, got %+v", te.Finding)
		}
	}
}

// TestBuildSnapshot_TurnsCappedAtTicker guards I3's own "capped at 50
// lines" for the Now page's turn ticker (snap.Turns, via BuildSnapshot).
func TestBuildSnapshot_TurnsCappedAtTicker(t *testing.T) {
	now := time.Now().UTC()
	var events []schema.Event
	for i := 0; i < 60; i++ {
		events = append(events, schema.Event{
			Vendor: "anthropic", Agent: "claude-code", SessionID: "s", RequestID: "r" + string(rune('a'+i)),
			Model: "claude-sonnet-5", At: now.Add(time.Duration(i) * time.Second), Input: 100, Output: 50,
		})
	}
	snap := BuildSnapshot(events, testConfig(), now.Add(time.Minute))
	if len(snap.Turns) != turnTickerCap {
		t.Fatalf("want the ticker capped at %d turns, got %d", turnTickerCap, len(snap.Turns))
	}
}

// TestBuildTurns_NoCapOverTickerLimit is the regression for the bug review
// caught, 2026-09-24: BuildTurns (the exported function phase 4's advisor
// rule engine and export bundle both call, over an arbitrary window wider
// than the Now page's 30-minute chart) must return every real turn in the
// window, not silently inherit the Now page ticker's 50-turn cap. A caller
// that only ever saw the newest 50 turns of a busy day would under-count
// totals and could make a rule like "this harness had zero turns" fire for
// a harness whose real turns had simply aged out of the cap.
func TestBuildTurns_NoCapOverTickerLimit(t *testing.T) {
	now := time.Now().UTC()
	var events []schema.Event
	for i := 0; i < 60; i++ {
		events = append(events, schema.Event{
			Vendor: "anthropic", Agent: "claude-code", SessionID: "s", RequestID: "r" + string(rune('a'+i)),
			Model: "claude-sonnet-5", At: now.Add(time.Duration(i) * time.Minute), Input: 100, Output: 50,
		})
	}
	turns := BuildTurns(events, testConfig(), now.Add(-time.Hour), now.Add(2*time.Hour))
	if len(turns) != 60 {
		t.Fatalf("want all 60 turns uncapped, got %d", len(turns))
	}
}

// TestSnapshotChangesOnAppend is the v0.1 Step 3 done-when: "a test feeds a
// fixture append to a temp trail and asserts the snapshot changes." Parses a
// real trail file through the claude adapter twice, incrementally, exactly
// as internal/watch's onChange callback would after an fsnotify write event.
func TestSnapshotChangesOnAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abc-123.jsonl")

	now := time.Now().UTC()
	line1 := `{"type":"user","cwd":"/home/dev/proj","message":{"content":"hi"},"timestamp":"` +
		now.Add(-20*time.Second).Format(time.RFC3339Nano) + `","requestId":"r0"}` + "\n"
	if err := os.WriteFile(path, []byte(line1), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := claude.Adapter{}
	events, _, offset, err := adapter.Parse(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	before := BuildSnapshot(events, testConfig(), now)
	if len(before.Sessions) != 0 {
		t.Fatalf("want no running session before any turn, got %d", len(before.Sessions))
	}

	line2 := `{"type":"assistant","message":{"model":"claude-sonnet-5","usage":{` +
		`"input_tokens":1200,"cache_read_input_tokens":800,"output_tokens":300}},` +
		`"timestamp":"` + now.Add(-5*time.Second).Format(time.RFC3339Nano) + `","requestId":"r1"}` + "\n"
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(line2); err != nil {
		t.Fatal(err)
	}
	f.Close()

	moreEvents, _, _, err := adapter.Parse(path, offset)
	if err != nil {
		t.Fatal(err)
	}
	if len(moreEvents) == 0 {
		t.Fatal("want at least one event from the appended line")
	}
	after := BuildSnapshot(append(events, moreEvents...), testConfig(), now)

	if len(after.Sessions) != 1 {
		t.Fatalf("want 1 running session after the append, got %d", len(after.Sessions))
	}
	if after.Sessions[0].Tokens == 0 {
		t.Fatal("want the running session's tokens to reflect the appended turn")
	}
	var occupied int
	for _, b := range after.Chart {
		if len(b.BySession) > 0 {
			occupied++
		}
	}
	if occupied == 0 {
		t.Fatal("want the appended turn to land in a chart bucket")
	}
}
