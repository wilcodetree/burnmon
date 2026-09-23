package export

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"burnmon/internal/pricing"
	"burnmon/internal/schema"
)

func testConfig(t *testing.T) pricing.Config {
	t.Helper()
	cfg, err := pricing.Load("")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestBuild_DefaultLabel(t *testing.T) {
	cfg := testConfig(t)
	doc := Build(nil, &cfg, "", time.Now())
	if doc.Label != DefaultLabel {
		t.Fatalf("Label = %q, want %q", doc.Label, DefaultLabel)
	}
	if doc.Schema != Schema {
		t.Fatalf("Schema = %d, want %d", doc.Schema, Schema)
	}
}

func TestBuild_GroupsByDayOwnerClientVendorModel(t *testing.T) {
	cfg := testConfig(t)
	day1 := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	events := []schema.Event{
		{Vendor: "anthropic", SessionID: "s1", RequestID: "r1", At: day1, Model: "claude-sonnet-5",
			Owner: "ZND", Client: "burnmon", Input: 100, Output: 10},
		{Vendor: "anthropic", SessionID: "s1", RequestID: "r2", At: day1.Add(2 * time.Minute), Model: "claude-sonnet-5",
			Owner: "ZND", Client: "burnmon", Input: 50, Output: 5},
		{Vendor: "anthropic", SessionID: "s2", RequestID: "r3", At: day2, Model: "claude-sonnet-5",
			Owner: "ZND", Client: "burnmon", Input: 200, Output: 20},
		{Vendor: "openai", SessionID: "s3", RequestID: "r4", At: day1, Model: "gpt-5.6-terra",
			Owner: "Valona", Client: "unassigned", Input: 300, Output: 30},
	}
	doc := Build(events, &cfg, "dev-1", time.Now())
	if len(doc.Rows) != 3 {
		t.Fatalf("got %d rows, want 3 (two events share day/owner/client/vendor/model)", len(doc.Rows))
	}
	var found bool
	for _, r := range doc.Rows {
		if r.Day == "2026-09-20" && r.Owner == "ZND" && r.Client == "burnmon" && r.Vendor == "anthropic" {
			found = true
			if r.Input != 150 || r.Output != 15 {
				t.Fatalf("merged row Input/Output = %d/%d, want 150/15", r.Input, r.Output)
			}
			if r.ActiveMinutes != 2 {
				t.Fatalf("ActiveMinutes = %v, want 2 (a 2-minute gap under the default cutoff)", r.ActiveMinutes)
			}
		}
	}
	if !found {
		t.Fatal("did not find the merged 2026-09-20/ZND/burnmon/anthropic row")
	}
}

func TestBuild_SyntheticToolOnlyEventsExcluded(t *testing.T) {
	cfg := testConfig(t)
	events := []schema.Event{
		{Vendor: "anthropic", SessionID: "s1", RequestID: "tool-1", At: time.Now(), Model: "", Input: 0, Output: 0},
	}
	doc := Build(events, &cfg, "dev-1", time.Now())
	if len(doc.Rows) != 0 {
		t.Fatalf("got %d rows, want 0 (synthetic tool-only event must not produce a row)", len(doc.Rows))
	}
}

// TestExportedJSON_NoPathsOrSessionIDs is the leak scanner the session
// prompt asked for: it builds a document from events carrying a realistic
// Windows path and a UUID-shaped session id in fields Row never exposes
// (Project, SessionID, RequestID, Title all live on schema.Event, not on
// export.Row), marshals the document the same way the CLI will, and fails if
// either a path separator or a UUID-shaped session id pattern survives into
// the bytes actually written to disk.
func TestExportedJSON_NoPathsOrSessionIDs(t *testing.T) {
	cfg := testConfig(t)
	events := []schema.Event{
		{
			Vendor: "anthropic", SessionID: "4b1f2e3a-9c1d-4a2b-8e3f-1a2b3c4d5e6f",
			RequestID: "req-1", At: time.Now(), Model: "claude-sonnet-5",
			Project: `C:\ZND\projects\burnmon`, Title: "fix the export bug",
			Owner: "ZND", Client: "burnmon", Input: 100, Output: 10,
		},
	}
	doc := Build(events, &cfg, "dev-1", time.Now())
	b, err := json.MarshalIndent(doc, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)

	pathSep := regexp.MustCompile(`[\\/]`)
	if pathSep.MatchString(out) {
		t.Fatalf("export JSON contains a path separator:\n%s", out)
	}
	uuid := regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	if uuid.MatchString(out) {
		t.Fatalf("export JSON contains a UUID-shaped session id:\n%s", out)
	}
}

func TestPriceBookDates(t *testing.T) {
	cfg := testConfig(t)
	dates := PriceBookDates(&cfg)
	for _, key := range []string{"anthropic_api", "openai_api", "copilot_credits"} {
		if dates[key] == "" {
			t.Fatalf("PriceBookDates()[%q] is empty, want the compiled-in book date", key)
		}
	}
}
