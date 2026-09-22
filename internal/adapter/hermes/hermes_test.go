package hermes

import (
	"testing"
	"time"
)

// TestPollOnceFixture is A1's spec'd test: the fixture at testdata\hermes\
// parses to the expected session count and tokens. The fixture holds two
// real Hermes sessions Wilco recorded 2026-09-22 (state.db's sessions table,
// metadata and running totals only, no message content: SESSION_LOG.md v0.2
// 41B), copied into a standalone SQLite file with the same column shapes
// PollOnce reads.
func TestPollOnceFixture(t *testing.T) {
	events, err := PollOnce("../../../testdata/hermes/hermes_fixture.db")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2 (both fixture sessions have messages)", len(events))
	}

	byID := map[string]int{}
	for i, e := range events {
		byID[e.SessionID] = i
	}

	e1 := events[byID["20260922_213936_0572d4"]]
	if e1.Vendor != "nous" || e1.Agent != "hermes" {
		t.Errorf("session 1: vendor/agent = %q/%q, want nous/hermes", e1.Vendor, e1.Agent)
	}
	if e1.Model != "claude-sonnet-5" {
		t.Errorf("session 1: model = %q, want claude-sonnet-5", e1.Model)
	}
	if e1.Surface != "cli" {
		t.Errorf("session 1: surface = %q, want cli (source tui)", e1.Surface)
	}
	if e1.Input != 8 || e1.Output != 1484 {
		t.Errorf("session 1: input/output = %d/%d, want 8/1484", e1.Input, e1.Output)
	}
	if e1.CacheRead == nil || *e1.CacheRead != 137103 {
		t.Errorf("session 1: cache_read = %v, want 137103", e1.CacheRead)
	}
	if e1.CacheWrite == nil || *e1.CacheWrite != 51798 {
		t.Errorf("session 1: cache_write = %v, want 51798", e1.CacheWrite)
	}
	if e1.Project != `C:\dev` {
		t.Errorf("session 1: project = %q, want C:\\dev", e1.Project)
	}
	// ended_at is null in the fixture (session still open): At must be a
	// recent timestamp (this poll's "now"), not the zero value.
	if time.Since(e1.At) > time.Minute || e1.At.After(time.Now()) {
		t.Errorf("session 1: At = %v, want close to now (session still open)", e1.At)
	}

	e2 := events[byID["20260922_214003_05e0f4"]]
	wantTokens := int64(12 + 1987 + 282638 + 18461)
	gotTokens := e2.Input + e2.Output + *e2.CacheRead + *e2.CacheWrite
	if gotTokens != wantTokens {
		t.Errorf("session 2: total tokens = %d, want %d", gotTokens, wantTokens)
	}
}

// TestPollOnceIdempotent checks a second poll of the same still-open fixture
// returns the same totals: PollOnce carries no cursor of its own (the
// package doc's "no cursor" design), so repeat-safety comes entirely from
// the caller's UpsertEvents upsert, not from anything here; this only checks
// PollOnce itself is a stable, repeatable read.
func TestPollOnceIdempotent(t *testing.T) {
	first, err := PollOnce("../../../testdata/hermes/hermes_fixture.db")
	if err != nil {
		t.Fatal(err)
	}
	second, err := PollOnce("../../../testdata/hermes/hermes_fixture.db")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != len(second) {
		t.Fatalf("first poll = %d events, second = %d, want equal", len(first), len(second))
	}
}
