// Package claude parses the JSONL session transcripts Claude Code and
// Cowork write, into schema.Events. Moved out of internal/scan/parse.go
// (v0.0.1) unchanged in its per-line logic; only the output shape changed,
// from a whole-file *scan.Session to incremental schema.Events.
package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"burnmon/internal/scan"
	"burnmon/internal/schema"
)

// Adapter implements adapter.Adapter for Claude Code and Cowork transcripts.
type Adapter struct{}

func (Adapter) Name() string { return "claude" }

func (Adapter) Roots() []string { return scan.DefaultSources() }

var tagNames = []string{
	"command-message", "command-name", "command-args", "system-reminder",
	"local-command-stdout", "local-command-stderr",
}

var tagRes = func() []*regexp.Regexp {
	rs := make([]*regexp.Regexp, 0, len(tagNames))
	for _, t := range tagNames {
		rs = append(rs, regexp.MustCompile(`(?s)<`+t+`>.*?</`+t+`>`))
	}
	return rs
}()

var cmdNameRe = regexp.MustCompile(`(?s)<command-name>(.*?)</command-name>`)
var anyTagRe = regexp.MustCompile(`<[^>]{1,40}>`)

func collapseWS(s string) string { return strings.Join(strings.Fields(s), " ") }

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// cleanTitle turns a first user message into something a human recognises
// in a list. Slash commands arrive wrapped in tags; the command name is the
// useful bit.
func cleanTitle(text string) string {
	if text == "" {
		return ""
	}
	cmd := cmdNameRe.FindStringSubmatch(text)
	stripped := text
	for _, re := range tagRes {
		stripped = re.ReplaceAllString(stripped, " ")
	}
	stripped = anyTagRe.ReplaceAllString(stripped, " ")
	stripped = collapseWS(stripped)
	if cmd != nil {
		name := strings.TrimLeft(collapseWS(cmd[1]), "/")
		return truncRunes(strings.TrimSpace("/"+name+" "+stripped), 110)
	}
	return truncRunes(stripped, 110)
}

func extractText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, b := range v {
			switch bb := b.(type) {
			case map[string]any:
				if bb["type"] == "text" {
					if t, ok := bb["text"].(string); ok {
						parts = append(parts, t)
					}
				}
			case string:
				parts = append(parts, bb)
			}
		}
		return strings.Join(parts, " ")
	}
	return ""
}

// classifySurface maps a trail file to the report's surface vocabulary.
// cowork and chat (both Claude Desktop) merge into "desktop"; code_agent
// (worktree/sprint agent runs) keeps its own bucket since it is a run-type
// distinction, not a runtime surface. See the Step 1 plan's "Decisions
// locked in" section.
func classifySurface(path, cwd string) string {
	p := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	c := strings.ToLower(strings.ReplaceAll(cwd, "\\", "/"))
	if strings.Contains(p, "local-agent-mode-sessions") || strings.Contains(c, "local-agent-mode-sessions") {
		return "desktop"
	}
	if strings.Contains(c, ".worktrees") && strings.Contains(c, "sprint") {
		return "code_agent"
	}
	return "cli"
}

func asInt(v any) int64 {
	if f, ok := v.(float64); ok {
		return int64(f)
	}
	return 0
}

// skillTag mirrors scan.ToolGroup's Skill re-tagging: a Skill tool call's
// own name is "Skill" for every skill, the real skill sits in the input.
// It re-tags to "skill:"+name, keeping the prefix; scan.ToolGroup strips
// it later, at report-aggregation time, not here.
func skillTag(name string, input map[string]any) string {
	if name != "Skill" {
		return name
	}
	if sk, ok := input["skill"].(string); ok && sk != "" {
		return "skill:" + sk
	}
	return name
}

// Parse reads path from byte offset from to EOF and returns one Event per
// distinct API call turn found (deduped by requestId/uuid, largest
// output_tokens wins, exactly as v0.0.1's ParseSession did), plus the byte
// offset right after the last complete line consumed. A trailing partial
// line (the file still being written) is left for the next call.
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

	cwd := ""
	title := ""
	type turn struct {
		key                      string
		ts                       string
		model                    string
		fresh, cacheW, cacheR, o int64
	}
	calls := map[string]*turn{}
	var order []string
	toolCounts := map[string]map[string]int64{}

	offset := from
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	rowIdx := 0
	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		lineLen := int64(len(lineBytes)) + 1 // + the newline the scanner stripped
		line := strings.TrimSpace(string(lineBytes))
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
		rowIdx++

		if cwd == "" {
			if c, ok := obj["cwd"].(string); ok && c != "" {
				cwd = c
			}
		}
		if title == "" && obj["type"] == "user" {
			if msg, ok := obj["message"].(map[string]any); ok {
				title = cleanTitle(extractText(msg["content"]))
			}
		}
		if obj["type"] != "assistant" {
			continue
		}
		msg, ok := obj["message"].(map[string]any)
		if !ok {
			continue
		}

		if content, ok := msg["content"].([]any); ok {
			lineTools := map[string]int64{}
			for _, item := range content {
				block, ok := item.(map[string]any)
				if !ok || block["type"] != "tool_use" {
					continue
				}
				name, ok := block["name"].(string)
				if !ok || name == "" {
					continue
				}
				input, _ := block["input"].(map[string]any)
				lineTools[skillTag(name, input)]++
			}
			if len(lineTools) > 0 {
				tk := ""
				if s, ok := obj["requestId"].(string); ok && s != "" {
					tk = s
				} else if s, ok := obj["uuid"].(string); ok && s != "" {
					tk = s
				} else {
					tk = fmt.Sprintf("_toolrow%d", rowIdx)
				}
				seen := toolCounts[tk]
				if seen == nil {
					seen = map[string]int64{}
					toolCounts[tk] = seen
				}
				for name, c := range lineTools {
					if c > seen[name] {
						seen[name] = c
					}
				}
			}
		}

		usage, ok := msg["usage"].(map[string]any)
		if !ok || len(usage) == 0 {
			continue
		}
		key := ""
		if s, ok := obj["requestId"].(string); ok && s != "" {
			key = s
		} else if s, ok := obj["uuid"].(string); ok && s != "" {
			key = s
		} else {
			key = fmt.Sprintf("_row%d", len(order))
		}
		model, _ := msg["model"].(string)
		ts, _ := obj["timestamp"].(string)
		if model == "" || model == "<synthetic>" {
			continue
		}
		t := &turn{
			key: key, ts: ts, model: model,
			fresh:  asInt(usage["input_tokens"]),
			cacheW: asInt(usage["cache_creation_input_tokens"]),
			cacheR: asInt(usage["cache_read_input_tokens"]),
			o:      asInt(usage["output_tokens"]),
		}
		prev := calls[key]
		if prev == nil {
			calls[key] = t
			order = append(order, key)
		} else if t.o >= prev.o {
			calls[key] = t
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, from, err
	}

	surface := classifySurface(path, cwd)
	base := filepath.Base(path)
	sessionID := strings.TrimSuffix(base, filepath.Ext(base))

	var events []schema.Event
	consumedToolKeys := map[string]bool{}
	for _, k := range order {
		t := calls[k]
		e := schema.Event{
			Vendor: "anthropic", Agent: "claude-code", Surface: surface,
			SessionID: sessionID, RequestID: k, Project: cwd, Title: title,
			Model: t.model, Input: t.fresh, Output: t.o,
		}
		if t.cacheW > 0 {
			v := t.cacheW
			e.CacheWrite = &v
		}
		if t.cacheR > 0 {
			v := t.cacheR
			e.CacheRead = &v
		}
		if t.ts != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, t.ts); err == nil {
				e.At = parsed.UTC()
			}
		}
		if tools := toolCounts[k]; len(tools) > 0 {
			e.Tools = tools
			consumedToolKeys[k] = true
		}
		events = append(events, e)
	}

	// Orphan tool-only lines: a requestId/uuid whose tool_use blocks never
	// had a matching usage line anywhere in this read. v0.0.1 still summed
	// these into the session's Tools total (toolCounts was summed
	// unconditionally), so they become their own zero-token Events here,
	// keyed by their own (unique, never a calls[] key) request id.
	for k, tools := range toolCounts {
		if consumedToolKeys[k] {
			continue
		}
		events = append(events, schema.Event{
			Vendor: "anthropic", Agent: "claude-code", Surface: surface,
			SessionID: sessionID, RequestID: k, Project: cwd, Title: title,
			Tools: tools,
		})
	}

	return events, offset, nil
}
