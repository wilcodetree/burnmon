//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"burnmon/internal/dataset"
	"burnmon/internal/live"
	"burnmon/internal/pricing"
	"burnmon/internal/store"
	"burnmon/internal/watch"
)

func TestAppRebuildNoticeNoWSL(t *testing.T) {
	got := appRebuildNotice(15*time.Minute, 4*time.Hour, nil)
	want := `<b>This window keeps itself current.</b> It opens straight to your last snapshot and quietly catches up in the background within moments. It also re-reads your session transcripts every 15 minutes while open, whenever you press Refresh now (top right), and right after you save changes in Settings (the gear icon).`
	if got != want {
		t.Errorf("appRebuildNotice with no WSL distros changed the byte-for-byte text.\ngot:  %s\nwant: %s", got, want)
	}
}

func TestAppRebuildNoticeWithWSL(t *testing.T) {
	got := appRebuildNotice(15*time.Minute, 4*time.Hour, []string{"Ubuntu"})
	for _, want := range []string{"15 minutes", "Ubuntu", "4 hours", "WSL included"} {
		if !strings.Contains(got, want) {
			t.Errorf("appRebuildNotice with WSL distros missing %q in: %s", want, got)
		}
	}

	multi := appRebuildNotice(15*time.Minute, 30*time.Minute, []string{"Ubuntu", "Debian"})
	if !strings.Contains(multi, "Ubuntu, Debian") {
		t.Errorf("appRebuildNotice did not join multiple distro names: %s", multi)
	}
	if !strings.Contains(multi, "30 minutes") {
		t.Errorf("appRebuildNotice did not format a sub-hour wsl interval in minutes: %s", multi)
	}
}

// TestLiveWatchCodexTurnWithinTwoSeconds guards F2's latency fix end to
// end, the way the spec asks: append one token_count line to a rollout
// already being watched, and the Now page's snapshot must pick it up
// within 2 seconds. Exercises the real pipeline (watch.Watcher -> fsnotify
// -> dataset.Cache.IngestFile -> store -> live.BuildSnapshot), not a mock.
func TestLiveWatchCodexTurnWithinTwoSeconds(t *testing.T) {
	dir := t.TempDir()
	dayDir := filepath.Join(dir, "2026", "09", "22")
	if err := os.MkdirAll(dayDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dayDir, "rollout-2026-09-22T12-00-00-testsession.jsonl")

	base := `{"timestamp":"2026-09-22T12:00:00.000Z","ordinal":0,"type":"session_meta","payload":{"cwd":"C:\\proj","originator":"codex-tui","thread_source":"user"}}` + "\n" +
		`{"timestamp":"2026-09-22T12:00:01.000Z","type":"turn_context","payload":{"cwd":"C:\\proj","model":"gpt-5.6-sol"}}` + "\n"
	if err := os.WriteFile(path, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := store.Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	cache := &dataset.Cache{Store: st}
	cache.SeedNativeRoots(nil, []string{dir})
	// Ingest the base content first, as the initial backfill would, so the
	// live turn below is a genuine incremental (from>0) read: exactly the
	// path that used to lose the session_meta-derived surface (F2).
	if err := cache.IngestFile(path); err != nil {
		t.Fatal(err)
	}

	changed := make(chan string, 4)
	wt, err := watch.New([]string{dir}, nil, func(p string) {
		if err := cache.IngestFile(p); err != nil {
			t.Errorf("IngestFile(%s): %v", p, err)
			return
		}
		changed <- p
	})
	if err != nil {
		t.Fatal(err)
	}
	wt.Start()
	defer wt.Stop()

	// Append one live turn, timestamped now so BuildSnapshot's running
	// window picks it up.
	now := time.Now().UTC()
	turn := fmt.Sprintf(
		`{"timestamp":%q,"ordinal":2,"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":50}}}}`+"\n",
		now.Format(time.RFC3339Nano))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(turn); err != nil {
		t.Fatal(err)
	}
	f.Close()

	select {
	case <-changed:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not fire an onChange for the appended turn within 2 seconds")
	}

	cfg := pricing.Defaults()
	events, err := st.EventsSince(now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	snap := live.BuildSnapshot(events, &cfg, time.Now())
	if len(snap.Sessions) != 1 {
		t.Fatalf("snapshot sessions = %d, want 1 within 2 seconds of the appended turn", len(snap.Sessions))
	}
	if snap.Sessions[0].Surface != "cli" {
		t.Fatalf("Surface = %q, want cli (session_meta re-read on the incremental parse)", snap.Sessions[0].Surface)
	}
}
