package dataset

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"burnmon/internal/pricing"
	"burnmon/internal/store"
)

func TestDedupeCaseInsensitive(t *testing.T) {
	in := []string{
		`C:\Users\wilco\.claude\projects`,
		`c:\users\wilco\.claude\projects`,
		`C:\Users\wilco\AppData\Roaming\Claude\local-agent-mode-sessions`,
		`\\wsl.localhost\Ubuntu\home\wilco\.claude\projects`,
		`\\WSL.LOCALHOST\Ubuntu\home\wilco\.claude\projects`,
	}
	want := []string{
		`C:\Users\wilco\.claude\projects`,
		`C:\Users\wilco\AppData\Roaming\Claude\local-agent-mode-sessions`,
		`\\wsl.localhost\Ubuntu\home\wilco\.claude\projects`,
	}
	got := dedupeCaseInsensitive(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedupeCaseInsensitive(%v) = %v, want %v", in, got, want)
	}
}

func TestAdapterForPathClassifiesByRoots(t *testing.T) {
	roots := map[string][]string{
		"claude": {`C:\Users\x\.claude\projects`},
		"codex":  {`C:\Users\x\.codex\sessions`},
	}
	got := adapterForPath(`C:\Users\x\.codex\sessions\2026\09\20\rollout-1.jsonl`, roots)
	if got == nil || got.Name() != "codex" {
		t.Fatalf("got %v, want codex adapter", got)
	}
	got2 := adapterForPath(`C:\Users\x\.claude\projects\proj\sess.jsonl`, roots)
	if got2 == nil || got2.Name() != "claude" {
		t.Fatalf("got %v, want claude adapter", got2)
	}
	got3 := adapterForPath(`C:\Users\x\somewhere-else\file.jsonl`, roots)
	if got3 != nil {
		t.Fatalf("got %v, want nil for a path under no known root", got3)
	}
}

func TestAdapterForPathFallsBackToClaudeWhenRootsEmpty(t *testing.T) {
	got := adapterForPath(`C:\anything.jsonl`, nil)
	if got == nil || got.Name() != "claude" {
		t.Fatalf("got %v, want claude fallback when roots is empty", got)
	}
}

// TestIngestDispatchesToCodexAdapter exercises the real store round trip
// (ingest -> UpsertEvents -> AllEvents) for a Codex trail, without touching
// this machine's real ~/.codex or ~/.claude folders: RootsByAdapter is set
// directly rather than going through resolveSources's auto-detection, so
// the test stays hermetic.
func TestIngestDispatchesToCodexAdapter(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	codexRoot := filepath.Join(dir, "codex-sessions")
	if err := os.MkdirAll(codexRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "codex", "three-turns.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(codexRoot, "rollout-fixture.jsonl")
	if err := os.WriteFile(dst, fixture, 0o644); err != nil {
		t.Fatal(err)
	}

	c := Cache{Store: st, RootsByAdapter: map[string][]string{"codex": {codexRoot}}}
	if err := c.ingest([]string{dst}, nil, false, nil); err != nil {
		t.Fatal(err)
	}

	events, err := st.AllEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	for _, e := range events {
		if e.Vendor != "openai" || e.Agent != "codex" {
			t.Fatalf("event %+v, want vendor openai / agent codex", e)
		}
		if e.SessionID != "rollout-fixture" {
			t.Fatalf("SessionID = %q, want rollout-fixture (the file basename)", e.SessionID)
		}
	}
}

func TestCollectFromStoreMatchesFixture(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srcDir := filepath.Join(dir, "projects")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("..", "adapter", "claude", "testdata", "basic.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sess-fixture.jsonl"), fixture, 0o644); err != nil {
		t.Fatal(err)
	}

	c := Cache{Store: st}
	cfg := pricing.Defaults()
	payload, err := c.Collect(&cfg, CollectOpts{Seat: "Standard", MonthsN: 24, Sources: []string{srcDir}, RefreshSlow: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if payload.Totals.Sessions != 1 {
		t.Fatalf("Totals.Sessions = %d, want 1", payload.Totals.Sessions)
	}
	if len(payload.Sessions) != 1 || payload.Sessions[0].SessionID != "sess-fixture" {
		t.Fatalf("got sessions %+v, want one session-fixture", payload.Sessions)
	}

	// A second Collect call against the same store and unchanged file must
	// not double the totals: the cursor should have marked it fully read.
	payload2, err := c.Collect(&cfg, CollectOpts{Seat: "Standard", MonthsN: 24, Sources: []string{srcDir}, RefreshSlow: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if payload2.Sessions[0].Calls != payload.Sessions[0].Calls {
		t.Fatalf("second Collect changed Calls: %d != %d (cursor did not dedupe the re-read)",
			payload2.Sessions[0].Calls, payload.Sessions[0].Calls)
	}
}
