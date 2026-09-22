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
	if s.Start != "2026-09-10T10:00:00.000Z" {
		t.Fatalf("Start = %q, want 2026-09-10T10:00:00.000Z", s.Start)
	}
	if s.End != "2026-09-11T09:00:00.000Z" {
		t.Fatalf("End = %q, want 2026-09-11T09:00:00.000Z", s.End)
	}
}

func TestSessionsFromEventsPricesOpenAIEvents(t *testing.T) {
	cfg := pricing.Defaults()
	events := []schema.Event{
		{
			Vendor: "openai", Agent: "codex", SessionID: "codex-sess", RequestID: "codex-sess:1",
			Model: "gpt-5.6-terra", Surface: "cli",
			At:        time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC),
			Input:     1000, // fresh, already input-minus-cached per the codex adapter
			CacheRead: ptr(int64(500)),
			Output:    100,
		},
	}
	sessions := SessionsFromEvents(events, &cfg)
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	s := sessions[0]
	want := (1000*2.00 + 500*0.20 + 100*12.00) / 1e6
	if diff := s.Cost - want; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("Cost = %v, want %v (gpt-5.6-terra list price)", s.Cost, want)
	}
	if s.CostSub != s.Cost {
		t.Fatalf("CostSub = %v, want equal to Cost (no OpenAI subscription calibration in v0.1)", s.CostSub)
	}
	if s.Unpriced != 0 {
		t.Fatalf("Unpriced = %d, want 0 for a priced model", s.Unpriced)
	}
	if _, ok := s.Models["GPT-5.6 Terra"]; !ok {
		t.Fatalf("Models keys = %v, want a GPT-5.6 Terra entry", s.Models)
	}
}

func TestSessionsFromEventsCountsUnpricedOpenAIModel(t *testing.T) {
	cfg := pricing.Defaults()
	events := []schema.Event{
		{
			Vendor: "openai", Agent: "codex", SessionID: "codex-sess2", RequestID: "codex-sess2:1",
			Model: "codex-auto-review", Surface: "cli",
			At:     time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC),
			Input:  1000,
			Output: 100,
		},
	}
	sessions := SessionsFromEvents(events, &cfg)
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	s := sessions[0]
	if s.Cost != 0 {
		t.Fatalf("Cost = %v, want 0 for an unpriced model", s.Cost)
	}
	if s.Unpriced != 1100 {
		t.Fatalf("Unpriced = %d, want 1100 (all tokens on the one unpriced call)", s.Unpriced)
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
