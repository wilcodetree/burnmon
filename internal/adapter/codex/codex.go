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
// Step 2): "codex-tui", "Codex Desktop", "codex_work_desktop"; the spec's
// guessed "codex_cli_rs" / "codex_vscode" were not found on disk, so this
// matches by substring rather than an exact enum.
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
		return "unknown"
	}
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

// Parse reads path from byte offset from to EOF and returns one Event per
// token_count line found, plus the byte offset right after the last
// complete line consumed. A trailing partial line (the file still being
// written) is left for the next call, same contract as the claude adapter.
//
// Model is the most recent turn_context.model seen since from: a session
// that switches model mid-file (rare, but real: see codex-auto-review and
// the model changes across Wilco's own trail) is picked up correctly within
// one read, but a token_count line appearing in a chunk that starts after
// its turn_context (a read resumed mid-turn) carries no model, matching the
// claude adapter's own precedent of only tracking state within one read.
func (Adapter) Parse(path string, from int64) ([]schema.Event, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, from, err
	}
	defer f.Close()

	if from > 0 {
		if _, err := f.Seek(from, io.SeekStart); err != nil {
			return nil, from, err
		}
	}

	base := filepath.Base(path)
	sessionID := strings.TrimSuffix(base, filepath.Ext(base))

	cwd := ""
	model := ""
	surface := "unknown"

	var events []schema.Event
	offset := from
	reader := bufio.NewReader(f)
	for {
		rawLine, readErr := reader.ReadString('\n')
		if readErr != nil {
			if readErr == io.EOF {
				break // trailing partial line, or nothing left; leave unconsumed
			}
			return nil, from, readErr
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
				if c, ok := payload["cwd"].(string); ok && cwd == "" {
					cwd = c
				}
				if o, ok := payload["originator"].(string); ok {
					surface = classifySurface(o)
				}
			}
			continue
		case "turn_context":
			if payload != nil {
				if c, ok := payload["cwd"].(string); ok && cwd == "" {
					cwd = c
				}
				if m, ok := payload["model"].(string); ok && m != "" {
					model = m
				}
			}
			continue
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

	return events, offset, nil
}
