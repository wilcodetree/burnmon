package live

import (
	"path/filepath"
	"testing"
	"time"

	"burnmon/internal/schema"
	"burnmon/internal/store"
)

// TestBuildTurnDetail checks bmTurn's whole payload: token classes, the gap
// since the previous turn (absent on turn 1), the matching finding, and the
// tool calls (plus their deduplicated file paths) joined by the turn's own
// RequestID.
func TestBuildTurnDetail(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	events := []schema.Event{
		{Vendor: "anthropic", Agent: "claude-code", Surface: "cli", SessionID: "s1", RequestID: "req-1",
			Model: "claude-sonnet-5", At: base, Input: 100, Output: 50},
		{Vendor: "anthropic", Agent: "claude-code", Surface: "cli", SessionID: "s1", RequestID: "req-2",
			Model: "claude-sonnet-5", At: base.Add(90 * time.Minute), Input: 100, CacheWrite: ptr(25_000), Output: 50},
	}
	if err := st.UpsertEvents(events); err != nil {
		t.Fatal(err)
	}
	calls := []schema.ToolCall{
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s1", CallID: "toolu_1",
			Turn: "req-2", Tool: "Read", At: base.Add(90 * time.Minute), InputBytes: 10, Path: "a.go"},
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s1", CallID: "toolu_2",
			Turn: "req-2", Tool: "Read", At: base.Add(90*time.Minute + time.Second), InputBytes: 10, Path: "a.go"},
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s1", CallID: "toolu_3",
			Turn: "req-1", Tool: "Bash", At: base, InputBytes: 5},
	}
	if err := st.UpsertToolCalls(calls); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig()

	d1, err := BuildTurnDetail(st, cfg, "s1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if d1.HasGap {
		t.Fatalf("turn 1 HasGap = true, want false (no previous turn)")
	}
	if len(d1.ToolCalls) != 1 || d1.ToolCalls[0].Tool != "Bash" {
		t.Fatalf("turn 1 ToolCalls = %+v, want just the Bash call", d1.ToolCalls)
	}
	if len(d1.Files) != 0 {
		t.Fatalf("turn 1 Files = %v, want none (Bash carries no path)", d1.Files)
	}

	d2, err := BuildTurnDetail(st, cfg, "s1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !d2.HasGap || d2.GapSeconds != 90*60 {
		t.Fatalf("turn 2 HasGap/GapSeconds = %v/%v, want true/5400", d2.HasGap, d2.GapSeconds)
	}
	if d2.CacheWrite != 25_000 {
		t.Fatalf("turn 2 CacheWrite = %d, want 25000", d2.CacheWrite)
	}
	if len(d2.Findings) == 0 {
		t.Fatal("turn 2 should carry at least a re-prefill finding")
	}
	if len(d2.ToolCalls) != 2 {
		t.Fatalf("turn 2 ToolCalls = %d, want 2", len(d2.ToolCalls))
	}
	if len(d2.Files) != 1 || d2.Files[0] != "a.go" {
		t.Fatalf("turn 2 Files = %v, want [a.go] (deduplicated)", d2.Files)
	}

	if _, err := BuildTurnDetail(st, cfg, "s1", 99); err != ErrTurnNotFound {
		t.Fatalf("out-of-range turn: err = %v, want ErrTurnNotFound", err)
	}
	if _, err := BuildTurnDetail(st, cfg, "no-such-session", 1); err != ErrTurnNotFound {
		t.Fatalf("unknown session: err = %v, want ErrTurnNotFound", err)
	}
}
