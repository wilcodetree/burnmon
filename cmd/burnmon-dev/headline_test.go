//go:build windows

package main

import (
	"testing"
	"time"

	"burnmon/internal/schema"
	"burnmon/internal/vendorstrip"
)

func ptr(n int64) *int64 { return &n }

func mkEvent(at time.Time, input, output int64, cacheRead, cacheWrite *int64) schema.Event {
	return schema.Event{At: at, Input: input, Output: output, CacheRead: cacheRead, CacheWrite: cacheWrite}
}

// TestHeadlineTodayTokens_AddsSinceCache is the ordinary case between two
// cache refreshes: events after the cache's own GeneratedAt add on top of
// the cached Today figure.
func TestHeadlineTodayTokens_AddsSinceCache(t *testing.T) {
	day := time.Date(2026, 9, 26, 0, 0, 0, 0, time.Local)
	cacheAt := day.Add(10 * time.Minute)
	now := day.Add(12 * time.Minute)

	vendorTotal := vendorstrip.Row{Today: 1000}
	events := []schema.Event{
		mkEvent(day.Add(5*time.Minute), 100, 50, nil, nil),  // before cache: already counted, must not add
		mkEvent(day.Add(11*time.Minute), 200, 30, nil, nil), // after cache: 230 new
	}

	got := headlineTodayTokens(vendorTotal, cacheAt.Format(time.RFC3339), events, now)
	want := int64(1000 + 230)
	if got != want {
		t.Fatalf("headlineTodayTokens = %d, want %d", got, want)
	}
}

// TestHeadlineTodayTokens_NoDoubleCountAfterRefresh simulates a cache
// refresh landing: the delta events from the previous tick are now folded
// into the cache's own Today figure and GeneratedAt moves past them, so
// re-running with the refreshed cache must not add them a second time, and
// the total must not drop (no jump back).
func TestHeadlineTodayTokens_NoDoubleCountAfterRefresh(t *testing.T) {
	day := time.Date(2026, 9, 26, 0, 0, 0, 0, time.Local)
	oldCacheAt := day.Add(10 * time.Minute)
	deltaEventAt := day.Add(11 * time.Minute)
	now := day.Add(12 * time.Minute)

	events := []schema.Event{mkEvent(deltaEventAt, 200, 30, nil, nil)}

	before := headlineTodayTokens(vendorstrip.Row{Today: 1000}, oldCacheAt.Format(time.RFC3339), events, now)
	if before != 1230 {
		t.Fatalf("before refresh: got %d, want 1230", before)
	}

	// The 60s cache refresh now runs, folding the 230 delta into Today and
	// moving GeneratedAt to (or past) deltaEventAt.
	after := headlineTodayTokens(vendorstrip.Row{Today: 1230}, deltaEventAt.Format(time.RFC3339), events, now)
	if after != 1230 {
		t.Fatalf("after refresh: got %d, want 1230 (no double count)", after)
	}
	if after < before {
		t.Fatalf("headline jumped back after cache refresh: before=%d after=%d", before, after)
	}
}

// TestHeadlineTodayTokens_CacheCrossesMidnight: a cache built before
// today's own start describes a day that has already ended, so it is not
// carried forward; only the fetched window's own today-events count.
func TestHeadlineTodayTokens_CacheCrossesMidnight(t *testing.T) {
	yesterday := time.Date(2026, 9, 25, 23, 59, 0, 0, time.Local)
	today := time.Date(2026, 9, 26, 0, 0, 0, 0, time.Local)
	now := today.Add(1 * time.Minute)

	events := []schema.Event{
		mkEvent(yesterday, 9000, 9000, nil, nil), // yesterday: must not count
		mkEvent(today.Add(30*time.Second), 40, 10, nil, nil),
	}

	got := headlineTodayTokens(vendorstrip.Row{Today: 555_000}, yesterday.Format(time.RFC3339), events, now)
	if got != 50 {
		t.Fatalf("headlineTodayTokens across midnight = %d, want 50 (stale cache dropped)", got)
	}
}

// TestHeadlineTodayMonotonic_NeverDipsOnStaleCache reproduces the gap found
// by review: a vendor_strip cache stalled for longer than live.ChartWindow,
// so an event counted on one tick later ages out of the fetched window
// while the cache still has not refreshed. The bare headlineTodayTokens
// would then report a lower number than before (its own doc comment says
// so); headlineTodayMonotonic must hold the floor instead.
func TestHeadlineTodayMonotonic_NeverDipsOnStaleCache(t *testing.T) {
	day := time.Date(2026, 9, 26, 0, 0, 0, 0, time.Local)
	cacheAt := day.Add(5 * time.Minute) // never refreshes again in this test
	vendorTotal := vendorstrip.Row{Today: 1000}
	tickEvent := mkEvent(cacheAt.Add(1*time.Minute), 500, 0, nil, nil)

	a := &app{}

	// Tick 1: the event is still inside the fetched window and after
	// cacheAt, so it adds on top of the cached base.
	now1 := cacheAt.Add(20 * time.Minute)
	got1 := a.headlineTodayMonotonic(vendorTotal, cacheAt.Format(time.RFC3339), []schema.Event{tickEvent}, now1)
	if got1 != 1500 {
		t.Fatalf("tick1 = %d, want 1500", got1)
	}

	// Tick 2: 35 minutes after the event, past live.ChartWindow (30 min),
	// so it no longer appears in the fetched events at all; the cache
	// still has not refreshed. Confirm the bare function really would dip
	// here (the gap this test guards against), then confirm the wrapper
	// does not.
	now2 := tickEvent.At.Add(35 * time.Minute)
	if bare := headlineTodayTokens(vendorTotal, cacheAt.Format(time.RFC3339), nil, now2); bare != 1000 {
		t.Fatalf("sanity: bare headlineTodayTokens = %d, want 1000 (demonstrating the gap this test guards against)", bare)
	}
	got2 := a.headlineTodayMonotonic(vendorTotal, cacheAt.Format(time.RFC3339), nil, now2)
	if got2 != 1500 {
		t.Fatalf("tick2 = %d, want 1500 (monotonic floor must hold, not dip)", got2)
	}
}

// TestHeadlineTodayMonotonic_DayRolloverResets: a genuine day boundary must
// still reset to the new day's own low total, not be clamped against the
// previous day's high one - that is the one intentional exception to
// "never dips".
func TestHeadlineTodayMonotonic_DayRolloverResets(t *testing.T) {
	yesterday := time.Date(2026, 9, 25, 23, 0, 0, 0, time.Local)
	today := time.Date(2026, 9, 26, 0, 30, 0, 0, time.Local)

	a := &app{}
	if got := a.headlineTodayMonotonic(vendorstrip.Row{Today: 900_000}, yesterday.Format(time.RFC3339), nil, yesterday.Add(10*time.Minute)); got != 900_000 {
		t.Fatalf("day1 = %d, want 900000", got)
	}
	if got := a.headlineTodayMonotonic(vendorstrip.Row{Today: 40}, today.Format(time.RFC3339), nil, today); got != 40 {
		t.Fatalf("day2 = %d, want 40 (new day resets, not clamped to yesterday's 900000)", got)
	}
}

// TestHeadlineDayStart_LocalNotUTC guards WS2 item 5: headlineDayStart used
// to be a UTC copy left behind when vendorstrip's own dayStart switched to
// time.Local (WS3, "local time everywhere"), so it silently disagreed with
// vendorStrip.Total.Today's own day boundary for part of every day (the gap
// between UTC midnight and local midnight, or vice versa, depending on the
// machine's own offset). Checked directly against both 2026 Europe/Amsterdam
// DST transition dates, same recipe internal/forecast's own DST test uses:
// a fixed instant just after local midnight must resolve to that same local
// midnight, not a UTC-anchored one, on both the spring-forward and
// fall-back day.
func TestHeadlineDayStart_LocalNotUTC(t *testing.T) {
	ams, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Skip("Europe/Amsterdam tzdata not available:", err)
	}
	cases := []struct {
		name       string
		day        time.Time // local midnight, Europe/Amsterdam
		wantOffset int       // expected UTC offset in seconds at that midnight
	}{
		{"spring-forward day (23h)", time.Date(2026, 3, 29, 0, 0, 0, 0, ams), 1 * 3600}, // still CET before the 02:00 jump
		{"fall-back day (25h)", time.Date(2026, 10, 25, 0, 0, 0, 0, ams), 2 * 3600},     // still CEST before the 03:00 fold
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// monday.Zone() in a forecast-test-style check would always
			// read ams's own offset, since tc.day was built with that
			// Location directly; what actually matters is what the real
			// machine's time.Local reads for this same instant.
			if _, off := tc.day.In(time.Local).Zone(); off != tc.wantOffset {
				t.Skipf("this machine's Europe/Amsterdam offset at %s is not the expected %+dh; skipping a DST test that assumes it", tc.day, tc.wantOffset/3600)
			}
			now := tc.day.Add(1 * time.Hour) // well past local midnight, still the same local day, before that day's own DST transition
			got := headlineDayStart(now)
			wantInstant := time.Date(tc.day.Year(), tc.day.Month(), tc.day.Day(), 0, 0, 0, 0, time.Local)
			if !got.Equal(wantInstant) {
				t.Fatalf("headlineDayStart(%v) = %v, want %v (local midnight, not a UTC-anchored one)", now, got, wantInstant)
			}
			if got.Location() != time.Local {
				t.Errorf("headlineDayStart's Location = %v, want time.Local", got.Location())
			}
		})
	}
}

// TestEventTokens_SumsAllFourClasses matches internal/store.VendorStripTotals'
// own SQL sum (input + cache_write + cache_read + output).
func TestEventTokens_SumsAllFourClasses(t *testing.T) {
	e := mkEvent(time.Now(), 10, 20, ptr(3), ptr(4))
	if got := eventTokens(e); got != 37 {
		t.Fatalf("eventTokens = %d, want 37", got)
	}
	// nil CacheRead/CacheWrite (a vendor with no such class) must not panic
	// and must not count as anything.
	e2 := mkEvent(time.Now(), 10, 20, nil, nil)
	if got := eventTokens(e2); got != 30 {
		t.Fatalf("eventTokens with nil cache fields = %d, want 30", got)
	}
}
