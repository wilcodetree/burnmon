// Package codex parses the JSONL rollout transcripts the Codex CLI, Desktop
// and VS Code extension write, into schema.Events.
//
// Source, confirmed on a live file (see SESSION_LOG.md, v0.1 Step 2):
// %USERPROFILE%\.codex\sessions\YYYY\MM\DD\rollout-<ts>-<uuid>.jsonl, or
// $CODEX_HOME\sessions when set. A per-turn token_count line is a top-level
// {"type":"event_msg","payload":{"type":"token_count",...}}; a turn_context
// line (the model) is top-level {"type":"turn_context","payload":{...}},
// with no event_msg wrapper. rate_limits sits beside info, not inside it:
// payload.rate_limits.primary / .secondary.
package codex

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"burnmon/internal/scan"
	"burnmon/internal/schema"
)

// Adapter implements adapter.Adapter for Codex rollout transcripts.
type Adapter struct{}

func (Adapter) Name() string { return "codex" }

// wslDeadline mirrors scan's own bound on WSL distro probing.
const wslDeadline = 5 * time.Second

// NativeSources returns the native (non-WSL) Codex sessions folder for this
// machine: $CODEX_HOME\sessions when set, else %USERPROFILE%\.codex\sessions.
func NativeSources() []string {
	var out []string
	add := func(p string) {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			out = append(out, p)
		}
	}
	if ch := os.Getenv("CODEX_HOME"); ch != "" {
		add(filepath.Join(ch, "sessions"))
	} else if home, err := os.UserHomeDir(); err == nil && home != "" {
		add(filepath.Join(home, ".codex", "sessions"))
	}
	return out
}

// DefaultSourcesWithOptions is NativeSources with WSL distro detection
// optional, mirroring scan.DefaultSourcesWithOptions so dataset.Collect can
// apply the same "two cadences" gating to Codex that it already applies to
// Claude.
func DefaultSourcesWithOptions(scanWSL bool) []string {
	out := NativeSources()
	if scanWSL {
		out = append(out, scan.WSLHomeSources(wslDeadline, ".codex/sessions", "CODEX_HOME")...)
	}
	logZstSiblings(out)
	return out
}

// Roots implements adapter.Adapter.
func (Adapter) Roots() []string { return DefaultSourcesWithOptions(true) }

// logZstSiblings logs (does not read) a rollout-*.jsonl.zst compressed
// sibling next to an uncompressed rollout, per the spec: "logged and
// skipped in v0.1" (openai/codex issue #24948 notes newer builds may write
// one alongside a state_5.sqlite index once a rollout grows large). It is
// already naturally skipped: scan.FindJSONL only picks up files ending in
// literal ".jsonl", which ".jsonl.zst" does not match.
func logZstSiblings(roots []string) {
	for _, root := range roots {
		matches, _ := filepath.Glob(filepath.Join(root, "*", "*", "*", "*.jsonl.zst"))
		for _, m := range matches {
			log.Printf("codex: skipping compressed rollout sibling %s (v0.1 does not read .jsonl.zst)", m)
		}
	}
}

func asInt(v any) int64 {
	if f, ok := v.(float64); ok {
		return int64(f)
	}
	return 0
}

func asFloat(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

// classifySurface maps a session_meta originator to the report's surface
// vocabulary. Values actually seen on Wilco's laptop (SESSION_LOG.md, v0.1
// Step 2, and the v0.1.1 originator values "codex-tui" / "codex_vscode"
// confirmed live on 2026-09-22): "codex-tui", "codex_vscode", "Codex
// Desktop", "codex_work_desktop"; the spec's guessed "codex_cli_rs" also
// matches the "cli" case by substring. Logs (once per distinct value) any
// originator this substring match does not recognise, per F2.
func classifySurface(originator string) string {
	o := strings.ToLower(originator)
	switch {
	case strings.Contains(o, "vscode"):
		return "vscode"
	case strings.Contains(o, "desktop"):
		return "desktop"
	case strings.Contains(o, "tui") || strings.Contains(o, "cli"):
		return "cli"
	default:
		logUnknownOriginatorOnce(originator)
		return "unknown"
	}
}

var (
	unknownOriginatorsMu sync.Mutex
	unknownOriginators   = map[string]bool{}
)

// logUnknownOriginatorOnce logs originator the first time classifySurface
// fails to recognise it, and stays silent on every later occurrence of the
// same value (a live-watched session re-parses its own session_meta line
// via scanHeaderMeta on every incremental read, so without dedup this would
// otherwise log once per poll for the life of the session).
func logUnknownOriginatorOnce(originator string) {
	unknownOriginatorsMu.Lock()
	defer unknownOriginatorsMu.Unlock()
	if unknownOriginators[originator] {
		return
	}
	unknownOriginators[originator] = true
	log.Printf("codex: unrecognized session_meta originator %q, surface set to unknown", originator)
}

// rateLimitWindow picks the rate_limits window closest to Codex's 5-hour
// quota: real data shows "primary" is not reliably the 5h window (an older
// CLI build had primary as the 10080-minute/weekly window with secondary
// nil; a newer one has primary at 300 minutes/5h and secondary at 10080).
// This prefers whichever of primary/secondary has window_minutes <= 360,
// falling back to primary so a window is still surfaced even when neither
// looks like a 5h window.
func rateLimitWindow(rl map[string]any) map[string]any {
	primary, _ := rl["primary"].(map[string]any)
	secondary, _ := rl["secondary"].(map[string]any)
	looks5h := func(w map[string]any) bool {
		if w == nil {
			return false
		}
		wm, ok := asFloat(w["window_minutes"])
		return ok && wm > 0 && wm <= 360
	}
	switch {
	case looks5h(primary):
		return primary
	case looks5h(secondary):
		return secondary
	case primary != nil:
		return primary
	default:
		return secondary
	}
}

// headerScanLimit bounds scanHeaderMeta's re-read of a rollout's own start:
// session_meta is always the first line in every rollout observed on
// Wilco's laptop, so this is normally one line, but the cap keeps a resumed
// read on a multi-gigabyte rollout cheap even if a build ever moves it.
const headerScanLimit = 64 * 1024

// scanHeaderMeta re-reads path's own start (bounded by headerScanLimit) so
// an incremental Parse call that starts at a later offset can still recover
// state that only appears near the top of the file and is never repeated:
// cwd, surface and whether the thread is a sub-run from session_meta
// (always the first line), and, N6 (v0.2.2, SESSION_LOG.md), the model from
// turn_context. Without the session_meta half of this, a live-watched Codex
// session's every turn after the first read reported surface "unknown" (F2,
// SESSION_LOG.md); without the turn_context half, the Now page showed
// "(model unknown)" for exactly the same reason, confirmed on a real
// rollout (68-line file, model=gpt-5.6-sol at turn_context on line 7 then
// not repeated again until line 150): most incremental reads landed in the
// gap between two turn_context lines and had none of their own. A session's
// model does change mid-file on rare occasions (the fixture this guards
// exercises that case), so this keeps scanning the whole header rather than
// stopping at the first turn_context, and returns the last one seen, same
// as the live read loop below would if it saw every line from the start.
// Leaves f positioned wherever the bounded read stopped; the caller seeks
// to its own offset afterward.
func scanHeaderMeta(f *os.File) (cwd, surface, model string, subRun bool) {
	surface = "unknown"
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return cwd, surface, model, subRun
	}
	reader := bufio.NewReader(io.LimitReader(f, headerScanLimit))
	for {
		rawLine, readErr := reader.ReadString('\n')
		line := strings.TrimSpace(rawLine)
		if line != "" {
			var obj map[string]any
			if json.Unmarshal([]byte(line), &obj) == nil {
				payload, _ := obj["payload"].(map[string]any)
				switch typ, _ := obj["type"].(string); typ {
				case "session_meta":
					if payload != nil {
						cwd, surface, subRun = readSessionMeta(payload)
					}
				case "turn_context":
					if payload != nil && !subRun {
						if m, ok := payload["model"].(string); ok && m != "" {
							model = m
						}
					}
				}
			}
		}
		if readErr != nil {
			break
		}
	}
	return cwd, surface, model, subRun
}

// readSessionMeta extracts cwd, surface and whether payload describes a
// sub-run thread (a reviewer or other internal sub-agent, not the user's
// own conversation) from one session_meta line's payload. Confirmed on a
// live rollout (SESSION_LOG.md, v0.1.1 F2): a normal session's session_meta
// carries "thread_source":"user"; a Codex-internal sub-run (observed:
// "codex-auto-review", thread_source "guardian_review", source.subagent
// set) carries a non-"user" thread_source and its turn_context.model is not
// a real model id, it is the sub-run's own name.
func readSessionMeta(payload map[string]any) (cwd, surface string, subRun bool) {
	surface = "unknown"
	if c, ok := payload["cwd"].(string); ok {
		cwd = c
	}
	if o, ok := payload["originator"].(string); ok {
		surface = classifySurface(o)
	}
	if ts, ok := payload["thread_source"].(string); ok && ts != "" && ts != "user" {
		subRun = true
	}
	return cwd, surface, subRun
}

// pendingToolCall is one function_call/custom_tool_call payload awaiting its
// matching *_output payload, tracked by call_id (the field both payload types
// share with their output).
type pendingToolCall struct {
	turn        string
	tool        string
	at          string
	inputBytes  int64
	resultBytes *int64
	path        string
}

// filePathFromArgs is I3's best-effort file path extraction for a Codex
// function_call/custom_tool_call's own raw input: both arrive as a raw
// string (function_call's "arguments" is JSON, custom_tool_call's "input"
// usually is not, e.g. a shell command), so this only ever returns a path
// when raw happens to decode as a JSON object carrying "file_path" or
// "path". Unverified against a real custom_tool_call fixture (SESSION_LOG.md
// carries the caveat); "" is the safe, common case (a shell/exec call named
// no single file at all).
func filePathFromArgs(raw string) string {
	var obj map[string]any
	if json.Unmarshal([]byte(raw), &obj) != nil {
		return ""
	}
	if p, ok := obj["file_path"].(string); ok && p != "" {
		return p
	}
	if p, ok := obj["path"].(string); ok && p != "" {
		return p
	}
	return ""
}

// toolCallOutputBytes approximates a *_output payload's byte size: a plain
// output string (function_call_output) is its own length; a block list
// (custom_tool_call_output) sums its "text" fields' lengths.
func toolCallOutputBytes(output any) int64 {
	switch v := output.(type) {
	case string:
		return int64(len(v))
	case []any:
		var n int64
		for _, item := range v {
			if b, ok := item.(map[string]any); ok {
				if t, ok := b["text"].(string); ok {
					n += int64(len(t))
				}
			}
		}
		return n
	}
	return 0
}

// Parse reads path from byte offset from to EOF and returns one Event per
// token_count line found, one ToolCall per function_call/custom_tool_call
// payload (S2: both families, not just "function_call" as the spec's prose
// names, since custom_tool_call is where real Codex shell/exec activity
// lives, see SESSION_LOG.md), plus the byte offset right after the last
// complete line consumed. A trailing partial line (the file still being
// written) is left for the next call, same contract as the claude adapter.
//
// Model is the most recent turn_context.model seen since from, seeded (N6,
// v0.2.2, SESSION_LOG.md) by scanHeaderMeta's own header-window scan for
// from > 0, then overwritten by any fresher turn_context this read's own
// chunk carries: a session that switches model mid-file (rare, but real)
// is still picked up correctly, and a token_count line appearing in a
// chunk with no turn_context of its own (the common case, confirmed on a
// real rollout, see scanHeaderMeta's own comment) now falls back to the
// header-scanned value instead of "". A sub-run thread (see
// readSessionMeta) never gets a Model at all: its turn_context.model is a
// sub-run name, not a model id (observed: "codex-auto-review"), so it is
// carried as Title instead and Model stays empty, "context window unknown"
// rather than a fabricated one.
func (Adapter) Parse(path string, from int64) ([]schema.Event, []schema.ToolCall, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, from, err
	}
	defer f.Close()

	base := filepath.Base(path)
	sessionID := strings.TrimSuffix(base, filepath.Ext(base))

	cwd := ""
	model := ""
	surface := "unknown"
	title := ""
	subRun := false

	if from > 0 {
		cwd, surface, model, subRun = scanHeaderMeta(f)
	}
	if _, err := f.Seek(from, io.SeekStart); err != nil {
		return nil, nil, from, err
	}

	var events []schema.Event
	pendingCalls := map[string]*pendingToolCall{}
	var toolCallOrder []string
	offset := from
	reader := bufio.NewReader(f)
	for {
		rawLine, readErr := reader.ReadString('\n')
		if readErr != nil {
			if readErr == io.EOF {
				break // trailing partial line, or nothing left; leave unconsumed
			}
			return nil, nil, from, readErr
		}
		lineLen := int64(len(rawLine))
		line := strings.TrimSpace(rawLine)
		if line == "" {
			offset += lineLen
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			offset += lineLen
			continue
		}
		offset += lineLen

		typ, _ := obj["type"].(string)
		payload, _ := obj["payload"].(map[string]any)

		switch typ {
		case "session_meta":
			if payload != nil {
				c, s, sr := readSessionMeta(payload)
				if cwd == "" {
					cwd = c
				}
				surface = s
				subRun = sr
			}
			continue
		case "turn_context":
			if payload != nil {
				if c, ok := payload["cwd"].(string); ok && cwd == "" {
					cwd = c
				}
				if m, ok := payload["model"].(string); ok && m != "" {
					if subRun {
						// A sub-run thread's turn_context.model is not a real
						// model id, it is the sub-run's own name (observed:
						// "codex-auto-review"); show it as the session title
						// instead of a fabricated model.
						title = m
					} else {
						model = m
					}
				}
			}
			continue
		}

		// S2's tool_calls: these arrive as top-level "response_item" lines,
		// their own family carried in payload["type"] rather than in typ
		// (unlike session_meta/turn_context/token_count above), paired by
		// call_id with their *_output line. Two distinct payload families
		// carry real tool calls (SESSION_LOG.md): "function_call" (rare, e.g.
		// request_user_input_async) and "custom_tool_call" (the actual
		// shell/exec activity Codex sessions mostly consist of).
		if typ == "response_item" && payload != nil {
			switch pt, _ := payload["type"].(string); pt {
			case "function_call", "custom_tool_call":
				name, _ := payload["name"].(string)
				var raw string
				if pt == "function_call" {
					raw, _ = payload["arguments"].(string)
				} else {
					raw, _ = payload["input"].(string)
				}
				callID, _ := payload["call_id"].(string)
				if callID != "" && name != "" {
					turn := ""
					if meta, ok := payload["internal_chat_message_metadata_passthrough"].(map[string]any); ok {
						turn, _ = meta["turn_id"].(string)
					}
					ts, _ := obj["timestamp"].(string)
					pendingCalls[callID] = &pendingToolCall{turn: turn, tool: name, at: ts, inputBytes: int64(len(raw)), path: filePathFromArgs(raw)}
					toolCallOrder = append(toolCallOrder, callID)
				}
				continue
			case "function_call_output", "custom_tool_call_output":
				callID, _ := payload["call_id"].(string)
				if pc, ok := pendingCalls[callID]; ok {
					n := toolCallOutputBytes(payload["output"])
					pc.resultBytes = &n
				}
				continue
			}
		}

		if typ != "event_msg" || payload == nil || payload["type"] != "token_count" {
			continue
		}
		info, _ := payload["info"].(map[string]any)
		if info == nil {
			continue
		}
		last, _ := info["last_token_usage"].(map[string]any)
		if last == nil {
			// Spec fallback: derive the turn as a delta of total_token_usage.
			// Every rollout observed on Wilco's laptop carried
			// last_token_usage on every token_count line, so this path is
			// untested against a live file; kept as the documented,
			// spec-required fallback rather than a guess at its shape.
			continue
		}

		ordinal, _ := asFloat(obj["ordinal"])
		e := schema.Event{
			Vendor:    "openai",
			Agent:     "codex",
			Surface:   surface,
			SessionID: sessionID,
			RequestID: sessionID + ":" + strconv.FormatInt(int64(ordinal), 10),
			Project:   cwd,
			Model:     model,
			Title:     title,
		}

		inputTokens := asInt(last["input_tokens"])
		cachedInput := asInt(last["cached_input_tokens"])
		e.Input = inputTokens - cachedInput
		if cachedInput > 0 {
			v := cachedInput
			e.CacheRead = &v
		}
		e.Output = asInt(last["output_tokens"])
		if r := asInt(last["reasoning_output_tokens"]); r > 0 {
			e.Reasoning = &r
		}

		if ts, ok := obj["timestamp"].(string); ok && ts != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				e.At = parsed.UTC()
			}
		}

		if rl, ok := payload["rate_limits"].(map[string]any); ok && rl != nil {
			if w := rateLimitWindow(rl); w != nil {
				if up, ok := asFloat(w["used_percent"]); ok {
					e.WindowUsed = &up
				}
				if ra, ok := asFloat(w["resets_at"]); ok {
					t := time.Unix(int64(ra), 0).UTC()
					e.WindowReset = &t
				}
			}
		}

		events = append(events, e)
	}

	var toolCalls []schema.ToolCall
	for _, callID := range toolCallOrder {
		pc := pendingCalls[callID]
		var at time.Time
		if pc.at != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, pc.at); err == nil {
				at = parsed.UTC()
			}
		}
		toolCalls = append(toolCalls, schema.ToolCall{
			Vendor: "openai", Agent: "codex", SessionID: sessionID,
			CallID: callID, Turn: pc.turn, Tool: pc.tool, At: at,
			InputBytes: pc.inputBytes, ResultBytes: pc.resultBytes, Path: pc.path,
		})
	}

	return events, toolCalls, offset, nil
}
