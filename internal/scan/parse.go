package scan

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"claudecost/internal/pricing"
)

// LongSessionCalls marks a session whose per-call cost has clearly entered
// the marathon regime.
const LongSessionCalls = 100

// SurfaceLabel maps a surface key to its display name.
var SurfaceLabel = map[string]string{
	"cowork":     "Claude Cowork",
	"code":       "Claude Code",
	"code_agent": "Claude Code, agent run",
	"chat":       "Claude Chat",
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
}

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

func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// cleanTitle turns a first user message into something a human recognises in
// a list. Slash commands arrive wrapped in tags; the command name is the
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

func classifySurface(path, cwd string) string {
	p := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	c := strings.ToLower(strings.ReplaceAll(cwd, "\\", "/"))
	if strings.Contains(p, "local-agent-mode-sessions") || strings.Contains(c, "local-agent-mode-sessions") {
		return "cowork"
	}
	if strings.Contains(c, ".worktrees") && strings.Contains(c, "sprint") {
		return "code_agent"
	}
	return "code"
}

func asInt(v any) int64 {
	if f, ok := v.(float64); ok {
		return int64(f)
	}
	return 0
}

func round6(x float64) float64 {
	return math.Round(x*1e6) / 1e6
}

// ToolGroup maps a raw tool name to the thing a human recognises: the
// connector it belongs to, the plugin-qualified skill it invoked, or
// "(built in)" for Claude's own tools.
//
// mcp__<server>__<tool> groups as <server>. A Skill call is pre-tagged by
// ParseSession as "skill:<name>", where <name> is whatever the Skill
// tool's own "skill" argument was (already "plugin:skill" for a plugin
// skill, or a bare name for an unscoped one); that group is <name>
// unchanged, so the plugin, if any, is right there in the label. Anything
// else (Read, Write, Bash, Grep, Glob, Task, Agent, WebSearch, WebFetch,
// ToolSearch, TodoWrite, an unidentified Skill call, and any future name)
// groups as "(built in)". A connector name that is a bare UUID or hex
// blob is passed through unchanged: resolving it into something readable
// needs data this package does not have, so display-side truncation is
// the caller's job.
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

type turn struct {
	ts, model                string
	fresh, cacheW, cacheR, o int64
}

// ParseSession reads one JSONL transcript and returns one session, or nil if
// it has no usable turns. Dedup rule ported exactly: one API call is streamed
// as several lines sharing a requestId with only output_tokens growing, so
// keep one entry per requestId, the one with the largest output_tokens.
func ParseSession(path string, cfg *pricing.Config) *Session {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	cwd := ""
	title := ""
	calls := map[string]*turn{}
	var order []string
	// toolCounts is keyed by the same requestId/uuid used to dedup usage
	// turns below: a streamed call repeats its tool_use blocks across
	// partial lines, so each key keeps the LARGEST per-name count seen on
	// any single line, mirroring the largest-output_tokens-wins rule.
	toolCounts := map[string]map[string]int64{}

	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}

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

		// Tool calls are counted on every assistant line, before the usage
		// check below continues past lines with no usage block: a tool_use
		// content block can appear on a line that carries no token usage.
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
				// A Skill call's own name is just "Skill" for every skill; the
				// actual skill (already "plugin:skill" when it came from a
				// plugin) sits in the tool's input, mirroring the "skill"
				// argument on the Skill tool itself. Re-tag the counted name so
				// ToolGroup can break skills out instead of folding every one
				// of them into "(built in)". An identifiable skill call that is
				// missing or malformed input falls back to plain "Skill", which
				// still groups as "(built in)".
				if name == "Skill" {
					if in, ok := block["input"].(map[string]any); ok {
						if sk, ok := in["skill"].(string); ok && sk != "" {
							name = "skill:" + sk
						}
					}
				}
				lineTools[name]++
			}
			if len(lineTools) > 0 {
				tk := ""
				if s, ok := obj["requestId"].(string); ok && s != "" {
					tk = s
				} else if s, ok := obj["uuid"].(string); ok && s != "" {
					tk = s
				} else {
					tk = fmt.Sprintf("_toolrow%d", i)
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
		e := &turn{
			ts:     ts,
			model:  model,
			fresh:  asInt(usage["input_tokens"]),
			cacheW: asInt(usage["cache_creation_input_tokens"]),
			cacheR: asInt(usage["cache_read_input_tokens"]),
			o:      asInt(usage["output_tokens"]),
		}
		prev := calls[key]
		if prev == nil {
			calls[key] = e
			order = append(order, key)
		} else if e.o >= prev.o {
			calls[key] = e
		}
	}

	// requestId/uuid is not applied to tool counts across the request's
	// own dedup key; each key's largest-per-name count is summed once here.
	toolTotals := map[string]int64{}
	for _, seen := range toolCounts {
		for name, c := range seen {
			toolTotals[name] += c
		}
	}
	var toolsOut map[string]int64
	if len(toolTotals) > 0 {
		toolsOut = toolTotals
	}

	var turns []*turn
	for _, k := range order {
		t := calls[k]
		// Turns with no model, or model <synthetic>, are not API calls.
		if t.model != "" && t.model != "<synthetic>" {
			turns = append(turns, t)
		}
	}
	if len(turns) == 0 {
		return nil
	}

	var stamps []string
	for _, t := range turns {
		if t.ts != "" {
			stamps = append(stamps, t.ts)
		}
	}
	if len(stamps) == 0 {
		return nil
	}
	sort.Strings(stamps)

	perModel := map[string]*PerModel{}
	daily := map[string]*PerModel{}
	var fresh, cacheW, cacheR, out int64
	var cost, costSub float64

	for _, t := range turns {
		fam := cfg.ModelFamily(t.model)
		c := cfg.CallCostUSD(fam, t.fresh, t.cacheW, t.cacheR, t.o)
		cs := cfg.CallCostSubUSD(fam, t.fresh, t.o)
		tokens := t.fresh + t.cacheW + t.cacheR + t.o
		fresh += t.fresh
		cacheW += t.cacheW
		cacheR += t.cacheR
		out += t.o
		cost += c
		costSub += cs

		label := cfg.Label(fam)
		pm := perModel[label]
		if pm == nil {
			pm = &PerModel{}
			perModel[label] = pm
		}
		pm.Calls++
		pm.Tokens += tokens
		pm.Cost += c
		pm.CostSub += cs

		ts := t.ts
		if ts == "" {
			ts = stamps[0]
		}
		day := ts
		if len(day) >= 10 {
			day = day[:10]
		}
		d := daily[day]
		if d == nil {
			d = &PerModel{}
			daily[day] = d
		}
		d.Calls++
		d.Tokens += tokens
		d.Cost += c
		d.CostSub += cs
	}

	for _, pm := range perModel {
		pm.Cost = round6(pm.Cost)
		pm.CostSub = round6(pm.CostSub)
	}
	for _, d := range daily {
		d.Cost = round6(d.Cost)
		d.CostSub = round6(d.CostSub)
	}

	callsN := int64(len(turns))
	base := filepath.Base(path)
	title2 := title
	if title2 == "" {
		title2 = "(untitled session)"
	}
	return &Session{
		SessionID:      strings.TrimSuffix(base, filepath.Ext(base)),
		Title:          title2,
		Surface:        classifySurface(path, cwd),
		CWD:            cwd,
		Start:          stamps[0],
		End:            stamps[len(stamps)-1],
		Calls:          callsN,
		Tokens:         fresh + cacheW + cacheR + out,
		Cost:           round6(cost),
		CostSub:        round6(costSub),
		CostPerCall:    round6(cost / float64(callsN)),
		CostSubPerCall: round6(costSub / float64(callsN)),
		Fresh:          fresh,
		CacheW:         cacheW,
		CacheR:         cacheR,
		Out:            out,
		Models:         perModel,
		Tools:          toolsOut,
		Daily:          daily,
		Long:           callsN > LongSessionCalls,
	}
}
