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
	if s.ContextWindow != 200_000 {
		t.Fatalf("want context window 200000 for claude-sonnet-5, got %d", s.ContextWindow)
	}
	if s.CacheHitRatio <= 0 {
		t.Fatalf("want a positive cache hit ratio, got %v", s.CacheHitRatio)
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

func TestBuildSnapshot_ChartBucketsLast30Minutes(t *testing.T) {
	now := time.Now().UTC()
	events := []schema.Event{
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s", RequestID: "r1",
			Model: "claude-sonnet-5", At: now.Add(-2 * time.Minute), Input: 10, Output: 5},
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s", RequestID: "r2",
			Model: "claude-sonnet-5", At: now.Add(-40 * time.Minute), Input: 10, Output: 5},
	}
	snap := BuildSnapshot(events, testConfig(), now)
	if len(snap.Chart) != 1 {
		t.Fatalf("want 1 bucket (only the turn inside the last 30 minutes), got %d", len(snap.Chart))
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
	events, offset, err := adapter.Parse(path, 0)
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

	moreEvents, _, err := adapter.Parse(path, offset)
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
	if len(after.Chart) == 0 {
		t.Fatal("want the appended turn to land in a chart bucket")
	}
}
