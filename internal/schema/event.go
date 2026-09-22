// Package schema defines the vendor-agnostic Event, the one shape every
// adapter (Claude, Codex, ...) parses its own transcripts into before they
// land in the store.
package schema

import "time"

// Event is one API call turn from any supported vendor. RequestID together
// with Vendor and SessionID is the store's dedup key: "largest output wins"
// on upsert.
type Event struct {
	Vendor    string // "anthropic", "openai", "github", "nous"
	Agent     string // "claude-code", "cowork", "codex", "copilot-cli", "hermes"
	Surface   string // "cli", "vscode", "desktop", "code_agent", "unknown"
	SessionID string
	RequestID string // dedup key together with Vendor and SessionID; Codex: session id + turn index
	ParentID  string // Claude subagent's parent session id, else ""
	At        time.Time // UTC
	Model     string
	Project   string // cwd or repo path as the trail gives it, "" if unknown

	// Owner is the light client map's result (pricing.Config.OwnerFor) for
	// this event's Project, applied at ingest. "" when the config carries no
	// owner rules (P6's default: one owner, no owner column shown anywhere).
	Owner string

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

// ToolCall is one tool invocation (a Claude tool_use block, a Codex
// function_call or custom_tool_call payload), stored alongside Events so S2's
// `tool_calls` table and the I3 "tool calls that turn" drawer don't need to
// re-parse a session's transcript. Fields carry json tags because the I3
// drawer (internal/live.BuildTurnDetail) returns ToolCalls straight through
// bmTurn to the Now page.
type ToolCall struct {
	Vendor    string `json:"vendor"`
	Agent     string `json:"agent"`
	SessionID string `json:"session_id"`
	CallID    string `json:"call_id"` // the vendor's own tool_use id / call_id; dedup key together with Vendor and SessionID
	Turn      string `json:"turn"`    // the turn this call belongs to: Claude's Event.RequestID key, Codex's rollout turn_id
	Tool      string `json:"tool"`
	At        time.Time `json:"at"`

	InputBytes int64 `json:"input_bytes"`
	// ResultBytes is nil until the matching tool_result/*_output is seen in
	// the same Parse read; a call and its result almost always land in the
	// same incremental read in practice (they are adjacent lines), so this
	// is not backfilled across separate reads.
	ResultBytes *int64 `json:"result_bytes,omitempty"`
	// Path is the file path the call touched, when the tool's own input
	// names one plainly (Claude's Read/Edit/Write/NotebookEdit: file_path or
	// notebook_path; Codex's function_call arguments, when they are a JSON
	// object carrying file_path or path). "" when the tool named no file
	// (a shell/exec call, a search) or the trail did not carry one in a
	// recognised shape: this is a best-effort extraction, not a guaranteed
	// one, per I3's "files read where the trail has them".
	Path string `json:"path,omitempty"`
}
