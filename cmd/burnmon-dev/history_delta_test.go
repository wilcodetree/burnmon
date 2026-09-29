//go:build windows

package main

import (
	"testing"
	"time"

	"burnmon/internal/sysmon"
)

func sysBuf(start time.Time, n int) []sysmon.Sample {
	out := make([]sysmon.Sample, n)
	for i := range out {
		// sub-millisecond offset on purpose: the cursor is whole ms.
		out[i] = sysmon.Sample{Ts: start.Add(time.Duration(i)*time.Second + 456*time.Microsecond), CPUPct: float64(i)}
	}
	return out
}

func TestSysHistSinceDeltaAndFull(t *testing.T) {
	start := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	buf := sysBuf(start, 3600) // 60 min at 1s
	windowStart := start.Add(30 * time.Minute)

	full, isFull := sysHistSince(buf, 0, windowStart)
	if !isFull || len(full) != 1800 || full[0].Ts.Before(windowStart) {
		t.Fatalf("no cursor: full=%v len=%d", isFull, len(full))
	}

	cursor := buf[3597].Ts.UnixMilli()
	delta, isFull := sysHistSince(buf, cursor, windowStart)
	if isFull || len(delta) != 2 || delta[0].CPUPct != 3598 {
		t.Fatalf("delta: full=%v len=%d first=%v", isFull, len(delta), delta)
	}

	if d, f := sysHistSince(buf, buf[3599].Ts.UnixMilli(), windowStart); f || len(d) != 0 {
		t.Fatalf("up to date: full=%v len=%d", f, len(d))
	}
	if _, f := sysHistSince(buf, windowStart.Add(-time.Second).UnixMilli(), windowStart); !f {
		t.Fatal("cursor older than the window must be a full answer")
	}
	if _, f := sysHistSince(buf, buf[3599].Ts.Add(time.Second).UnixMilli(), windowStart); !f {
		t.Fatal("cursor newer than the newest sample must be a full answer")
	}
}

func TestGroupsHistSince(t *testing.T) {
	start := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	var buf []sysmon.ProcessGroupSample
	for i := 0; i < 10; i++ {
		ts := start.Add(time.Duration(i) * time.Second)
		buf = append(buf, sysmon.ProcessGroupSample{Ts: ts, Harness: "claude"}, sysmon.ProcessGroupSample{Ts: ts, Harness: "codex"})
	}
	d, f := groupsHistSince(buf, buf[15].Ts.UnixMilli())
	if f || len(d) != 4 {
		t.Fatalf("delta: full=%v len=%d", f, len(d))
	}
	if _, f := groupsHistSince(buf, start.Add(-time.Second).UnixMilli()); !f {
		t.Fatal("cursor older than the buffer must be a full answer")
	}
	if d, f := groupsHistSince(buf, 0); !f || len(d) != 20 {
		t.Fatalf("no cursor: full=%v len=%d", f, len(d))
	}
}

func TestPruneFrontCopiesOnceHalfIsDropped(t *testing.T) {
	buf := make([]int, 0, 16)
	dropped := 0
	for i := 0; i < 10; i++ {
		buf = append(buf, i)
	}
	buf = pruneFront(buf, 3, &dropped)
	if dropped != 3 || len(buf) != 7 || buf[0] != 3 {
		t.Fatalf("reslice: dropped=%d buf=%v", dropped, buf)
	}
	want := &buf[2]
	buf = pruneFront(buf, 2, &dropped) // 5 dropped vs 5 kept: not over half yet
	if dropped != 5 || &buf[0] != want {
		t.Fatalf("expected a plain reslice, dropped=%d", dropped)
	}
	buf = pruneFront(buf, 1, &dropped) // 6 dropped vs 4 kept: copy
	if dropped != 0 || len(buf) != 4 || buf[0] != 6 || cap(buf) > 8 {
		t.Fatalf("expected a fresh copy: dropped=%d buf=%v cap=%d", dropped, buf, cap(buf))
	}
	if got := pruneFront(buf, 0, &dropped); len(got) != 4 {
		t.Fatal("n=0 must be a no-op")
	}
}
