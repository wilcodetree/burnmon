package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBasicFixture(t *testing.T) {
	a := Adapter{}
	path := filepath.Join("testdata", "basic.jsonl")

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
			if e.Tools["graphify"] != 1 {
				t.Fatalf("req-2 Tools[graphify] = %d, want 1 (skill: prefix stripped)", e.Tools["graphify"])
			}
		default:
			t.Fatalf("unexpected RequestID %q", e.RequestID)
		}
	}
	_ = req1
	_ = req2
}

func TestParseResumesFromOffset(t *testing.T) {
	a := Adapter{}
	path := filepath.Join("testdata", "basic.jsonl")

	first, off1, err := a.Parse(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("first pass: got %d events, want 2", len(first))
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
	if a.Name() != "claude" {
		t.Fatalf("Name() = %q, want claude", a.Name())
	}
	_ = a.Roots() // must not panic; a fresh CI box legitimately has zero folders
}
