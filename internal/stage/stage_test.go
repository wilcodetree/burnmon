package stage

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func TestClassify(t *testing.T) {
	sec := time.Second
	base := Input{Now: t0, Start: t0.Add(-time.Hour), LastTurn: t0.Add(-5 * sec)}
	cases := []struct {
		name string
		mod  func(in *Input)
		want Stage
	}{
		{"new session arrives", func(in *Input) { in.Start = t0.Add(-5 * sec) }, Arriving},
		{"compaction just fired", func(in *Input) { in.CompactedAt = t0.Add(-20 * sec) }, Compacting},
		{"old compaction ignored, answered turn waits", func(in *Input) { in.CompactedAt = t0.Add(-5 * time.Minute) }, Waiting},
		{"silent past cutoff is dormant", func(in *Input) { in.LastTurn = t0.Add(-11 * time.Minute) }, Dormant},
		{"open Read is reading", func(in *Input) { in.LastTool, in.LastToolAt, in.LastToolInLatestTurn = "Read", t0.Add(-2*sec), true }, Reading},
		{"done Read, short gap stays reading", func(in *Input) {
			in.LastTool, in.LastToolAt, in.LastToolInLatestTurn, in.LastToolDone = "Read", t0.Add(-3*sec), true, true
			in.LastTurn = t0.Add(-4 * sec)
		}, Reading},
		{"done Edit, long gap is thinking", func(in *Input) {
			in.LastTool, in.LastToolAt, in.LastToolInLatestTurn, in.LastToolDone = "Edit", t0.Add(-20*sec), true, true
			in.LastTurn = t0.Add(-21 * sec)
		}, Thinking},
		{"open WebFetch fetches", func(in *Input) { in.LastTool, in.LastToolAt, in.LastToolInLatestTurn = "WebFetch", t0.Add(-sec), true }, Fetching},
		{"mcp tool fetches", func(in *Input) { in.LastTool, in.LastToolAt, in.LastToolInLatestTurn = "mcp__slack__read", t0.Add(-sec), true }, Fetching},
		{"long open Bash keeps running", func(in *Input) {
			in.LastTool, in.LastToolAt, in.LastToolInLatestTurn = "Bash", t0.Add(-5*time.Minute), true
			in.LastTurn = t0.Add(-5 * time.Minute)
		}, Running},
		{"long open Edit is a permission prompt", func(in *Input) {
			in.LastTool, in.LastToolAt, in.LastToolInLatestTurn = "Edit", t0.Add(-4*time.Minute), true
			in.LastTurn = t0.Add(-4 * time.Minute)
		}, Waiting},
		{"open Task delegates", func(in *Input) {
			in.LastTool, in.LastToolAt, in.LastToolInLatestTurn = "Task", t0.Add(-6*time.Minute), true
			in.LastTurn = t0.Add(-6 * time.Minute)
		}, Delegating},
		{"AskUserQuestion waits", func(in *Input) { in.LastTool, in.LastToolAt, in.LastToolInLatestTurn = "AskUserQuestion", t0.Add(-sec), true }, Waiting},
		{"tool from an older turn is past", func(in *Input) { in.LastTool, in.LastToolAt = "Edit", t0.Add(-30*sec) }, Waiting},
		{"codex apply_patch codes", func(in *Input) { in.LastTool, in.LastToolAt, in.LastToolInLatestTurn = "apply_patch", t0.Add(-sec), true }, Coding},
		{"codex shell runs", func(in *Input) { in.LastTool, in.LastToolAt, in.LastToolInLatestTurn = "shell", t0.Add(-sec), true }, Running},
		{"TodoWrite plans", func(in *Input) { in.LastTool, in.LastToolAt, in.LastToolInLatestTurn = "TodoWrite", t0.Add(-sec), true }, Planning},
	}
	for _, c := range cases {
		in := base
		c.mod(&in)
		if got := Classify(in).Stage; got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestDormantSinceIsCutoffMoment(t *testing.T) {
	in := Input{Now: t0, Start: t0.Add(-time.Hour), LastTurn: t0.Add(-15 * time.Minute)}
	r := Classify(in)
	if want := t0.Add(-5 * time.Minute); !r.Since.Equal(want) {
		t.Fatalf("Since = %v, want %v", r.Since, want)
	}
}

func TestUnknownToolIsCoding(t *testing.T) {
	if s := ToolStage("SomeNewTool"); s != Coding {
		t.Fatalf("got %s", s)
	}
}

// TestCodexExecRuns: the real store holds Codex shell calls under the name
// "exec" (93 rows since 2026-09-25 in Wilco's burnmon.db, no "shell" or
// "exec_command"), which fell through to the Coding default.
func TestCodexExecRuns(t *testing.T) {
	if s := ToolStage("exec"); s != Running {
		t.Fatalf("got %s", s)
	}
}

// TestNoToolTrail: Copilot in VS Code records one OTel row per request and no
// tool calls, so a session mid-work has no tool to anchor a stage on. Measured
// on the real store (2026-10-05, 513 gaps between consecutive requests): the
// median gap is 9 s, 93 percent are within 3 minutes. Until that long a
// silence is the agent between requests (thinking), not a hand-back.
func TestNoToolTrail(t *testing.T) {
	sec := time.Second
	base := Input{Now: t0, Start: t0.Add(-time.Hour), LastTurn: t0.Add(-5 * sec), NoToolTrail: true}
	cases := []struct {
		name string
		mod  func(in *Input)
		want Stage
	}{
		{"request just landed, agent is between requests", func(in *Input) {}, Thinking},
		{"one minute of silence is still between requests", func(in *Input) { in.LastTurn = t0.Add(-time.Minute) }, Thinking},
		{"past the stuck-tool window the human is up", func(in *Input) { in.LastTurn = t0.Add(-StuckToolAfter - sec) }, Waiting},
		{"silent past the cutoff is dormant", func(in *Input) { in.LastTurn = t0.Add(-11 * time.Minute) }, Dormant},
		{"new session still arrives", func(in *Input) { in.Start = t0.Add(-5 * sec) }, Arriving},
		{"compaction still wins", func(in *Input) { in.CompactedAt = t0.Add(-20 * sec) }, Compacting},
		{"without the flag the same silence is waiting", func(in *Input) { in.NoToolTrail = false }, Waiting},
	}
	for _, c := range cases {
		in := base
		c.mod(&in)
		if got := Classify(in).Stage; got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
	r := Classify(base)
	if want := base.LastTurn; !r.Since.Equal(want) {
		t.Errorf("Since = %v, want the last request %v", r.Since, want)
	}
	waiting := base
	waiting.LastTurn = t0.Add(-StuckToolAfter - sec)
	if want := waiting.LastTurn.Add(StuckToolAfter); !Classify(waiting).Since.Equal(want) {
		t.Errorf("waiting Since = %v, want %v", Classify(waiting).Since, want)
	}
}
