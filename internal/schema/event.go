// internal\schema\event.go

// Package schema defines the vendor-agnostic Event, the one shape every
// adapter (Claude, Codex, ...) parses its own transcripts into before they
// land in the store.
package schema

import "time"

// Event is one API call turn from any supported vendor. RequestID together
// with Vendor is the store's dedup key: "largest output wins" on upsert.
type Event struct {
	Vendor    string // "anthropic", "openai", "github", "nous"
	Agent     string // "claude-code", "cowork", "codex", "copilot-cli", "hermes"
	Surface   string // "cli", "vscode", "desktop", "code_agent", "unknown"
	SessionID string
	RequestID string // dedup key together with Vendor; Codex: session id + turn index
	ParentID  string // Claude subagent's parent session id, else ""
	At        time.Time // UTC
	Model     string
	Project   string // cwd or repo path as the trail gives it, "" if unknown

	// Title is not in the v0.1 spec's Event type. It is the session's
	// cleaned first-user-message text, carried per event so the store alone
	// (not a file rescan) can reconstruct a session's display title:
	// session reconstruction takes the first non-empty Title seen for a
	// SessionID. See the Step 1 plan's "Decisions locked in" section.
	Title string

	Input       int64  // fresh input
	CacheWrite  *int64 // nil when the vendor has no such class
	CacheRead   *int64
	Output      int64
	Reasoning   *int64
	VendorCost  *float64 // Hermes records real cost; nil otherwise
	WindowUsed  *float64 // Codex rate_limits used_percent (5h) when present
	WindowReset *time.Time
	Tools       map[string]int64 // tool calls in this turn, Claude only for now
}
