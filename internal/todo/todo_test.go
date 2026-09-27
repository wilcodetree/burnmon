package todo

import (
	"testing"
	"time"
)

// TestLocalDueDate_UTC is WS2 alpha.3 item 5's own regression gate: the old
// code compared dueDateTime.DateTime's leading yyyy-mm-dd straight against
// the local calendar date, which reads a late-evening UTC due date as one
// day early once converted to a positive UTC offset like Europe/Amsterdam.
// 22:30 UTC on 2026-09-27 is 00:30 CEST on 2026-09-28 - the two dates
// disagree, proving the conversion (not a bare substring) is what runs now.
func TestLocalDueDate_UTC(t *testing.T) {
	ams, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	dt := &dueDateTimeT{DateTime: "2026-09-27T22:30:00.0000000", TimeZone: "UTC"}
	got, ok := localDueDate(dt, ams)
	if !ok {
		t.Fatalf("localDueDate: not ok")
	}
	if want := "2026-09-28"; got != want {
		t.Fatalf("localDueDate = %q, want %q (22:30 UTC = 00:30 CEST the next day)", got, want)
	}
}

// TestLocalDueDate_NonUTCZone confirms a dueDateTime already expressed in
// the same zone as loc passes through unchanged (the common case once a
// tenant sends Prefer: outlook.timezone, which this package does not send
// yet, but dt.TimeZone should still be honored if Graph ever returns one).
func TestLocalDueDate_NonUTCZone(t *testing.T) {
	ams, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	dt := &dueDateTimeT{DateTime: "2026-09-27T23:30:00.0000000", TimeZone: "Europe/Amsterdam"}
	got, ok := localDueDate(dt, ams)
	if !ok {
		t.Fatalf("localDueDate: not ok")
	}
	if want := "2026-09-27"; got != want {
		t.Fatalf("localDueDate = %q, want %q (already Amsterdam local, no shift expected)", got, want)
	}
}

// TestLocalDueDate_DST crosses 2026-10-25's Europe/Amsterdam DST transition
// (clocks step back from CEST, UTC+2, to CET, UTC+1, at 03:00 CEST = 01:00
// UTC): a UTC due date well after that transition, on the transition day
// itself, must convert at the new, smaller UTC+1 offset, not the CEST
// offset the day started under - the same class of bug WS3 fixed elsewhere
// in this codebase (headline.go's headlineDayStart and friends). 22:30 UTC
// (well past the 01:00 UTC transition) is 23:30 CET the same day; a
// lingering-CEST bug (a fixed +2 instead of the zone's real, transitioned
// offset) would instead land on 00:30 the next day, 2026-10-26 - a
// genuinely different answer, not just a few hours' difference within the
// same day (an earlier version of this test used an instant 1.5h before
// the transition, which only ever exercised CEST both ways and could not
// have caught that class of bug).
func TestLocalDueDate_DST(t *testing.T) {
	ams, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	dt := &dueDateTimeT{DateTime: "2026-10-25T22:30:00.0000000", TimeZone: "UTC"}
	got, ok := localDueDate(dt, ams)
	if !ok {
		t.Fatalf("localDueDate: not ok")
	}
	if want := "2026-10-25"; got != want {
		t.Fatalf("localDueDate = %q, want %q (22:30 UTC = 23:30 CET the same day, after the transition; 2026-10-26 would mean this is still assuming the old CEST offset)", got, want)
	}
}

// TestLocalDueDate_UnrecognizedZoneFallsBackToUTC: a Windows time zone name
// (as opposed to an IANA one) is not something Go's tzdata can resolve;
// localDueDate's own doc comment documents the conservative fallback
// (treat as UTC rather than guess) rather than dropping the task or
// panicking.
func TestLocalDueDate_UnrecognizedZoneFallsBackToUTC(t *testing.T) {
	ams, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	dt := &dueDateTimeT{DateTime: "2026-09-27T22:30:00.0000000", TimeZone: "Pacific Standard Time"}
	got, ok := localDueDate(dt, ams)
	if !ok {
		t.Fatalf("localDueDate: not ok")
	}
	if want := "2026-09-28"; got != want {
		t.Fatalf("localDueDate = %q, want %q (unrecognized zone name should fall back to UTC, same as the UTC test above)", got, want)
	}
}

func TestLocalDueDate_NilOrShortDateTime(t *testing.T) {
	if _, ok := localDueDate(nil, time.UTC); ok {
		t.Fatalf("localDueDate(nil, ...) = ok, want not ok")
	}
	if _, ok := localDueDate(&dueDateTimeT{DateTime: "2026-09"}, time.UTC); ok {
		t.Fatalf("localDueDate with a too-short DateTime = ok, want not ok")
	}
}
