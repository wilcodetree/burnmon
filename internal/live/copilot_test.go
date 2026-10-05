package live

import (
	"path/filepath"
	"testing"
	"time"

	"burnmon/internal/pricing"
	"burnmon/internal/schema"
	"burnmon/internal/store"
)

// copilotShape is a synthetic sequence shaped like the real Copilot in VS Code
// trail (measured 2026-10-05 on the real store: 435 of 520 requests are
// helper-sized, 6 of 7 sessions end on one, and no tool call is ever
// recorded): a main-model request with a large context, then small helper
// requests on another model, newest last. Nothing here comes from a real
// session.
func copilotShape(agent string, now time.Time) []schema.Event {
	ev := func(key string, sec int, model string, in int64) schema.Event {
		return schema.Event{Vendor: "github", Agent: agent, Surface: "vscode", SessionID: "sess",
			RequestID: key, Model: model, At: now.Add(time.Duration(-sec) * time.Second), Input: in, Output: 20}
	}
	return []schema.Event{
		ev("m1", 120, "synthetic-main", 90000),
		ev("m2", 40, "synthetic-main", 131000),
		ev("h1", 6, "synthetic-helper", 1700),
		ev("h2", 3, "synthetic-helper", 253),
	}
}

func copilotConfig() *pricing.Config {
	cfg := testConfig()
	cfg.ContextWindows = map[string]int64{"synthetic-main": 200000, "synthetic-helper": 128000}
	return cfg
}

// TestCopilotContextIgnoresHelperRequests: the session's context is the
// conversation's, not the title helper's 253 tokens.
func TestCopilotContextIgnoresHelperRequests(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	cfg := copilotConfig()
	snap := BuildSnapshot(copilotShape("copilot-vscode", now), cfg, now)
	s := snap.Sessions[0]
	if s.Context != 131000 {
		t.Errorf("Context = %d, want the main request's 131000", s.Context)
	}
	if s.ContextWindow != 200000 {
		t.Errorf("ContextWindow = %d, want the main model's 200000", s.ContextWindow)
	}
}

// TestCopilotContextAllHelperFallsBack: a session with only small requests
// keeps the newest turn's context, as before.
func TestCopilotContextAllHelperFallsBack(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	events := copilotShape("copilot-vscode", now)[2:]
	snap := BuildSnapshot(events, copilotConfig(), now)
	if got := snap.Sessions[0].Context; got != 253 {
		t.Errorf("Context = %d, want the newest turn's 253", got)
	}
}

// TestOtherAgentsContextStaysLastTurn: the rule is Copilot-only; the same
// shape under Claude Code still reads the newest turn.
func TestOtherAgentsContextStaysLastTurn(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	events := copilotShape("claude-code", now)
	for i := range events {
		events[i].Vendor = "anthropic"
	}
	snap := BuildSnapshot(events, copilotConfig(), now)
	s := snap.Sessions[0]
	if s.Context != 253 || s.ContextWindow != 128000 {
		t.Errorf("Context %d window %d, want 253 and 128000", s.Context, s.ContextWindow)
	}
}

// TestCopilotStageBetweenRequests: an active Copilot session whose newest
// request landed 3 s ago is not "Waiting on you"; the same silence under
// Claude Code (a turn that called no tool) still is.
func TestCopilotStageBetweenRequests(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	cfg := copilotConfig()
	st, err := store.Open(filepath.Join(t.TempDir(), "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, c := range []struct {
		agent, vendor, want string
	}{
		{"copilot-vscode", "github", "thinking"},
		{"claude-code", "anthropic", "waiting"},
		{"copilot-cli", "github", "waiting"},
	} {
		events := copilotShape(c.agent, now)
		if c.agent != "copilot-vscode" {
			// Control agents get a flat context so no compaction finding
			// fires and the stage is decided by the no-tool rule alone.
			events = events[:0]
			for i, sec := range []int{40, 3} {
				events = append(events, schema.Event{Agent: c.agent, Surface: "cli", RequestID: string(rune('a' + i)),
					Model: "synthetic-main", At: now.Add(time.Duration(-sec) * time.Second), Input: 1000, Output: 20})
			}
		}
		for i := range events {
			events[i].Vendor = c.vendor
			events[i].SessionID = c.agent
		}
		if err := st.UpsertEvents(events); err != nil {
			t.Fatal(err)
		}
		snap := BuildSnapshot(events, cfg, now)
		if err := ApplySessionTotals(snap.Sessions, st, cfg); err != nil {
			t.Fatal(err)
		}
		if err := ApplyStages(snap.Sessions, st, now); err != nil {
			t.Fatal(err)
		}
		if got := snap.Sessions[0].Stage; got != c.want {
			t.Errorf("%s: stage %q, want %q", c.agent, got, c.want)
		}
	}
}
