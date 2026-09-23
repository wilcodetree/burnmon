package scan

import "strings"

// LongSessionCalls marks a session whose per-call cost has clearly entered
// the marathon regime.
const LongSessionCalls = 100

// SurfaceLabel maps a surface key to its display name. Generic vocabulary
// as of v0.1 Step 1: cowork and chat (both Claude Desktop) merged into
// "desktop"; code_agent (worktree/sprint agent runs) kept as its own
// bucket since it is a run-type distinction, not a runtime surface. This
// is an intentional, acknowledged diff from v0.0.1's cowork/code/
// code_agent/chat vocabulary; see the Step 1 plan's "Decisions locked in".
var SurfaceLabel = map[string]string{
	"cli":        "Claude Code",
	"code_agent": "Claude Code, agent run",
	"desktop":    "Claude Desktop",
	"unknown":    "Unknown",
}

// PerModel is one aggregation cell: calls, tokens and both cost models.
// Also used for per-day cells inside a session.
type PerModel struct {
	Calls   int64   `json:"calls"`
	Tokens  int64   `json:"tokens"`
	Cost    float64 `json:"cost"`
	CostSub float64 `json:"cost_sub"`
}

// Session is one parsed transcript. JSON tags match schema 1 of
// claude_usage_extract.py; CWD and Daily are internal only.
type Session struct {
	SessionID      string               `json:"session_id"`
	Title          string               `json:"title"`
	// Owner is P6's light client map result, "" when no owner rules are
	// configured (one owner, no owner column shown anywhere).
	Owner          string               `json:"owner,omitempty"`
	// Client is K1's client map result, "unassigned" when Owners is
	// configured but nothing matched, "" when the store predates K1.
	Client         string               `json:"client,omitempty"`
	Surface        string               `json:"surface"`
	CWD            string               `json:"-"`
	Start          string               `json:"start"`
	End            string               `json:"end"`
	Calls          int64                `json:"calls"`
	Tokens         int64                `json:"tokens"`
	Cost           float64              `json:"cost"`
	CostSub        float64              `json:"cost_sub"`
	CostPerCall    float64              `json:"cost_per_call"`
	CostSubPerCall float64              `json:"cost_sub_per_call"`
	Fresh          int64                `json:"fresh"`
	CacheW         int64                `json:"cache_w"`
	CacheR         int64                `json:"cache_r"`
	Out            int64                `json:"out"`
	Models         map[string]*PerModel `json:"models"`
	Tools          map[string]int64     `json:"tools,omitempty"`
	Daily          map[string]*PerModel `json:"-"`
	Long           bool                 `json:"long"`

	// Unpriced is the token count of calls whose model had no price-book
	// entry (v0.1 Step 2: an OpenAI model id burnmon has not seen priced
	// yet, e.g. "codex-auto-review"). Zero cost, not zero usage; surfaced
	// separately so the dashboard does not silently under-report.
	Unpriced int64 `json:"unpriced_tokens,omitempty"`

	// ActiveMinutes is K2's active time: the sum of gaps between
	// consecutive events, any gap strictly above the configured
	// active_idle_minutes counting 0. "active time, from transcript
	// timestamps, not billable" (spec label, surfaced by the UI in V3-3).
	ActiveMinutes float64 `json:"active_minutes"`
}

// ToolGroup maps a raw tool name to the thing a human recognises: the
// connector it belongs to, the plugin-qualified skill it invoked, or
// "(built in)" for Claude's own tools. See the historical note this
// function carried in parse.go before the v0.1 Step 1 move: mcp__<server>
// groups as <server>; a pre-tagged "skill:<name>" groups as <name>
// unchanged; anything else groups as "(built in)".
func ToolGroup(name string) string {
	const mcpPrefix = "mcp__"
	const skillPrefix = "skill:"
	switch {
	case strings.HasPrefix(name, mcpPrefix):
		rest := name[len(mcpPrefix):]
		if idx := strings.Index(rest, "__"); idx >= 0 {
			return rest[:idx]
		}
		return rest
	case strings.HasPrefix(name, skillPrefix):
		return name[len(skillPrefix):]
	default:
		return "(built in)"
	}
}
