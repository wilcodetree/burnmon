package dataset

import (
	"testing"
	"time"

	"burnmon/internal/pricing"
	"burnmon/internal/schema"
)

func ptr[T any](v T) *T { return &v }

func TestSessionsFromEventsAggregatesOneSession(t *testing.T) {
	cfg := pricing.Defaults()
	events := []schema.Event{
		{
			Vendor: "anthropic", SessionID: "sess-a", RequestID: "r1",
			Model: cfg.Families()[0], Title: "hello", Surface: "cli",
			At: time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC),
			Input: 100, Output: 10,
		},
		{
			Vendor: "anthropic", SessionID: "sess-a", RequestID: "r2",
			Model: cfg.Families()[0], Title: "", Surface: "cli",
			At: time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC),
			Input: 50, Output: 5, CacheRead: ptr(int64(20)),
			Tools: map[string]int64{"Read": 2},
		},
	}
	sessions := SessionsFromEvents(events, &cfg)
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	s := sessions[0]
	if s.SessionID != "sess-a" {
		t.Fatalf("SessionID = %q, want sess-a", s.SessionID)
	}
	if s.Title != "hello" {
		t.Fatalf("Title = %q, want the first non-empty Title seen", s.Title)
	}
	if s.Calls != 2 || s.Fresh != 150 || s.CacheR != 20 || s.Out != 15 {
		t.Fatalf("totals wrong: calls=%d fresh=%d cacheR=%d out=%d", s.Calls, s.Fresh, s.CacheR, s.Out)
	}
	if len(s.Daily) != 2 {
		t.Fatalf("got %d daily buckets, want 2 (2026-09-10 and 2026-09-11)", len(s.Daily))
	}
	if s.Tools["Read"] != 2 {
		t.Fatalf("Tools[Read] = %d, want 2", s.Tools["Read"])
	}
	if s.Start != "2026-09-10T10:00:00Z" && s.Start[:10] != "2026-09-10" {
		t.Fatalf("Start = %q, want to begin 2026-09-10", s.Start)
	}
}

func TestSessionsFromEventsGroupsByVendorAndSessionID(t *testing.T) {
	cfg := pricing.Defaults()
	events := []schema.Event{
		{Vendor: "anthropic", SessionID: "s1", RequestID: "r1", Model: cfg.Families()[0],
			At: time.Now().UTC(), Input: 1, Output: 1},
		{Vendor: "anthropic", SessionID: "s2", RequestID: "r2", Model: cfg.Families()[0],
			At: time.Now().UTC(), Input: 1, Output: 1},
	}
	sessions := SessionsFromEvents(events, &cfg)
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}
}
