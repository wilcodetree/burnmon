package codex

import (
	"os"
	"path/filepath"
	"strings"
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

// TestParseIncrementalReadKeepsSurface guards F2's fix: session_meta (the
// only line carrying originator) is written once, near the top of a
// rollout. Before scanHeaderMeta, an incremental Parse call starting after
// that line (exactly what the live watcher does on every turn after the
// first) reported surface "unknown" for every subsequent turn, the "surface
// UNKNOWN" bug from the v0.1.1 fix spec.
func TestParseIncrementalReadKeepsSurface(t *testing.T) {
	a := Adapter{}
	src := filepath.Join("..", "..", "..", "testdata", "codex", "three-turns.jsonl")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "live.jsonl")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Consume the whole fixture first, as the initial full backfill would.
	_, off1, err := a.Parse(path, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate one more live turn appended to the file: a fsnotify Write
	// event fires and the watcher calls Parse from the cursor it already
	// has, never seeing session_meta again.
	nextTurn := `{"timestamp":"2026-09-16T20:23:00.000Z","ordinal":7,"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"cache_write_input_tokens":0,"output_tokens":50,"reasoning_output_tokens":0,"total_tokens":1050},"model_context_window":272000}}}` + "\n"
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(nextTurn); err != nil {
		t.Fatal(err)
	}
	f.Close()

	events, _, err := a.Parse(path, off1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events on the incremental read, want 1", len(events))
	}
	if events[0].Surface != "cli" {
		t.Fatalf("Surface = %q, want cli (session_meta re-read via scanHeaderMeta)", events[0].Surface)
	}
	if events[0].Project != `C:\ZND\projects\burnmon` {
		t.Fatalf("Project = %q, want the session cwd re-read via scanHeaderMeta", events[0].Project)
	}
}

// TestParseSubRunModelBecomesTitle guards F2's second label fix: a Codex
// sub-run thread (observed live: an auto-review guardian sub-agent) writes
// its own name, not a real model id, into turn_context.payload.model. Its
// session_meta carries a non-"user" thread_source; that name must land in
// Title, and Model must stay empty rather than showing a fabricated model
// like "codex-auto-review".
func TestParseSubRunModelBecomesTitle(t *testing.T) {
	a := Adapter{}
	dir := t.TempDir()
	path := filepath.Join(dir, "subrun.jsonl")
	lines := []string{
		`{"timestamp":"2026-09-22T10:41:36.550Z","ordinal":0,"type":"session_meta","payload":{"session_id":"01a0c8b4","id":"01a0c8b4","cwd":"C:\\dev\\Work","originator":"codex-tui","cli_version":"0.154.0","source":"cli","thread_source":"guardian_review","model_provider":"openai"}}`,
		`{"timestamp":"2026-09-22T10:41:37.529Z","type":"turn_context","payload":{"turn_id":"t1","cwd":"C:\\dev\\Work","model":"codex-auto-review"}}`,
		`{"timestamp":"2026-09-22T10:41:40.000Z","ordinal":2,"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":500,"cached_input_tokens":0,"cache_write_input_tokens":0,"output_tokens":20,"reasoning_output_tokens":0,"total_tokens":520}}}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	events, _, err := a.Parse(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	e := events[0]
	if e.Model != "" {
		t.Fatalf("Model = %q, want empty (sub-run name is not a model)", e.Model)
	}
	if e.Title != "codex-auto-review" {
		t.Fatalf("Title = %q, want codex-auto-review", e.Title)
	}
}

func TestNameAndRoots(t *testing.T) {
	a := Adapter{}
	if a.Name() != "codex" {
		t.Fatalf("Name() = %q, want codex", a.Name())
	}
	_ = a.Roots() // must not panic; a fresh CI box legitimately has zero folders
}
