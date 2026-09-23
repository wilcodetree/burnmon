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

	events, _, offset, err := a.Parse(path, 0)
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

	first, _, off1, err := a.Parse(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 3 {
		t.Fatalf("first pass: got %d events, want 3", len(first))
	}
	second, _, off2, err := a.Parse(path, off1)
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
	_, _, off1, err := a.Parse(path, 0)
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

	events, _, _, err := a.Parse(path, off1)
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
	// N6 (v0.2.2, SESSION_LOG.md): same bug as Surface/Project above, for
	// Model. The fixture's last turn_context (before the appended line) set
	// gpt-6-astra; scanHeaderMeta must recover that, not "".
	if events[0].Model != "gpt-6-astra" {
		t.Fatalf("Model = %q, want gpt-6-astra (turn_context re-read via scanHeaderMeta)", events[0].Model)
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

	events, _, _, err := a.Parse(path, 0)
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

// TestParseToolCalls guards S2's tool_calls extraction for Codex: both
// payload families carry real tool calls (SESSION_LOG.md), custom_tool_call
// (the shell/exec activity Codex sessions mostly consist of) and the rarer
// function_call, each paired with its own *_output by call_id.
func TestParseToolCalls(t *testing.T) {
	a := Adapter{}
	path := filepath.Join("..", "..", "..", "testdata", "codex", "tool-calls.jsonl")

	_, toolCalls, _, err := a.Parse(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(toolCalls) != 2 {
		t.Fatalf("got %d tool calls, want 2", len(toolCalls))
	}

	byID := map[string]struct {
		tool       string
		inputBytes int64
		resultLen  int64
	}{
		"call_shell1": {"exec", int64(len(`{"cmd":"go build ./..."}`)), int64(len("build ok\n"))},
		"call_ask1":   {"request_user_input_async", int64(len(`{"question":"proceed?"}`)), int64(len(`{"accepted":true}`))},
	}
	for _, tc := range toolCalls {
		want, ok := byID[tc.CallID]
		if !ok {
			t.Fatalf("unexpected CallID %q", tc.CallID)
		}
		if tc.Tool != want.tool {
			t.Fatalf("%s Tool = %q, want %q", tc.CallID, tc.Tool, want.tool)
		}
		if tc.Turn != "turn-1" {
			t.Fatalf("%s Turn = %q, want turn-1", tc.CallID, tc.Turn)
		}
		if tc.Vendor != "openai" || tc.Agent != "codex" {
			t.Fatalf("%s Vendor/Agent = %q/%q, want openai/codex", tc.CallID, tc.Vendor, tc.Agent)
		}
		if tc.At.IsZero() {
			t.Fatalf("%s At is zero, want the call line's timestamp", tc.CallID)
		}
		if tc.InputBytes != want.inputBytes {
			t.Fatalf("%s InputBytes = %d, want %d", tc.CallID, tc.InputBytes, want.inputBytes)
		}
		if tc.ResultBytes == nil || *tc.ResultBytes != want.resultLen {
			t.Fatalf("%s ResultBytes = %v, want %d", tc.CallID, tc.ResultBytes, want.resultLen)
		}
	}
}

// TestFilePathFromArgs checks I3's best-effort file path extraction: a
// function_call's JSON arguments carrying file_path or path decode; a
// custom_tool_call's plain shell-command string (not JSON at all) safely
// yields no path rather than an error.
func TestFilePathFromArgs(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"file_path key", `{"file_path":"C:\\ZND\\foo.go","content":"x"}`, `C:\ZND\foo.go`},
		{"path key", `{"path":"bar.txt"}`, "bar.txt"},
		{"not json (shell command)", `go build ./...`, ""},
		{"json with neither key", `{"cmd":"go build ./..."}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := filePathFromArgs(tc.raw); got != tc.want {
				t.Fatalf("filePathFromArgs(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestNameAndRoots(t *testing.T) {
	a := Adapter{}
	if a.Name() != "codex" {
		t.Fatalf("Name() = %q, want codex", a.Name())
	}
	_ = a.Roots() // must not panic; a fresh CI box legitimately has zero folders
}
