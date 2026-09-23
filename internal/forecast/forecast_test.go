package forecast

import (
	"path/filepath"
	"testing"
	"time"

	"burnmon/internal/schema"
	"burnmon/internal/store"
)

func mkEvent(vendor, agent, id string, at time.Time, tokens int64) schema.Event {
	return schema.Event{
		Vendor: vendor, Agent: agent, SessionID: id,
		RequestID: id + "-" + at.Format("20060102150405"),
		At:        at, Model: "m", Input: tokens,
	}
}

// TestPlanLineFourWeeksWeekdayAware is F1's spec'd fixture test: four weeks
// of identical per-weekday totals should reproduce exactly, day for day, as
// the plan line for the following week.
func TestPlanLineFourWeeksWeekdayAware(t *testing.T) {
	ws := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC) // Monday
	byDay := map[string]int64{}
	for w := 1; w <= 4; w++ {
		for i := 0; i < 7; i++ {
			d := ws.AddDate(0, 0, i-7*w)
			byDay[d.Format("2006-01-02")] = int64((i + 1) * 100)
		}
	}

	got := planLine(byDay, ws)
	if len(got) != 7 {
		t.Fatalf("len(plan) = %d, want 7", len(got))
	}
	for i, p := range got {
		wantDate := ws.AddDate(0, 0, i).Format("2006-01-02")
		wantTokens := int64((i + 1) * 100)
		if p.Date != wantDate || p.Tokens != wantTokens {
			t.Errorf("day %d = %+v, want {%s %d}", i, p, wantDate, wantTokens)
		}
	}
}

// TestBuildLockedWithZeroScoredWeeks is F1's spec'd gate test: a store with
// events but no scored week yet must show the gate, history only.
func TestBuildLockedWithZeroScoredWeeks(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	now := time.Date(2026, 9, 23, 15, 0, 0, 0, time.UTC) // Wednesday
	if err := st.UpsertEvents([]schema.Event{
		mkEvent("anthropic", "claude-code", "s1", now.Add(-2*time.Hour), 500),
	}); err != nil {
		t.Fatal(err)
	}

	p, err := Build(st, now)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Locked {
		t.Fatal("Locked = false, want true with zero scored weeks")
	}
	if p.Message != GateMessage {
		t.Errorf("Message = %q, want %q", p.Message, GateMessage)
	}
	if len(p.Plan) != 0 || len(p.Live) != 0 {
		t.Errorf("Plan/Live should be empty while locked, got plan=%v live=%v", p.Plan, p.Live)
	}
	if len(p.History) != 7 {
		t.Errorf("len(History) = %d, want 7", len(p.History))
	}
	if p.ScoredWeeks != 0 {
		t.Errorf("ScoredWeeks = %d, want 0", p.ScoredWeeks)
	}
}

// TestBuildOneScoredWeekShowsBand drives two calendar weeks the way the app
// actually would: a Build call inside week 1 records that week's plan
// (still locked, nothing scored yet); a Build call the following week finds
// week 1 has fully elapsed and scores it. That first scored week should
// unlock the plan/live chart and produce a non-zero error band.
func TestBuildOneScoredWeekShowsBand(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	week1 := time.Date(2026, 8, 31, 9, 0, 0, 0, time.UTC) // Monday, week 1

	// Four weeks of history before week 1, a flat 700 tokens/day every day,
	// so week 1's own plan (the weekday average of those four weeks) comes
	// out to 700/day, 4900 for the week.
	var seed []schema.Event
	for w := 1; w <= 4; w++ {
		for i := 0; i < 7; i++ {
			d := week1.AddDate(0, 0, i-7*w)
			seed = append(seed, mkEvent("anthropic", "claude-code", "seed", d, 700))
		}
	}
	if err := st.UpsertEvents(seed); err != nil {
		t.Fatal(err)
	}

	p1, err := Build(st, week1)
	if err != nil {
		t.Fatal(err)
	}
	if !p1.Locked {
		t.Fatal("week 1's own Build should still be locked: nothing scored yet")
	}

	// Week 1's actual: 1000/day, 7000 for the week, a different figure from
	// its 4900-token plan so the error band comes out non-zero.
	var actual []schema.Event
	for i := 0; i < 7; i++ {
		actual = append(actual, mkEvent("anthropic", "claude-code", "actual", week1.AddDate(0, 0, i), 1000))
	}
	if err := st.UpsertEvents(actual); err != nil {
		t.Fatal(err)
	}

	week2 := week1.AddDate(0, 0, 7)
	p2, err := Build(st, week2)
	if err != nil {
		t.Fatal(err)
	}
	if p2.Locked {
		t.Fatal("Locked = true, want false once one week is scored")
	}
	if p2.ScoredWeeks != 1 {
		t.Fatalf("ScoredWeeks = %d, want 1", p2.ScoredWeeks)
	}
	wantBand := float64(7000-4900) / 4900
	if diff := p2.ErrorBandPct - wantBand; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("ErrorBandPct = %v, want %v", p2.ErrorBandPct, wantBand)
	}
	if len(p2.Plan) != 7 || len(p2.Live) != 7 {
		t.Errorf("Plan/Live should be populated once unlocked, got plan=%d live=%d", len(p2.Plan), len(p2.Live))
	}
}
