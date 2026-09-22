package claude

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseDoesNotConsumeTrailingPartialLine guards against the incremental
// read regression where a trailing, not-yet-newline-terminated line (the
// file still being written by Claude Code) was treated as a complete line:
// its bytes were consumed and the offset advanced past it, so the rest of
// that line, appended moments later, could never be read again. Parse must
// leave such a line untouched, both in its returned events and in the
// offset it returns.
func TestParseDoesNotConsumeTrailingPartialLine(t *testing.T) {
	a := Adapter{}
	dir := t.TempDir()
	path := filepath.Join(dir, "live.jsonl")

	line1 := `{"type":"user","cwd":"C:\\proj\\demo","message":{"content":"hello"}}` + "\n"
	line2 := `{"type":"assistant","timestamp":"2026-09-10T10:00:00.000Z","requestId":"req-1","message":{"model":"claude-x","usage":{"input_tokens":10,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":5}}}` + "\n"
	// A trailing partial line: the writer has flushed this much but has not
	// yet written the closing brace or the newline.
	partial := `{"type":"assistant","timestamp":"2026-09-10T10:05:00.000Z","requestId":"req-2","message":{"model":"claude-x","usage":{"input_tokens":20`

	if err := os.WriteFile(path, []byte(line1+line2+partial), 0o644); err != nil {
		t.Fatal(err)
	}

	events, offset, err := a.Parse(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1 (only req-1; the partial line must not be parsed)", len(events))
	}
	if events[0].RequestID != "req-1" {
		t.Fatalf("event RequestID = %q, want req-1", events[0].RequestID)
	}
	wantOffset := int64(len(line1) + len(line2))
	if offset != wantOffset {
		t.Fatalf("offset = %d, want %d (start of the still-partial trailing line, not past it)", offset, wantOffset)
	}

	// Simulate the rest of that line being written: append the missing
	// closing content plus the newline that completes it.
	rest := `,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":15}}}` + "\n"
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(rest); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	events2, offset2, err := a.Parse(path, offset)
	if err != nil {
		t.Fatal(err)
	}
	if len(events2) != 1 {
		t.Fatalf("second pass: got %d events, want 1 (req-2, now complete)", len(events2))
	}
	if events2[0].RequestID != "req-2" {
		t.Fatalf("second pass: RequestID = %q, want req-2", events2[0].RequestID)
	}
	if events2[0].Input != 20 || events2[0].Output != 15 {
		t.Fatalf("second pass: req-2 tokens wrong: input=%d output=%d", events2[0].Input, events2[0].Output)
	}
	fi, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if offset2 != fi.Size() {
		t.Fatalf("offset2 = %d, want file size %d (fully consumed, no double-counting)", offset2, fi.Size())
	}
}
