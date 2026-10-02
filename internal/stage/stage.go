// Package stage classifies a live session into one Station stage (the
// BurnMon Dev secret screen, spec 02_roadmap/2026-10-01_station_secret_screen.md).
//
// The trails show tool calls and finished turns, never the model's own
// thought, so every stage here is inferred from three facts: which tool the
// latest turn called, whether that tool's result has landed, and how long
// the session has been silent. Thinking in particular is a silence gap
// after a tool result, not an observed state. The rules are ordered; the
// first one that matches wins.
package stage

import (
	"strings"
	"time"
)

// Stage is one Station room.
type Stage string

const (
	Arriving   Stage = "arriving"   // airlock: session started moments ago
	Reading    Stage = "reading"    // archive: Read, Grep, Glob and friends
	Fetching   Stage = "fetching"   // uplink: web and MCP tools
	Planning   Stage = "planning"   // plan table: todo and plan tools
	Coding     Stage = "coding"     // fabricator: Edit, Write, apply_patch
	Running    Stage = "running"    // test chamber: shell, tests, builds
	Thinking   Stage = "thinking"   // think pods: tool result in, next turn not yet
	Delegating Stage = "delegating" // briefing room: a subagent is out
	Waiting    Stage = "waiting"    // lounge: turn ended, the human is up
	Dormant    Stage = "dormant"    // cryo: silent past the idle cutoff
	Compacting Stage = "compacting" // recycler: context compaction just fired
)

// Tunables. IdleCutoff matches K2's active-time cutoff (10 minutes) so the
// Station and the active-time figure agree on when a session went quiet.
const (
	IdleCutoff     = 10 * time.Minute
	ArrivalWindow  = 15 * time.Second
	CompactWindow  = 60 * time.Second
	ThinkAfter     = 8 * time.Second // silence after a tool result before the agent counts as thinking
	StuckToolAfter = 3 * time.Minute // an open non-shell tool this long is most likely a permission prompt
)

// Input is everything Classify needs about one session. Zero times mean
// "never seen".
type Input struct {
	Now      time.Time
	Start    time.Time
	LastTurn time.Time // newest real API-call turn

	// LastTool is the newest tool call's name, "" when the session has none.
	// LastToolInLatestTurn is true when that call belongs to the newest
	// turn (its Turn key equals the newest turn's key), so the agent is
	// still busy with it rather than past it.
	LastTool             string
	LastToolAt           time.Time
	LastToolInLatestTurn bool
	// LastToolDone is true when the call's result has landed
	// (schema.ToolCall.ResultBytes non-nil).
	LastToolDone bool

	// CompactedAt is the newest compaction finding's turn time.
	CompactedAt time.Time
}

// Result is the classified stage plus the moment it began and the tool
// that put the session there ("" when no tool did).
type Result struct {
	Stage Stage     `json:"stage"`
	Since time.Time `json:"since"`
	Tool  string    `json:"tool,omitempty"`
}

// Classify applies the ordered rules.
func Classify(in Input) Result {
	last := in.LastTurn
	if in.LastToolAt.After(last) {
		last = in.LastToolAt
	}
	if last.IsZero() {
		last = in.Start
	}
	idle := in.Now.Sub(last)

	switch {
	case !in.Start.IsZero() && in.Now.Sub(in.Start) < ArrivalWindow:
		return Result{Stage: Arriving, Since: in.Start}
	case !in.CompactedAt.IsZero() && in.Now.Sub(in.CompactedAt) < CompactWindow:
		return Result{Stage: Compacting, Since: in.CompactedAt}
	case idle >= IdleCutoff:
		return Result{Stage: Dormant, Since: last.Add(IdleCutoff)}
	}

	if in.LastTool != "" && in.LastToolInLatestTurn {
		s := ToolStage(in.LastTool)
		if s == Waiting {
			return Result{Stage: Waiting, Since: in.LastToolAt, Tool: in.LastTool}
		}
		if in.LastToolDone {
			if idle >= StuckToolAfter {
				return Result{Stage: Waiting, Since: last.Add(StuckToolAfter), Tool: in.LastTool}
			}
			if idle >= ThinkAfter {
				return Result{Stage: Thinking, Since: last.Add(ThinkAfter), Tool: in.LastTool}
			}
			return Result{Stage: s, Since: in.LastToolAt, Tool: in.LastTool}
		}
		if idle >= StuckToolAfter && s != Running && s != Delegating {
			return Result{Stage: Waiting, Since: last.Add(StuckToolAfter), Tool: in.LastTool}
		}
		return Result{Stage: s, Since: in.LastToolAt, Tool: in.LastTool}
	}

	// The newest turn called no tool: the agent answered and handed back.
	return Result{Stage: Waiting, Since: last}
}

// ToolStage maps one tool name (Claude Code, Cowork, Codex, Copilot CLI,
// Hermes spellings) to its room. Unknown tools count as Coding: the
// fabricator is the general workbench.
func ToolStage(tool string) Stage {
	t := strings.ToLower(strings.TrimSpace(tool))
	if strings.HasPrefix(t, "mcp__") || strings.HasPrefix(t, "mcp.") {
		return Fetching
	}
	switch t {
	case "read", "grep", "glob", "ls", "notebookread", "view", "read_file",
		"list_dir", "list_directory", "search", "find", "file_search", "grep_search":
		return Reading
	case "webfetch", "websearch", "web_fetch", "web_search", "fetch", "toolsearch",
		"listmcpresourcestool", "readmcpresourcetool":
		return Fetching
	case "todowrite", "todoread", "taskcreate", "taskupdate", "tasklist", "taskget",
		"exitplanmode", "enterplanmode", "update_plan", "plan":
		return Planning
	case "edit", "write", "multiedit", "notebookedit", "apply_patch", "str_replace",
		"create_file", "edit_file", "write_file", "str_replace_editor":
		return Coding
	case "bash", "bashoutput", "killshell", "powershell", "shell", "exec", "exec_command",
		"local_shell", "run_command", "terminal", "write_stdin":
		return Running
	case "task", "agent", "sendmessage", "spawn_agent", "subagent":
		return Delegating
	case "askuserquestion", "request_user_input", "ask_user":
		return Waiting
	}
	return Coding
}
