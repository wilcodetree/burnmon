package copilotvsc

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const fixturePath = "../../../testdata/copilotvsc/copilotvsc_fixture.jsonl"

func TestPollOnce_EmptyPath(t *testing.T) {
	events, err := PollOnce("")
	if err != nil {
		t.Fatalf("PollOnce(\"\"): %v", err)
	}
	if events != nil {
		t.Fatalf("PollOnce(\"\") = %v, want nil", events)
	}
}

func TestPollOnce_MissingFile(t *testing.T) {
	events, err := PollOnce(`C:\does\not\exist\copilot-otel.jsonl`)
	if err != nil {
		t.Fatalf("PollOnce(missing): %v", err)
	}
	if events != nil {
		t.Fatalf("PollOnce(missing) = %v, want nil", events)
	}
}

// TestPollOnce_Fixture parses the real (stripped) span file and checks the
// facts SESSION_LOG.md's A4 entry documents: ten distinct requests (13
// inference lines, one response id repeated four times), the repeated one
// resolved to its last line in file order (not its largest output), two
// distinct sessions grouping the requests that follow each session.start,
// and github/copilot-vscode/vscode on every event.
func TestPollOnce_Fixture(t *testing.T) {
	events, err := PollOnce(fixturePath)
	if err != nil {
		t.Fatalf("PollOnce(fixture): %v", err)
	}
	if len(events) != 10 {
		t.Fatalf("len(events) = %d, want 10", len(events))
	}

	bySessionCount := map[string]int{}
	var repeated *eventByRequest
	for i := range events {
		e := events[i]
		if e.Vendor != "github" || e.Agent != "copilot-vscode" || e.Surface != "vscode" {
			t.Fatalf("event %d: vendor/agent/surface = %q/%q/%q, want github/copilot-vscode/vscode", i, e.Vendor, e.Agent, e.Surface)
		}
		bySessionCount[e.SessionID]++
		if e.RequestID == "dc0615ba-1c0a-4396-a2da-3456c613e556" {
			repeated = &eventByRequest{input: e.Input, output: e.Output, model: e.Model}
		}
	}

	if repeated == nil {
		t.Fatal("did not find the repeated request id dc0615ba-...")
	}
	// Last line in file order for that id (not the largest-output one,
	// which would also be 1197 here by coincidence): input 103664, output
	// 1197, per the fixture's own last occurrence.
	if repeated.input != 103664 || repeated.output != 1197 || repeated.model != "claude-sonnet-5" {
		t.Fatalf("repeated request resolved to input=%d output=%d model=%q, want 103664/1197/claude-sonnet-5",
			repeated.input, repeated.output, repeated.model)
	}

	if got := bySessionCount["f48556df-99e0-447b-b11f-9c7da4fb3258"]; got != 7 {
		t.Fatalf("first session's request count = %d, want 7", got)
	}
	if got := bySessionCount["9fec09c7-6e4f-4834-97e6-314bee5820c0"]; got != 3 {
		t.Fatalf("second session's request count = %d, want 3", got)
	}
}

type eventByRequest struct {
	input, output int64
	model         string
}

// TestTail_IncrementalMatchesFullRead appends the fixture to a temp file in
// two halves, plus a partial line, and checks Tail's two polls together
// name the same requests, with the same sessions, as one PollOnce over the
// whole fixture: the session id must carry across the poll boundary.
func TestTail_IncrementalMatchesFullRead(t *testing.T) {
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	want, err := PollOnce(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	wantSession := map[string]string{}
	for _, e := range want {
		wantSession[e.RequestID] = e.SessionID
	}

	lines := bytes.SplitAfter(raw, []byte("\n"))
	half := len(lines) / 2
	first := bytes.Join(lines[:half], nil)
	rest := bytes.Join(lines[half:], nil)

	path := filepath.Join(t.TempDir(), "copilot-otel.jsonl")
	if err := os.WriteFile(path, first, 0o644); err != nil {
		t.Fatal(err)
	}
	var tail Tail
	got := map[string]string{}
	poll := func() int {
		events, err := tail.Poll(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range events {
			got[e.RequestID] = e.SessionID
		}
		return len(events)
	}
	poll()
	cut := len(rest) - 10 // leave the last line partial for one poll
	appendBytes(t, path, rest[:cut])
	poll()
	appendBytes(t, path, rest[cut:])
	poll()
	if n := poll(); n != 0 {
		t.Fatalf("idle poll returned %d events, want 0", n)
	}
	if len(got) != len(wantSession) {
		t.Fatalf("tail saw %d requests, full read %d", len(got), len(wantSession))
	}
	for id, sid := range wantSession {
		if got[id] != sid {
			t.Fatalf("request %s: session %q via tail, %q via full read", id, got[id], sid)
		}
	}
}

// TestTail_ResetsWhenFileShrinksOrIsReplaced covers the two restart cases:
// a file truncated below the offset, and a new file at the same path.
func TestTail_ResetsWhenFileShrinksOrIsReplaced(t *testing.T) {
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "copilot-otel.jsonl")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	var tail Tail
	full, err := tail.Poll(path)
	if err != nil || len(full) != 10 {
		t.Fatalf("first poll: %d events, err %v; want 10", len(full), err)
	}

	// Shrink: rewrite with the first half only, smaller than the offset.
	lines := bytes.SplitAfter(raw, []byte("\n"))
	firstHalf := bytes.Join(lines[:len(lines)/2], nil)
	if err := os.WriteFile(path, firstHalf, 0o644); err != nil {
		t.Fatal(err)
	}
	shrunk, err := tail.Poll(path)
	if err != nil || len(shrunk) == 0 {
		t.Fatalf("after shrink: %d events, err %v; want a re-read from byte 0", len(shrunk), err)
	}

	// Replace: a different file, same path, larger than the old offset.
	other := filepath.Join(dir, "other.jsonl")
	if err := os.WriteFile(other, append(append([]byte{}, raw...), raw...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(other, path); err != nil {
		t.Fatal(err)
	}
	replaced, err := tail.Poll(path)
	if err != nil || len(replaced) != 10 {
		t.Fatalf("after replace: %d events, err %v; want 10 from a re-read from byte 0", len(replaced), err)
	}
}

func appendBytes(t *testing.T, path string, b []byte) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(b); err != nil {
		t.Fatal(err)
	}
}
