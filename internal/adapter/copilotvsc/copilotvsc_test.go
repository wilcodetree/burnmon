package copilotvsc

import (
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
