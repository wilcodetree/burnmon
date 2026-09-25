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
	day := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
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
	day := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
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
	yesterday := time.Date(2026, 9, 25, 23, 59, 0, 0, time.UTC)
	today := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
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
