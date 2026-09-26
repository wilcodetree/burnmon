//go:build windows

package main

import (
	"testing"
	"time"
)

// TestParseExportTime_BareDayIsLocalNotUTC guards WS2 item 5: a bare
// YYYY-MM-DD --since/--until parsed with time.Parse (no Location) defaults
// to UTC, the same class of bug WS3 already fixed in cmd\burnmon-cli's own
// --since/--until (this file's own doc comment says it "matches burnmon-cli
// export's own... convention", modeled on that command's pre-WS3 behavior,
// but was never itself updated when burnmon-cli was). A bare day here must
// mean local midnight, like every other day boundary in cmd\burnmon-dev.
func TestParseExportTime_BareDayIsLocalNotUTC(t *testing.T) {
	got, err := parseExportTime("2026-09-26", false)
	if err != nil {
		t.Fatalf("parseExportTime: %v", err)
	}
	want := time.Date(2026, 9, 26, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Fatalf("parseExportTime(%q, false) = %v, want %v (local midnight, not UTC)", "2026-09-26", got, want)
	}
}

// TestParseExportTime_EndOfDayAddsOneLocalDay guards the same class of bug
// for the --until "through the end of that day" shift: AddDate on a
// UTC-located time adds a UTC calendar day (always exactly 24h), not a
// local one (23h/25h across a DST transition) - the exact mistake a fresh
// review caught elsewhere in WS3 (internal/forecast's AddDate bug).
func TestParseExportTime_EndOfDayAddsOneLocalDay(t *testing.T) {
	got, err := parseExportTime("2026-09-26", true)
	if err != nil {
		t.Fatalf("parseExportTime: %v", err)
	}
	want := time.Date(2026, 9, 27, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Fatalf("parseExportTime(%q, true) = %v, want %v (local midnight of the next day)", "2026-09-26", got, want)
	}
	if got.Location() != time.Local {
		t.Errorf("parseExportTime's Location = %v, want time.Local", got.Location())
	}
}

// TestParseExportTime_RFC3339PassesThrough confirms an explicit instant is
// never reinterpreted: only the bare-day form is Location-ambiguous.
func TestParseExportTime_RFC3339PassesThrough(t *testing.T) {
	got, err := parseExportTime("2026-09-26T14:30:00+02:00", false)
	if err != nil {
		t.Fatalf("parseExportTime: %v", err)
	}
	want, _ := time.Parse(time.RFC3339, "2026-09-26T14:30:00+02:00")
	if !got.Equal(want) {
		t.Fatalf("parseExportTime(RFC3339) = %v, want %v", got, want)
	}
}
