package store

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// TestEventsForSession guards the query `burnmon-cli insight <session-id>`
// (v0.2 I1) reads through: every event for sessionID, ordered oldest first,
// and none of another session's events.
func TestEventsForSession(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	s1a := schema.Event{
		Vendor: "anthropic", Agent: "claude-code", Surface: "cli",
		SessionID: "s1", RequestID: "r1", Model: "claude-x",
		At: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), Input: 10, Output: 1,
	}
	s1b := s1a
	s1b.RequestID = "r2"
	s1b.At = time.Date(2026, 9, 1, 10, 5, 0, 0, time.UTC)
	other := s1a
	other.SessionID = "s2"
	other.RequestID = "r3"
	if err := st.UpsertEvents([]schema.Event{s1b, s1a, other}); err != nil {
		t.Fatal(err)
	}

	got, err := st.EventsForSession("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].RequestID != "r1" || got[1].RequestID != "r2" {
		t.Fatalf("EventsForSession = %+v, want r1 then r2", got)
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

// TestFreshStoreAtHeadVersion guards S1: a brand-new store must run every
// migration and land at the current head version.
func TestFreshStoreAtHeadVersion(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var version int
	if err := st.db.QueryRow(`SELECT version FROM schema_version LIMIT 1`).Scan(&version); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if version != 5 {
		t.Fatalf("schema_version = %d, want 5", version)
	}
}

// TestUpsertToolCallsAndTotals guards S2's tool_calls store path: a call and
// its result upsert onto the same row (matched by call_id), a later upsert
// that carries no result never wipes one already recorded, and
// ToolCallTotals aggregates per tool name within the given cutoff.
func TestUpsertToolCallsAndTotals(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	base := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	call := schema.ToolCall{
		Vendor: "anthropic", Agent: "claude-code", SessionID: "s1",
		CallID: "toolu_1", Turn: "req-1", Tool: "Read", At: base, InputBytes: 20,
	}
	if err := st.UpsertToolCalls([]schema.ToolCall{call}); err != nil {
		t.Fatal(err)
	}
	// A later row for the same call_id with no result yet (input_bytes only)
	// must not erase a result already recorded... except none has been
	// recorded yet here, so this checks the opposite: recording one now.
	withResult := call
	withResult.ResultBytes = ptr(int64(100))
	if err := st.UpsertToolCalls([]schema.ToolCall{withResult}); err != nil {
		t.Fatal(err)
	}
	// Simulate a stray re-upsert of the bare call (e.g. a re-read of the same
	// region) with no result: the already-recorded result_bytes must survive.
	if err := st.UpsertToolCalls([]schema.ToolCall{call}); err != nil {
		t.Fatal(err)
	}

	other := schema.ToolCall{
		Vendor: "openai", Agent: "codex", SessionID: "s2",
		CallID: "call_1", Turn: "turn-1", Tool: "exec",
		At: base.Add(time.Minute), InputBytes: 50, ResultBytes: ptr(int64(30)),
	}
	old := schema.ToolCall{
		Vendor: "anthropic", Agent: "claude-code", SessionID: "s1",
		CallID: "toolu_old", Turn: "req-0", Tool: "Read",
		At: base.Add(-48 * time.Hour), InputBytes: 5, ResultBytes: ptr(int64(5)),
	}
	if err := st.UpsertToolCalls([]schema.ToolCall{other, old}); err != nil {
		t.Fatal(err)
	}

	rows, err := st.ToolCallTotals(base.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	byTool := map[string]ToolCallTotal{}
	for _, r := range rows {
		byTool[r.Tool] = r
	}
	read, ok := byTool["Read"]
	if !ok {
		t.Fatal("missing Read row")
	}
	if read.Calls != 1 || read.Sessions != 1 || read.InputBytes != 20 || read.ResultBytes != 100 {
		t.Fatalf("Read totals = %+v, want 1 call, 1 session, 20 input bytes, 100 result bytes (old call outside the window excluded)", read)
	}
	exec, ok := byTool["exec"]
	if !ok {
		t.Fatal("missing exec row")
	}
	if exec.Calls != 1 || exec.InputBytes != 50 || exec.ResultBytes != 30 {
		t.Fatalf("exec totals = %+v, want 1 call, 50 input bytes, 30 result bytes", exec)
	}
}

// TestToolCallsForTurn checks the I3 drawer's join: looked up by (vendor,
// session_id, turn), oldest first, a path carried through and preserved
// across a later re-upsert that has no path of its own (migration4).
func TestToolCallsForTurn(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	base := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	read := schema.ToolCall{
		Vendor: "anthropic", Agent: "claude-code", SessionID: "s1",
		CallID: "toolu_1", Turn: "req-1", Tool: "Read", At: base,
		InputBytes: 20, Path: `C:\ZND\foo.go`,
	}
	edit := schema.ToolCall{
		Vendor: "anthropic", Agent: "claude-code", SessionID: "s1",
		CallID: "toolu_2", Turn: "req-1", Tool: "Edit", At: base.Add(time.Second),
		InputBytes: 40, Path: `C:\ZND\bar.go`,
	}
	otherTurn := schema.ToolCall{
		Vendor: "anthropic", Agent: "claude-code", SessionID: "s1",
		CallID: "toolu_3", Turn: "req-2", Tool: "Read", At: base.Add(time.Minute),
		InputBytes: 5, Path: `C:\ZND\baz.go`,
	}
	if err := st.UpsertToolCalls([]schema.ToolCall{read, edit, otherTurn}); err != nil {
		t.Fatal(err)
	}
	// A stray re-upsert of the same call_id with an empty path (e.g. a
	// second Parse pass that could not re-derive it) must not blank out the
	// path already recorded.
	readNoPath := read
	readNoPath.Path = ""
	readNoPath.ResultBytes = ptr(int64(99))
	if err := st.UpsertToolCalls([]schema.ToolCall{readNoPath}); err != nil {
		t.Fatal(err)
	}

	calls, err := st.ToolCallsForTurn("anthropic", "s1", "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("want 2 calls for req-1, got %d: %+v", len(calls), calls)
	}
	if calls[0].Tool != "Read" || calls[0].Path != `C:\ZND\foo.go` {
		t.Fatalf("calls[0] = %+v, want Read with its path preserved through the re-upsert", calls[0])
	}
	if calls[0].ResultBytes == nil || *calls[0].ResultBytes != 99 {
		t.Fatalf("calls[0].ResultBytes = %v, want 99", calls[0].ResultBytes)
	}
	if calls[1].Tool != "Edit" || calls[1].Path != `C:\ZND\bar.go` {
		t.Fatalf("calls[1] = %+v, want Edit with its own path", calls[1])
	}

	none, err := st.ToolCallsForTurn("anthropic", "s1", "req-nope")
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("want no calls for an unknown turn, got %d", len(none))
	}
}

// copyFile is a plain byte copy, used to snapshot a real store file into a
// temp dir without touching the original.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// rawEventKeys opens path with a bare sqlite connection (not Open, which
// would migrate it) and returns the (vendor, session_id, request_id) key of
// every row in events, plus the row count.
func rawEventKeys(t *testing.T, path string) (map[string]bool, int) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw %s: %v", path, err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT vendor, session_id, request_id FROM events`)
	if err != nil {
		t.Fatalf("query raw events in %s: %v", path, err)
	}
	defer rows.Close()
	keys := map[string]bool{}
	for rows.Next() {
		var v, s, r string
		if err := rows.Scan(&v, &s, &r); err != nil {
			t.Fatalf("scan raw event in %s: %v", path, err)
		}
		keys[v+"|"+s+"|"+r] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read raw events in %s: %v", path, err)
	}
	return keys, len(keys)
}

// TestMigrateRealV01Store is S1's own acceptance test: copy a real v0.1
// store from this laptop (DefaultPath, a database created before this
// session's migrations existed, so it has no schema_version table and no
// owner column) into a temp dir, run it to head through Open, and check
// every one of its events survived with identical (vendor, session_id,
// request_id) ids and that schema_version now reads 2. Skips (does not
// fail) when this laptop has no such store, or when it is locked by a
// running burnmon/burnmon-cli process, since neither says anything about
// whether the migration code is correct.
func TestMigrateRealV01Store(t *testing.T) {
	real, err := DefaultPath()
	if err != nil {
		t.Skip("no default store path resolvable on this platform")
	}
	if _, err := os.Stat(real); err != nil {
		t.Skipf("no real store at %s to migrate: %v", real, err)
	}

	dir := t.TempDir()
	dst := filepath.Join(dir, "burnmon.db")
	if err := copyFile(real, dst); err != nil {
		t.Skipf("could not copy the real store at %s (likely locked by a running app): %v", real, err)
	}

	wantKeys, wantCount := rawEventKeys(t, dst)
	if wantCount == 0 {
		t.Skip("real store has no events to check")
	}

	st, err := Open(dst)
	if err != nil {
		t.Fatalf("Open (migrate) the copied v0.1 store: %v", err)
	}
	defer st.Close()

	var version int
	if err := st.db.QueryRow(`SELECT version FROM schema_version LIMIT 1`).Scan(&version); err != nil {
		t.Fatalf("read schema_version after migrate: %v", err)
	}
	if version != 5 {
		t.Fatalf("schema_version after migrate = %d, want 5", version)
	}

	got, err := st.AllEvents()
	if err != nil {
		t.Fatalf("AllEvents after migrate: %v", err)
	}
	if len(got) != wantCount {
		t.Fatalf("event count after migrate = %d, want %d (every v0.1 event must survive)", len(got), wantCount)
	}
	for _, e := range got {
		key := e.Vendor + "|" + e.SessionID + "|" + e.RequestID
		if !wantKeys[key] {
			t.Fatalf("event %s not present in the pre-migration v0.1 store", key)
		}
		if e.Owner != "" {
			t.Fatalf("event %s Owner = %q, want empty (migration 2's default before any reown)", key, e.Owner)
		}
	}
}

// TestReownEvents guards `burnmon-cli reown`: every event's owner is
// recomputed from its stored Project, only rows whose owner actually
// changes are written, and a repeat call with the same rules is a no-op.
func TestReownEvents(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	base := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	events := []schema.Event{
		{Vendor: "anthropic", Agent: "claude-code", Surface: "cli", SessionID: "s1", RequestID: "r1",
			Model: "claude-x", At: base, Input: 1, Output: 1, Project: `C:\ZND\projects\burnmon`},
		{Vendor: "anthropic", Agent: "claude-code", Surface: "cli", SessionID: "s2", RequestID: "r2",
			Model: "claude-x", At: base, Input: 1, Output: 1, Project: `C:\dev\Work\other`, Owner: "personal"},
	}
	if err := st.UpsertEvents(events); err != nil {
		t.Fatal(err)
	}

	ownerFor := func(project string) string {
		if strings.HasPrefix(strings.ToLower(project), `c:\znd\`) {
			return "ZND"
		}
		return "Valona"
	}
	n, err := st.ReownEvents(ownerFor)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("ReownEvents updated %d row(s), want 2 (both differ from their stored owner)", n)
	}

	got, err := st.AllEvents()
	if err != nil {
		t.Fatal(err)
	}
	bySession := map[string]schema.Event{}
	for _, e := range got {
		bySession[e.SessionID] = e
	}
	if bySession["s1"].Owner != "ZND" {
		t.Fatalf("s1 Owner = %q, want ZND", bySession["s1"].Owner)
	}
	if bySession["s2"].Owner != "Valona" {
		t.Fatalf("s2 Owner = %q, want Valona", bySession["s2"].Owner)
	}

	n2, err := st.ReownEvents(ownerFor)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Fatalf("second ReownEvents updated %d row(s), want 0 (already reowned)", n2)
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

// TestForecastScoresRoundTrip guards F1's store layer: InsertForecastPlan
// never overwrites a week already on record, RecordForecastActual only
// fills a week in once, and DailyTokenTotals buckets by UTC calendar day.
func TestForecastScoresRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ws := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC) // Monday
	isoYear, isoWeek := ws.ISOWeek()

	if _, ok, err := st.ForecastScore(isoYear, isoWeek); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("ForecastScore found a row before any was inserted")
	}

	if err := st.InsertForecastPlan(isoYear, isoWeek, ws, 4900); err != nil {
		t.Fatal(err)
	}
	// A later call for the same week must not overwrite the plan already
	// recorded ("the forecast made on Monday", never revised).
	if err := st.InsertForecastPlan(isoYear, isoWeek, ws, 9999); err != nil {
		t.Fatal(err)
	}

	sc, ok, err := st.ForecastScore(isoYear, isoWeek)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ForecastScore: row not found after insert")
	}
	if sc.PlanTokens != 4900 {
		t.Fatalf("PlanTokens = %d, want 4900 (second insert must not overwrite)", sc.PlanTokens)
	}
	if sc.ActualTokens != nil {
		t.Fatalf("ActualTokens = %v, want nil before scoring", sc.ActualTokens)
	}

	scoredAt := ws.AddDate(0, 0, 8)
	if err := st.RecordForecastActual(isoYear, isoWeek, 7000, scoredAt); err != nil {
		t.Fatal(err)
	}
	// A second scoring pass over the same week must not change the actual
	// already recorded.
	if err := st.RecordForecastActual(isoYear, isoWeek, 1, scoredAt); err != nil {
		t.Fatal(err)
	}

	scores, err := st.ForecastScores()
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 1 {
		t.Fatalf("len(ForecastScores) = %d, want 1", len(scores))
	}
	got := scores[0]
	if got.ActualTokens == nil || *got.ActualTokens != 7000 {
		t.Fatalf("ActualTokens = %v, want 7000", got.ActualTokens)
	}
	if got.ScoredAt == nil {
		t.Fatal("ScoredAt = nil, want set")
	}

	events := []schema.Event{
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s1", RequestID: "r1",
			Model: "m", At: ws.Add(9 * time.Hour), Input: 300},
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s1", RequestID: "r2",
			Model: "m", At: ws.Add(20 * time.Hour), Input: 200},
		{Vendor: "anthropic", Agent: "claude-code", SessionID: "s1", RequestID: "r3",
			Model: "m", At: ws.AddDate(0, 0, 1).Add(9 * time.Hour), Input: 50},
	}
	if err := st.UpsertEvents(events); err != nil {
		t.Fatal(err)
	}
	daily, err := st.DailyTokenTotals(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(daily) != 2 {
		t.Fatalf("len(DailyTokenTotals) = %d, want 2 (two distinct days)", len(daily))
	}
	if daily[0].Date != ws.Format("2006-01-02") || daily[0].Tokens != 500 {
		t.Fatalf("day 0 = %+v, want {%s 500}", daily[0], ws.Format("2006-01-02"))
	}
	if daily[1].Tokens != 50 {
		t.Fatalf("day 1 tokens = %d, want 50", daily[1].Tokens)
	}
}
