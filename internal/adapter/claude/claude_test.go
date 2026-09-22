package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBasicFixture(t *testing.T) {
	a := Adapter{}
	path := filepath.Join("testdata", "basic.jsonl")

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
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (req-1 deduped to one, req-2 the other)", len(events))
	}

	var req1, req2 *struct{ output, fresh, cacheW, cacheR int64 }
	for _, e := range events {
		switch e.RequestID {
		case "req-1":
			if e.Output != 40 {
				t.Fatalf("req-1 Output = %d, want 40 (largest wins)", e.Output)
			}
			if e.Tools["Read"] != 1 {
				t.Fatalf("req-1 Tools[Read] = %d, want 1", e.Tools["Read"])
			}
			if e.Title != "/graphify build the graph" {
				t.Fatalf("req-1 Title = %q, want the cleaned first user message", e.Title)
			}
			if e.Surface != "cli" {
				t.Fatalf("req-1 Surface = %q, want cli", e.Surface)
			}
			if e.Agent != "claude-code" {
				t.Fatalf("req-1 Agent = %q, want claude-code (F4: cli surface)", e.Agent)
			}
			if e.Project != `C:\proj\demo` {
				t.Fatalf("req-1 Project = %q, want the cwd", e.Project)
			}
		case "req-2":
			if e.Output != 30 || e.Input != 50 {
				t.Fatalf("req-2 tokens wrong: output=%d input=%d", e.Output, e.Input)
			}
			if e.CacheWrite == nil || *e.CacheWrite != 20 {
				t.Fatalf("req-2 CacheWrite = %v, want 20", e.CacheWrite)
			}
			if e.CacheRead == nil || *e.CacheRead != 5 {
				t.Fatalf("req-2 CacheRead = %v, want 5", e.CacheRead)
			}
			if e.Tools["skill:graphify"] != 1 {
				t.Fatalf("req-2 Tools[skill:graphify] = %d, want 1 (prefix kept until report-aggregation time)", e.Tools["skill:graphify"])
			}
		default:
			t.Fatalf("unexpected RequestID %q", e.RequestID)
		}
	}
	_ = req1
	_ = req2
}

// TestAgentFor guards F4: every transcript previously got Agent:
// "claude-code" regardless of surface, so the Cowork card wrongly read
// "CLAUDE-CODE · DESKTOP" instead of "COWORK · DESKTOP". agentFor must map
// the desktop/cowork surfaces to "cowork" and leave every other surface
// (cli, code_agent) as "claude-code".
func TestAgentFor(t *testing.T) {
	cases := []struct {
		surface string
		want    string
	}{
		{"desktop", "cowork"},
		{"cowork", "cowork"},
		{"cli", "claude-code"},
		{"code_agent", "claude-code"},
		{"unknown", "claude-code"},
	}
	for _, c := range cases {
		if got := agentFor(c.surface); got != c.want {
			t.Errorf("agentFor(%q) = %q, want %q", c.surface, got, c.want)
		}
	}
}

// TestParseDesktopSurfaceGetsCoworkAgent guards F4 end to end through
// Parse: a trail file whose path lands under local-agent-mode-sessions (the
// Cowork/Desktop trail location, see classifySurface) must produce events
// with Agent "cowork", not "claude-code".
func TestParseDesktopSurfaceGetsCoworkAgent(t *testing.T) {
	dir := t.TempDir()
	desktopDir := filepath.Join(dir, "local-agent-mode-sessions")
	if err := os.MkdirAll(desktopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(desktopDir, "desktop-session.jsonl")
	line := `{"type":"assistant","cwd":"/home/dev/proj","message":{"model":"claude-fable-5-1","usage":{` +
		`"input_tokens":10,"output_tokens":5}},"timestamp":"2026-09-22T10:00:00Z","requestId":"r1"}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	a := Adapter{}
	events, _, _, err := a.Parse(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Surface != "desktop" {
		t.Fatalf("Surface = %q, want desktop", events[0].Surface)
	}
	if events[0].Agent != "cowork" {
		t.Fatalf("Agent = %q, want cowork (F4)", events[0].Agent)
	}
}

func TestParseResumesFromOffset(t *testing.T) {
	a := Adapter{}
	path := filepath.Join("testdata", "basic.jsonl")

	first, _, off1, err := a.Parse(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("first pass: got %d events, want 2", len(first))
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

// TestParseToolCalls guards S2's tool_calls extraction: two tool_use blocks,
// each with its own id and a matching tool_result on the next "user" line,
// must become two ToolCalls with input and result bytes filled in.
func TestParseToolCalls(t *testing.T) {
	a := Adapter{}
	path := filepath.Join("testdata", "toolcalls.jsonl")

	_, toolCalls, _, err := a.Parse(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(toolCalls) != 2 {
		t.Fatalf("got %d tool calls, want 2", len(toolCalls))
	}

	byID := map[string]struct {
		tool       string
		turn       string
		inputBytes int64
		resultLen  int64
		path       string
	}{
		"toolu_1": {"Read", "req-1", int64(len(`{"file_path":"a.go"}`)), int64(len("package a\n")), "a.go"},
		"toolu_2": {"Bash", "req-2", int64(len(`{"command":"go build ./..."}`)), int64(len("ok\n")), ""},
	}
	for _, tc := range toolCalls {
		want, ok := byID[tc.CallID]
		if !ok {
			t.Fatalf("unexpected CallID %q", tc.CallID)
		}
		if tc.Tool != want.tool {
			t.Fatalf("%s Tool = %q, want %q", tc.CallID, tc.Tool, want.tool)
		}
		if tc.Turn != want.turn {
			t.Fatalf("%s Turn = %q, want %q", tc.CallID, tc.Turn, want.turn)
		}
		if tc.Path != want.path {
			t.Fatalf("%s Path = %q, want %q (I3 file extraction)", tc.CallID, tc.Path, want.path)
		}
		if tc.Vendor != "anthropic" || tc.Agent != "claude-code" {
			t.Fatalf("%s Vendor/Agent = %q/%q, want anthropic/claude-code", tc.CallID, tc.Vendor, tc.Agent)
		}
		if tc.At.IsZero() {
			t.Fatalf("%s At is zero, want the tool_use line's timestamp", tc.CallID)
		}
		if tc.InputBytes != want.inputBytes {
			t.Fatalf("%s InputBytes = %d, want %d", tc.CallID, tc.InputBytes, want.inputBytes)
		}
		if tc.ResultBytes == nil || *tc.ResultBytes != want.resultLen {
			t.Fatalf("%s ResultBytes = %v, want %d", tc.CallID, tc.ResultBytes, want.resultLen)
		}
	}
}

func TestNameAndRoots(t *testing.T) {
	a := Adapter{}
	if a.Name() != "claude" {
		t.Fatalf("Name() = %q, want claude", a.Name())
	}
	_ = a.Roots() // must not panic; a fresh CI box legitimately has zero folders
}
