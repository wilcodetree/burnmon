package store

import (
	"path/filepath"
	"testing"
	"time"

	"burnmon/internal/schema"
)

func ptr[T any](v T) *T { return &v }

func TestUpsertEventsLargestOutputWins(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	base := schema.Event{
		Vendor: "anthropic", Agent: "claude-code", Surface: "cli",
		SessionID: "s1", RequestID: "r1", Model: "claude-x",
		At: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		Input: 100, Output: 5,
	}
	if err := st.UpsertEvents([]schema.Event{base}); err != nil {
		t.Fatal(err)
	}
	bigger := base
	bigger.Output = 50
	if err := st.UpsertEvents([]schema.Event{bigger}); err != nil {
		t.Fatal(err)
	}
	smaller := base
	smaller.Output = 10
	if err := st.UpsertEvents([]schema.Event{smaller}); err != nil {
		t.Fatal(err)
	}

	got, err := st.AllEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1 (same vendor+request_id must dedup)", len(got))
	}
	if got[0].Output != 50 {
		t.Fatalf("Output = %d, want 50 (largest output wins)", got[0].Output)
	}
}

// TestUpsertEventsScopedPerSession guards against the Cowork mirror-session
// bug: two different sessions can share the same request_id (e.g. a
// transcript mirrored across session-id files), and each session's event
// must survive independently rather than being deduped against the other.
func TestUpsertEventsScopedPerSession(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	a := schema.Event{
		Vendor: "anthropic", Agent: "claude-code", Surface: "cli",
		SessionID: "session-a", RequestID: "req_shared", Model: "claude-x",
		At: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		Input: 100, Output: 100,
	}
	b := schema.Event{
		Vendor: "anthropic", Agent: "claude-code", Surface: "cli",
		SessionID: "session-b", RequestID: "req_shared", Model: "claude-x",
		At: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		Input: 100, Output: 50,
	}
	if err := st.UpsertEvents([]schema.Event{a, b}); err != nil {
		t.Fatal(err)
	}

	got, err := st.AllEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2 (same request_id, different session_id must not dedup)", len(got))
	}
	bySession := map[string]schema.Event{}
	for _, e := range got {
		bySession[e.SessionID] = e
	}
	ga, ok := bySession["session-a"]
	if !ok {
		t.Fatal("missing event for session-a")
	}
	if ga.Output != 100 {
		t.Fatalf("session-a Output = %d, want 100 (not overwritten by session-b)", ga.Output)
	}
	gb, ok := bySession["session-b"]
	if !ok {
		t.Fatal("missing event for session-b")
	}
	if gb.Output != 50 {
		t.Fatalf("session-b Output = %d, want 50 (not overwritten by session-a)", gb.Output)
	}
}

func TestCursorRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if _, _, _, ok, err := st.Cursor(`C:\trail.jsonl`); err != nil || ok {
		t.Fatalf("Cursor on an unknown path: ok=%v err=%v, want ok=false", ok, err)
	}
	mtime := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := st.SetCursor(`C:\trail.jsonl`, 1234, mtime, 5000); err != nil {
		t.Fatal(err)
	}
	offset, gotMtime, size, ok, err := st.Cursor(`C:\trail.jsonl`)
	if err != nil || !ok {
		t.Fatalf("Cursor after SetCursor: ok=%v err=%v", ok, err)
	}
	if offset != 1234 || size != 5000 || !gotMtime.Equal(mtime) {
		t.Fatalf("Cursor = (%d, %v, %d), want (1234, %v, 5000)", offset, gotMtime, size, mtime)
	}
}

func TestNullableFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	e := schema.Event{
		Vendor: "openai", Agent: "codex", Surface: "cli",
		SessionID: "s2", RequestID: "r2", Model: "gpt-x",
		At: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Input: 10, Output: 20,
		CacheRead: ptr(int64(5)), Reasoning: ptr(int64(3)),
		Tools: map[string]int64{"Read": 2},
	}
	if err := st.UpsertEvents([]schema.Event{e}); err != nil {
		t.Fatal(err)
	}
	got, err := st.AllEvents()
	if err != nil || len(got) != 1 {
		t.Fatalf("AllEvents: %v, len=%d", err, len(got))
	}
	g := got[0]
	if g.CacheWrite != nil {
		t.Fatal("CacheWrite should stay nil")
	}
	if g.CacheRead == nil || *g.CacheRead != 5 {
		t.Fatalf("CacheRead = %v, want 5", g.CacheRead)
	}
	if g.Tools["Read"] != 2 {
		t.Fatalf("Tools[Read] = %d, want 2", g.Tools["Read"])
	}
}

// TestEventsSince guards F1's replacement for AllEvents on the bmLive poll
// path: only events at or after the cutoff come back.
func TestEventsSince(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	old := schema.Event{
		Vendor: "anthropic", Agent: "claude-code", Surface: "cli",
		SessionID: "s1", RequestID: "r1", Model: "claude-x",
		At: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), Input: 10, Output: 1,
	}
	recent := old
	recent.RequestID = "r2"
	recent.At = time.Date(2026, 9, 1, 10, 20, 0, 0, time.UTC)
	if err := st.UpsertEvents([]schema.Event{old, recent}); err != nil {
		t.Fatal(err)
	}

	got, err := st.EventsSince(time.Date(2026, 9, 1, 10, 10, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RequestID != "r2" {
		t.Fatalf("EventsSince = %+v, want only r2", got)
	}
}

// TestSessionTotals guards F1's SQL-aggregated whole-session totals used by
// live.ApplySessionTotals: sums must be exact and grouped per model, and the
// claude adapter's synthetic tool-only events (model="" and input=output=0)
// must be excluded, matching internal/live's own isTurn filter.
func TestSessionTotals(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	base := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	events := []schema.Event{
		{Vendor: "anthropic", Agent: "claude-code", Surface: "cli", SessionID: "s1",
			RequestID: "r1", Model: "claude-sonnet-5", At: base, Input: 100, Output: 10},
		{Vendor: "anthropic", Agent: "claude-code", Surface: "cli", SessionID: "s1",
			RequestID: "r2", Model: "claude-sonnet-5", At: base.Add(time.Minute), Input: 200, Output: 20},
		{Vendor: "anthropic", Agent: "claude-code", Surface: "cli", SessionID: "s1",
			RequestID: "r3", Model: "", At: base.Add(2 * time.Minute), Input: 0, Output: 0}, // synthetic tool-only event
	}
	if err := st.UpsertEvents(events); err != nil {
		t.Fatal(err)
	}

	rows, err := st.SessionTotals([]SessionKey{{Vendor: "anthropic", SessionID: "s1"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 (vendor,session,model) group, got %d: %+v", len(rows), rows)
	}
	r := rows[0]
	if r.Input != 300 || r.Output != 30 || r.Count != 2 {
		t.Fatalf("want input=300 output=30 count=2, got input=%d output=%d count=%d", r.Input, r.Output, r.Count)
	}
	if !r.MinAt.Equal(base) {
		t.Fatalf("MinAt = %v, want %v", r.MinAt, base)
	}
}

// TestSessionTotalsVendorQualified guards Wilco's decision to key
// SessionTotals by (vendor, session_id) pairs, not session_id alone: two
// different vendors can in principle mint the same session id, and a
// vendor-unqualified lookup would wrongly sum both vendors' events into one
// session's totals.
func TestSessionTotalsVendorQualified(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	base := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	events := []schema.Event{
		{Vendor: "anthropic", Agent: "claude-code", Surface: "cli", SessionID: "shared",
			RequestID: "r1", Model: "claude-sonnet-5", At: base, Input: 100, Output: 10},
		{Vendor: "openai", Agent: "codex", Surface: "cli", SessionID: "shared",
			RequestID: "r1", Model: "gpt-5.6-sol", At: base, Input: 5000, Output: 500},
	}
	if err := st.UpsertEvents(events); err != nil {
		t.Fatal(err)
	}

	rows, err := st.SessionTotals([]SessionKey{{Vendor: "anthropic", SessionID: "shared"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 group for the anthropic key alone, got %d: %+v", len(rows), rows)
	}
	if rows[0].Vendor != "anthropic" || rows[0].Input != 100 || rows[0].Output != 10 {
		t.Fatalf("want anthropic's own totals only (input=100 output=10), got %+v", rows[0])
	}
}
