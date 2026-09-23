// Package copilotvsc reads the JSONL file GitHub Copilot Chat in VS Code
// writes when its own OpenTelemetry export is turned on, and turns it into
// schema.Events.
//
// Confirmed on a live file (SESSION_LOG.md, 2026-09-23 A3 correction, and
// this A4 session): four settings in %APPDATA%\Code\User\settings.json
// (github.copilot.chat.otel.enabled, .exporterType "file", .outfile, and
// .dbSpanExporter.enabled) make the extension append one JSON object per
// line, mixing OTel log records (a span/event, carrying "attributes") with
// OTel metric records (carrying "scopeMetrics", no "attributes" at all) and
// occasional blank "{}" lines; this package reads only the log records and
// ignores the rest. Every log record whose attributes["event.name"] is
// "gen_ai.client.inference.operation.details" is one LLM call: model
// (gen_ai.response.model, falling back to gen_ai.request.model) and tokens
// (gen_ai.usage.input_tokens / .output_tokens) come from its own
// attributes, keyed for dedup by gen_ai.response.id. That id is not always
// a wire "request" 1:1: a real multi-step agent turn recorded four lines
// sharing one response id, each with different token counts (60281/434,
// 91502/369, 95281/256, 103664/1197 tokens, in file order) — read as the
// same in-flight response being re-emitted as it grows, so "last write
// wins per request" (the spec's own phrase, decision #8 of the v0.3 grill)
// means keyed on response.id, keep the line last seen in file order, not
// the one with the largest output. session.id only appears as its own
// attribute on "copilot_chat.session.start" lines (a real per-conversation
// id, distinct from the constant resource-level session.id every line
// shares for the life of the VS Code window); every inference line between
// one session.start and the next is attributed to that session.
//
// No workspace/folder/cwd attribute of any kind was found on any line of
// the real file this was built from (476KB growing to 14.7MB over one
// working session, testdata\copilotvsc\copilotvsc_fixture.jsonl is a
// stripped, representative excerpt): "project from a workspace attribute
// where present" (spec 2.3 A4) has nothing to read in practice today, so
// every copilotvsc Event carries Project == "" (unassigned, same as any
// other adapter with no project info). workspaceAttrCandidates is kept as a
// forward-compatible, best-effort lookup in case a future extension version
// adds one; VERIFY per the spec's own section 6.
//
// No prompt or message text was found in any attribute value either (every
// value on every line is under 200 bytes): the "fixture ... stripped of
// prompt text" instruction turned out to already hold for this file's own
// shape, nothing needed removing.
package copilotvsc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"time"

	"burnmon/internal/schema"
)

// maxLineBytes bounds bufio.Scanner's per-line buffer, generously above
// anything seen on a real file (the largest line in the fixture is under
// 1KB; no attribute value observed anywhere carries prompt text, which is
// what would make a line grow large).
const maxLineBytes = 4 << 20

// logRecord is the shape of one OTel log-record line this package cares
// about. A metrics line ("scopeMetrics", no "attributes") or a blank "{}"
// line unmarshals into a zero-value logRecord (Attributes nil) and is
// skipped by parse.
type logRecord struct {
	HrTime     []float64      `json:"hrTime"`
	Attributes map[string]any `json:"attributes"`
}

// workspaceAttrCandidates are attribute names that might carry the VS Code
// workspace/folder path, per GitHub's own OTel documentation. None appeared
// on any span in the real file this adapter was built from (see the package
// doc); kept as a best-effort lookup, VERIFY per spec section 6.
var workspaceAttrCandidates = []string{
	"workspace.folder",
	"workspace.uri",
	"workspace.name",
	"vscode.workspace.folder",
	"cwd",
}

// PollOnce reads path (the VS Code otel.outfile the README's two settings
// produce) from the start and returns one Event per distinct
// gen_ai.response.id seen, last write wins in file order. No offset: like
// Hermes and Copilot CLI, this is a plain 5-second poll of the whole file,
// not an incremental tail (5-second poll, per the spec's own "like Hermes"
// phrasing); UpsertEvents' own dedup (largest output per RequestID) makes a
// repeat read of unchanged lines a no-op. path == "" (A4 not configured, no
// otel.outfile set in burnmon.json) and a path that does not exist yet (the
// setting is on but VS Code has not been reloaded, or no chat has been sent
// since) both return (nil, nil), not an error, the same "not installed" idiom
// hermes.DefaultDBPath and copilotcli.DefaultDBPath use.
func PollOnce(path string) ([]schema.Event, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	return parse(f)
}

func parse(r io.Reader) ([]schema.Event, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	sessionID := ""
	var order []string
	byRequest := map[string]schema.Event{}

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec logRecord
		if err := json.Unmarshal(line, &rec); err != nil || rec.Attributes == nil {
			// A metrics record, a blank "{}" line, or a malformed/partial
			// line (the file still being written): skip, not fatal.
			continue
		}

		eventName, _ := rec.Attributes["event.name"].(string)
		switch eventName {
		case "copilot_chat.session.start":
			if sid, ok := rec.Attributes["session.id"].(string); ok && sid != "" {
				sessionID = sid
			}
		case "gen_ai.client.inference.operation.details":
			e, ok := inferenceEvent(rec, sessionID)
			if !ok {
				continue
			}
			if _, seen := byRequest[e.RequestID]; !seen {
				order = append(order, e.RequestID)
			}
			byRequest[e.RequestID] = e // last write wins per request
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	events := make([]schema.Event, 0, len(order))
	for _, id := range order {
		events = append(events, byRequest[id])
	}
	return events, nil
}

func inferenceEvent(rec logRecord, sessionID string) (schema.Event, bool) {
	requestID, _ := rec.Attributes["gen_ai.response.id"].(string)
	if requestID == "" {
		return schema.Event{}, false
	}
	model, _ := rec.Attributes["gen_ai.response.model"].(string)
	if model == "" {
		model, _ = rec.Attributes["gen_ai.request.model"].(string)
	}

	e := schema.Event{
		Vendor:    "github",
		Agent:     "copilot-vscode",
		Surface:   "vscode",
		SessionID: sessionID,
		RequestID: requestID,
		Model:     model,
		Project:   workspaceProject(rec.Attributes),
		Input:     asInt(rec.Attributes["gen_ai.usage.input_tokens"]),
		Output:    asInt(rec.Attributes["gen_ai.usage.output_tokens"]),
	}
	if t, ok := hrTimeToTime(rec.HrTime); ok {
		e.At = t
	} else {
		e.At = time.Now().UTC()
	}
	return e, true
}

func workspaceProject(attrs map[string]any) string {
	for _, key := range workspaceAttrCandidates {
		if v, ok := attrs[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func asInt(v any) int64 {
	if f, ok := v.(float64); ok {
		return int64(f)
	}
	return 0
}

// hrTimeToTime converts an OTel JS SDK hrTime pair ([seconds, nanoseconds]
// since the Unix epoch, confirmed real on this laptop's own file: hrTime[0]
// values land squarely in 2026) into a UTC time.Time.
func hrTimeToTime(hr []float64) (time.Time, bool) {
	if len(hr) != 2 {
		return time.Time{}, false
	}
	return time.Unix(int64(hr[0]), int64(hr[1])).UTC(), true
}

