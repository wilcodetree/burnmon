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
