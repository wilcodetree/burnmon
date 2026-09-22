package history

import (
	"path/filepath"
	"testing"
	"time"

	"burnmon/internal/pricing"
	"burnmon/internal/schema"
	"burnmon/internal/store"
)

func ptr[T any](v T) *T { return &v }

// TestBuildWeekAndMonth is 41A's spec'd test: a store fixture with two
// vendors (Claude Code/anthropic, Codex/openai) over three weeks, bmHistory
// (Build, the function the binding calls) for period=week and period=month,
// checked against hand-computed totals.
func TestBuildWeekAndMonth(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// Three Mondays, seven days apart, all inside September 2026: distinct
	// ISO weeks, one calendar month.
	weekStarts := []time.Time{
		time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC),
	}

	var events []schema.Event
	for i, at := range weekStarts {
		events = append(events,
			schema.Event{
				Vendor: "anthropic", Agent: "claude-code", Surface: "cli",
				SessionID: "claude-" + at.Format("2006-01-02"), RequestID: "r-claude-" + at.Format("2006-01-02"),
				At: at, Model: "claude-sonnet-5",
				Input: 1000, Output: 200,
				CacheWrite: ptr(int64(0)), CacheRead: ptr(int64(0)),
			},
			schema.Event{
				Vendor: "openai", Agent: "codex", Surface: "cli",
				SessionID: "codex-" + at.Format("2006-01-02"), RequestID: "r-codex-" + at.Format("2006-01-02"),
				At: at.Add(2 * time.Hour), Model: "gpt-6-astra",
				Input: 500, Output: 100,
				CacheWrite: ptr(int64(0)), CacheRead: ptr(int64(0)),
			},
		)
		_ = i
	}
	if err := st.UpsertEvents(events); err != nil {
		t.Fatal(err)
	}

	got, err := st.AllEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 6 {
		t.Fatalf("AllEvents returned %d events, want 6", len(got))
	}

	cfg := pricing.Defaults()
	filter := Filter{Period: "week", Vendor: "", From: "2026-09-01", To: "2026-09-30"}

	payload := Build(got, &cfg, filter)
	if len(payload.Rows) != 3 {
		t.Fatalf("week rows = %d, want 3 (one per week), got keys %+v", len(payload.Rows), payload.Rows)
	}
	for _, row := range payload.Rows {
		if row.Sessions != 2 {
			t.Errorf("week %s: sessions = %d, want 2 (one Claude Code, one Codex)", row.Key, row.Sessions)
		}
		if row.Turns != 2 {
			t.Errorf("week %s: turns = %d, want 2", row.Key, row.Turns)
		}
		wantTokens := int64(1000 + 200 + 500 + 100)
		if row.Tokens != wantTokens {
			t.Errorf("week %s: tokens = %d, want %d", row.Key, row.Tokens, wantTokens)
		}
		if row.CostNote != "tokens only until v0.3" {
			t.Errorf("week %s: cost_note = %q, want the v0.3 note (vendor=All never shows cost)", row.Key, row.CostNote)
		}
		if row.CostUSD != nil {
			t.Errorf("week %s: cost_usd = %v, want nil when vendor is All", row.Key, *row.CostUSD)
		}
	}
	if payload.Totals.Sessions != 6 {
		t.Errorf("overall sessions = %d, want 6 (3 weeks x 2 vendors, distinct session ids)", payload.Totals.Sessions)
	}
	if payload.Totals.Turns != 6 {
		t.Errorf("overall turns = %d, want 6", payload.Totals.Turns)
	}
	wantVendors := []string{"claude-code", "codex"}
	if len(payload.Vendors) != len(wantVendors) || payload.Vendors[0] != wantVendors[0] || payload.Vendors[1] != wantVendors[1] {
		t.Errorf("vendors = %+v, want %+v", payload.Vendors, wantVendors)
	}

	// Single covered vendor: cost appears, computed against pricing.Defaults.
	claudeOnly := Build(got, &cfg, Filter{Period: "week", Vendor: "claude-code", From: "2026-09-01", To: "2026-09-30"})
	if claudeOnly.Totals.CostUSD == nil {
		t.Fatal("claude-code totals: cost_usd is nil, want a figure (anthropic is a covered vendor)")
	}
	// sonnet: in 3.0, out 15.0 USD/Mtok; one call/week x 3 weeks, 1000 input + 200 output.
	wantCostPerCall := (1000.0*3.0 + 200.0*15.0) / 1e6
	wantTotalCost := wantCostPerCall * 3
	if diff := *claudeOnly.Totals.CostUSD - wantTotalCost; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("claude-code total cost_usd = %v, want %v", *claudeOnly.Totals.CostUSD, wantTotalCost)
	}
	if claudeOnly.Totals.Sessions != 3 {
		t.Errorf("claude-code overall sessions = %d, want 3", claudeOnly.Totals.Sessions)
	}

	// Same range, bucketed by month: all three weeks fall in September 2026,
	// so this must collapse to one row with the combined totals.
	monthFilter := Filter{Period: "month", Vendor: "", From: "2026-09-01", To: "2026-09-30"}
	monthPayload := Build(got, &cfg, monthFilter)
	if len(monthPayload.Rows) != 1 {
		t.Fatalf("month rows = %d, want 1, got %+v", len(monthPayload.Rows), monthPayload.Rows)
	}
	mr := monthPayload.Rows[0]
	if mr.Key != "2026-09" {
		t.Errorf("month key = %q, want 2026-09", mr.Key)
	}
	if mr.Sessions != 6 {
		t.Errorf("month sessions = %d, want 6", mr.Sessions)
	}
	if mr.Turns != 6 {
		t.Errorf("month turns = %d, want 6", mr.Turns)
	}
	wantMonthTokens := int64((1000 + 200 + 500 + 100) * 3)
	if mr.Tokens != wantMonthTokens {
		t.Errorf("month tokens = %d, want %d", mr.Tokens, wantMonthTokens)
	}
}

// TestBuildOwnerFilter checks P6's owner column is honoured by the same
// filter, independent of the vendor/period tests above.
func TestBuildOwnerFilter(t *testing.T) {
	cfg := pricing.Defaults()
	at := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	events := []schema.Event{
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s1", RequestID: "r1", At: at, Model: "claude-sonnet-5", Input: 10, Output: 5, Owner: "ZND"},
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s2", RequestID: "r2", At: at, Model: "claude-sonnet-5", Input: 10, Output: 5, Owner: "Valona"},
	}
	payload := Build(events, &cfg, Filter{Period: "day", Owner: "ZND", From: "2026-09-01", To: "2026-09-30"})
	if payload.Totals.Sessions != 1 {
		t.Fatalf("sessions = %d, want 1 (owner filter must exclude the Valona session)", payload.Totals.Sessions)
	}
	if len(payload.Owners) != 2 || payload.Owners[0] != "Valona" || payload.Owners[1] != "ZND" {
		t.Errorf("owners = %+v, want [Valona ZND] (every owner seen, unfiltered)", payload.Owners)
	}
}
