package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseThreeTurnsFixture(t *testing.T) {
	a := Adapter{}
	path := filepath.Join("..", "..", "..", "testdata", "codex", "three-turns.jsonl")

	events, offset, err := a.Parse(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	fi, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if offset != fi.Size() {
		t.Fatalf("offset = %d, want file size %d (fully consumed)", offset, fi.Size())
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3 (one per token_count line)", len(events))
	}

	sessionID := "three-turns"
	wantRequestIDs := map[string]bool{
		sessionID + ":2": true,
		sessionID + ":4": true,
		sessionID + ":6": true,
	}

	var withRateLimits int
	for _, e := range events {
		if e.Vendor != "openai" {
			t.Fatalf("Vendor = %q, want openai", e.Vendor)
		}
		if e.Agent != "codex" {
			t.Fatalf("Agent = %q, want codex", e.Agent)
		}
		if e.Surface != "cli" {
			t.Fatalf("Surface = %q, want cli (originator codex-tui)", e.Surface)
		}
		if e.SessionID != sessionID {
			t.Fatalf("SessionID = %q, want %q", e.SessionID, sessionID)
		}
		if !wantRequestIDs[e.RequestID] {
			t.Fatalf("unexpected RequestID %q", e.RequestID)
		}
		if e.Project != `C:\ZND\projects\burnmon` {
			t.Fatalf("Project = %q, want the session cwd", e.Project)
		}
		if e.CacheWrite != nil {
			t.Fatalf("CacheWrite = %v, want nil (Codex has no cache-write class)", e.CacheWrite)
		}
		if e.WindowUsed != nil {
			withRateLimits++
		}

		switch e.RequestID {
		case sessionID + ":2":
			if e.Model != "gpt-5.6-terra" {
				t.Fatalf("turn 1 Model = %q, want gpt-5.6-terra", e.Model)
			}
			if e.Input != 6000 { // 10000 - 4000 cached
				t.Fatalf("turn 1 Input = %d, want 6000 (fresh = input - cached)", e.Input)
			}
			if e.CacheRead == nil || *e.CacheRead != 4000 {
				t.Fatalf("turn 1 CacheRead = %v, want 4000", e.CacheRead)
			}
			if e.Output != 100 {
				t.Fatalf("turn 1 Output = %d, want 100", e.Output)
			}
			if e.Reasoning == nil || *e.Reasoning != 10 {
				t.Fatalf("turn 1 Reasoning = %v, want 10", e.Reasoning)
			}
			if e.WindowUsed == nil || *e.WindowUsed != 5.0 {
				t.Fatalf("turn 1 WindowUsed = %v, want 5.0 (the 300-minute, i.e. 5h, window)", e.WindowUsed)
			}
			if e.WindowReset == nil {
				t.Fatal("turn 1 WindowReset = nil, want the primary window's resets_at")
			}
		case sessionID + ":4":
			if e.Model != "gpt-5.6-terra" {
				t.Fatalf("turn 2 Model = %q, want gpt-5.6-terra", e.Model)
			}
			if e.Input != 4000 { // 15000 - 11000 cached, from last_token_usage
				t.Fatalf("turn 2 Input = %d, want 4000", e.Input)
			}
			if e.CacheRead == nil || *e.CacheRead != 11000 {
				t.Fatalf("turn 2 CacheRead = %v, want 11000", e.CacheRead)
			}
			if e.Output != 200 {
				t.Fatalf("turn 2 Output = %d, want 200", e.Output)
			}
			if e.WindowUsed != nil {
				t.Fatalf("turn 2 WindowUsed = %v, want nil (no rate_limits on this line)", e.WindowUsed)
			}
		case sessionID + ":6":
			if e.Model != "gpt-6-astra" {
				t.Fatalf("turn 3 Model = %q, want gpt-6-astra (model switched mid-session)", e.Model)
			}
			if e.Input != 10000 { // 15000 - 5000 cached
				t.Fatalf("turn 3 Input = %d, want 10000", e.Input)
			}
			if e.Reasoning != nil {
				t.Fatalf("turn 3 Reasoning = %v, want nil (0 reasoning tokens)", e.Reasoning)
			}
		}
	}
	if withRateLimits != 1 {
		t.Fatalf("events with WindowUsed set = %d, want 1 (fixture has one rate_limits object)", withRateLimits)
	}
}

func TestParseResumesFromOffset(t *testing.T) {
	a := Adapter{}
	path := filepath.Join("..", "..", "..", "testdata", "codex", "three-turns.jsonl")

	first, off1, err := a.Parse(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 3 {
		t.Fatalf("first pass: got %d events, want 3", len(first))
	}
	second, off2, err := a.Parse(path, off1)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Fatalf("second pass from a fully-consumed offset: got %d events, want 0", len(second))
	}
	if off2 != off1 {
		t.Fatalf("offset moved on an empty read: %d != %d", off2, off1)
	}
}

func TestNameAndRoots(t *testing.T) {
	a := Adapter{}
	if a.Name() != "codex" {
		t.Fatalf("Name() = %q, want codex", a.Name())
	}
	_ = a.Roots() // must not panic; a fresh CI box legitimately has zero folders
}
