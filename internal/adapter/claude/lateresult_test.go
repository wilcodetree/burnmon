package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseLateToolResult: a tool_result that lands in a later incremental
// read than its tool_use (a 20 s Bash on a live session) used to be dropped,
// so the call never got its result (1,812 of 3,805 Bash rows in Wilco's
// store had none). The later read now returns a result-only row for it.
func TestParseLateToolResult(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "toolcalls.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	cut := strings.Index(string(raw), `"tool_result"`)
	cut = strings.LastIndex(string(raw[:cut]), "\n") + 1
	path := filepath.Join(t.TempDir(), "live.jsonl")
	if err := os.WriteFile(path, raw[:cut], 0o644); err != nil {
		t.Fatal(err)
	}
	a := Adapter{}
	_, first, off, err := a.Parse(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].CallID != "toolu_1" || first[0].ResultBytes != nil {
		t.Fatalf("first read: %+v, want toolu_1 with no result yet", first)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	_, second, _, err := a.Parse(path, off)
	if err != nil {
		t.Fatal(err)
	}
	var late bool
	for _, c := range second {
		if c.CallID == "toolu_1" {
			late = true
			if c.Tool != "" || c.ResultBytes == nil || *c.ResultBytes != int64(len("package a\n")) {
				t.Fatalf("late row %+v, want result-only (no tool) with 10 result bytes", c)
			}
		}
	}
	if !late {
		t.Fatalf("second read %+v carries no result for toolu_1", second)
	}
}
