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
		// V3-1's headline cost function sums every covered vendor present,
		// not just a single vendor filter (C3/K3: replaces v0.2's "tokens
		// only until v0.3" All-vendor gate). claude-sonnet-5's book: in 2.0,
		// out 10.0; gpt-6-astra's book: in 10.0, out 50.0, USD/MTok.
		wantWeekCost := (1000.0*2.0+200.0*10.0)/1e6 + (500.0*10.0+100.0*50.0)/1e6
		if row.CostNote != "" {
			t.Errorf("week %s: cost_note = %q, want none (both vendors are covered)", row.Key, row.CostNote)
		}
		if row.CostUSD == nil {
			t.Fatalf("week %s: cost_usd is nil, want %v", row.Key, wantWeekCost)
		}
		if diff := *row.CostUSD - wantWeekCost; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("week %s: cost_usd = %v, want %v", row.Key, *row.CostUSD, wantWeekCost)
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
	// claude-sonnet-5 book: in 2.0, out 10.0 USD/Mtok; one call/week x 3
	// weeks, 1000 input + 200 output.
	wantCostPerCall := (1000.0*2.0 + 200.0*10.0) / 1e6
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

// TestBuildNoBookVendorIsTokensOnly guards the replacement text (K3/C3: "or
// 'tokens only' for vendors without a book"): Hermes ("nous") has no price
// book at all, so both the note and the fallback text read "tokens only",
// never the stale v0.2 "until v0.3" placeholder.
func TestBuildNoBookVendorIsTokensOnly(t *testing.T) {
	cfg := pricing.Defaults()
	at := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	events := []schema.Event{
		{Vendor: "nous", Agent: "hermes", SessionID: "h1", RequestID: "r1", At: at, Model: "hermes-1", Input: 100, Output: 50},
	}
	payload := Build(events, &cfg, Filter{Period: "day", From: "2026-09-01", To: "2026-09-30"})
	if payload.Totals.CostUSD != nil {
		t.Fatalf("Totals.CostUSD = %v, want nil (no vendor covered)", *payload.Totals.CostUSD)
	}
	if payload.Totals.CostNote != "tokens only" {
		t.Errorf("Totals.CostNote = %q, want %q", payload.Totals.CostNote, "tokens only")
	}
	if len(payload.Rows) != 1 || payload.Rows[0].CostNote != "tokens only" {
		t.Errorf("Rows = %+v, want one row noted %q", payload.Rows, "tokens only")
	}
}

// TestBuildClientFilterAndRows guards K3's client filter (narrows Totals the
// same way Owner does) and ClientRows (the per-client table, unaffected by
// the client filter itself, so both clients stay visible for comparison).
func TestBuildClientFilterAndRows(t *testing.T) {
	cfg := pricing.Defaults()
	at := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	events := []schema.Event{
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s1", RequestID: "r1", At: at, Model: "claude-sonnet-5", Input: 1000, Output: 200, Client: "Talon"},
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s2", RequestID: "r2", At: at.Add(time.Hour), Model: "claude-sonnet-5", Input: 500, Output: 100, Client: "OtherCo"},
	}
	filter := Filter{Period: "day", Client: "Talon", From: "2026-09-01", To: "2026-09-30"}
	payload := Build(events, &cfg, filter)

	if payload.Totals.Sessions != 1 {
		t.Fatalf("Totals.Sessions = %d, want 1 (client filter must exclude OtherCo's session)", payload.Totals.Sessions)
	}
	if len(payload.Clients) != 2 || payload.Clients[0] != "OtherCo" || payload.Clients[1] != "Talon" {
		t.Errorf("Clients = %+v, want [OtherCo Talon] (every client seen, unfiltered by Filter.Client)", payload.Clients)
	}
	if len(payload.ClientRows) != 2 {
		t.Fatalf("ClientRows = %+v, want 2 (both clients, the row table ignores Filter.Client)", payload.ClientRows)
	}
	byClient := map[string]ClientRow{}
	for _, r := range payload.ClientRows {
		byClient[r.Client] = r
	}
	talon := byClient["Talon"]
	if talon.Sessions != 1 {
		t.Errorf("Talon row sessions = %d, want 1", talon.Sessions)
	}
	if talon.Tokens != 1200 {
		t.Errorf("Talon row tokens = %d, want 1200", talon.Tokens)
	}
	if talon.CostUSD == nil {
		t.Fatal("Talon row cost_usd is nil, want a figure (anthropic is covered)")
	}
	wantCost := (1000.0*2.0 + 200.0*10.0) / 1e6
	if diff := *talon.CostUSD - wantCost; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("Talon row cost_usd = %v, want %v", *talon.CostUSD, wantCost)
	}
	other := byClient["OtherCo"]
	if other.Sessions != 1 || other.Tokens != 600 {
		t.Errorf("OtherCo row = %+v, want sessions=1 tokens=600", other)
	}
}
