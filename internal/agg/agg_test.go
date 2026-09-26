package agg

import (
	"testing"
	"time"

	"burnmon/internal/scan"
)

// TestBuild_CutoffComparesLocalDaysNotUTCInstant guards a bug a fresh review
// caught before this landed: cutoff (internal/dataset's monthStart, Wilco's
// decision, 2026-09-26: local time everywhere) carries a local-midnight
// instant, but Build used to compare it against s.End (a UTC-instant-
// labelled string, deliberately left that way) and against parseDay(daystr)
// (which anchors any date-only string to UTC midnight, regardless of what
// calendar day it actually names). A session whose last real activity falls
// in the early local morning hours right at the cutoff boundary has a UTC
// End-day one calendar day earlier than its true local day, so the old
// instant comparison (ed.Before(cutoff)) dropped the whole session even
// though its own s.Daily correctly (post the local-time fix) placed it
// inside the retained window.
func TestBuild_CutoffComparesLocalDaysNotUTCInstant(t *testing.T) {
	ams, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Skip("Europe/Amsterdam tzdata not available:", err)
	}
	// The cutoff a "keep from 2026-09-01" local-month window would use:
	// local midnight, 2026-09-01 CEST = 2026-08-31 22:00 UTC.
	cutoff := time.Date(2026, 9, 1, 0, 0, 0, 0, ams)

	// The session's one and only real activity: 2026-09-01 01:00 CEST
	// (2026-08-31 23:00 UTC), the distinguishing early-local-morning case.
	// Start/End mirror fromstore.go's own stamps format (a bare UTC-instant
	// string); Daily mirrors its buildSession fix (local calendar day key).
	s := &scan.Session{
		SessionID: "s1",
		Start:     "2026-08-31T23:00:00.000Z",
		End:       "2026-08-31T23:00:00.000Z",
		Daily: map[string]*scan.PerModel{
			"2026-09-01": {Calls: 1, Tokens: 100},
		},
		Models: map[string]*scan.PerModel{},
	}

	months, weeks, days, kept := Build([]*scan.Session{s}, cutoff)
	if len(kept) != 1 {
		t.Fatalf("len(kept) = %d, want 1 (the session's true local day, 2026-09-01, is inside the retained window)", len(kept))
	}
	if _, ok := days["2026-09-01"]; !ok {
		t.Errorf("days has no 2026-09-01 bucket, got keys %v", keysOfDayMap(days))
	}
	if _, ok := months["2026-09"]; !ok {
		t.Errorf("months has no 2026-09 bucket, got keys %v", keysOfDayMap(months))
	}
	_ = weeks
}

func keysOfDayMap(m map[string]*Bucket) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
