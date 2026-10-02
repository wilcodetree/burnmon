package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseLateToolResult: the Codex twin of the Claude test. An exec's
// output landing in a later read (the 2 s tail poll on a held-open rollout)
// comes back as a result-only row instead of being dropped.
func TestParseLateToolResult(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "codex", "tool-calls.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	cut := strings.Index(string(raw), `_output"`)
	cut = strings.LastIndex(string(raw[:cut]), "\n") + 1
	path := filepath.Join(t.TempDir(), "rollout-late.jsonl")
	if err := os.WriteFile(path, raw[:cut], 0o644); err != nil {
		t.Fatal(err)
	}
	a := Adapter{}
	_, first, off, err := a.Parse(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 || first[0].ResultBytes != nil {
		t.Fatalf("first read: %+v, want a call with no result yet", first)
	}
	id := first[0].CallID
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	_, second, _, err := a.Parse(path, off)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range second {
		if c.CallID == id {
			if c.Tool != "" || c.ResultBytes == nil {
				t.Fatalf("late row %+v, want result-only (no tool) with result bytes", c)
			}
			return
		}
	}
	t.Fatalf("second read %+v carries no result for %s", second, id)
}
