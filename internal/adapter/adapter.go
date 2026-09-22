// Package adapter defines the boundary every vendor's transcript parser
// implements: turn a trail file into schema.Events, incrementally.
package adapter

import "burnmon/internal/schema"

// Adapter parses one vendor's session transcripts into Events.
type Adapter interface {
	// Name is the vendor's short id: "claude", "codex", ...
	Name() string

	// Roots lists the folders this adapter's transcripts live under, from
	// the existing discovery logic (internal/scan for Claude).
	Roots() []string

	// Parse reads path starting at byte offset from and returns every
	// complete Event and ToolCall found from there to EOF, plus the new
	// offset to pass next time (the byte position right after the last
	// complete line consumed; a trailing partial line is left unread for the
	// next call). A tool call whose result lands after the read's end is
	// returned with ResultBytes nil (see schema.ToolCall).
	Parse(path string, from int64) (events []schema.Event, toolCalls []schema.ToolCall, newOffset int64, err error)
}
