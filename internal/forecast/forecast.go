// Package forecast builds the Now page's F1 forecast: a weekday-aware plan
// line from the last four weeks, a live line that continues today's own
// rate to end of day and end of month, and an error band from every scored
// week so far. Gated: until one scored week exists (a week with both a
// recorded plan and a recorded actual), the payload carries only the last
// seven days of history and a fixed message, no plan or live line at all.
// Refreshed once a minute from its own timer (bmForecast), the same
// "not on the 2-second poll" pattern internal/vendorstrip already uses,
// since every query here scans the whole events table rather than
// internal/live's 30-minute window.
package forecast

import (
	"time"

	"burnmon/internal/store"
)

// GateMessage is F1's fixed gate text, shown until one scored week exists.
const GateMessage = "forecast unlocks after the first scored week (week 46)"

// Point is one day's token total, used for both the history and the plan
// and live lines.
type Point struct {
	Date   string `json:"date"` // YYYY-MM-DD, UTC
	Tokens int64  `json:"tokens"`
}

// Payload is bmForecast's return value.
type Payload struct {
	GeneratedAt string `json:"generated_at"`
	// Locked is true until one scored week exists; Plan, Live and
	// ErrorBandPct are all empty/zero while it is.
	Locked  bool    `json:"locked"`
	Message string  `json:"message"`
	History []Point `json:"history"` // last 7 days, always present
	// Plan is F1's weekday-aware plan line: one point per day of the
	// current ISO week (Monday first), each day's tokens the average of
	// that same weekday over its last 4 occurrences.
	Plan []Point `json:"plan,omitempty"`
	// Live is F1's current-rate line: actual tokens for every day of the
	// current week already elapsed, today's own rate continued for the
	// rest of the week.
	Live             []Point `json:"live,omitempty"`
	EndOfDayTokens   int64   `json:"end_of_day_tokens,omitempty"`
	EndOfMonthTokens int64   `json:"end_of_month_tokens,omitempty"`
	// ErrorBandPct is the mean absolute percentage error between plan and
	// actual across every scored week, e.g. 0.18 for +/-18%.
	ErrorBandPct float64 `json:"error_band_pct,omitempty"`
	ScoredWeeks  int     `json:"scored_weeks"`
}

func dayStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// weekStart is the Monday 00:00 UTC of t's ISO week, matching
// internal/vendorstrip's own convention.
func weekStart(t time.Time) time.Time {
	d := dayStart(t)
	offset := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -offset)
}

func monthStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func dateKey(t time.Time) string { return t.Format("2006-01-02") }

// Build is bmForecast's implementation: runs EnsureScored first so the
// current week always has a plan on record and any week that just elapsed
// gets its actual filled in, then reads the gate state and, once unlocked,
// the plan, live and error-band figures.
func Build(st *store.Store, now time.Time) (Payload, error) {
	if err := EnsureScored(st, now); err != nil {
		return Payload{}, err
	}

	scores, err := st.ForecastScores()
	if err != nil {
		return Payload{}, err
	}
	scored := 0
	var errSum float64
	for _, sc := range scores {
		if sc.ActualTokens == nil {
			continue
		}
		scored++
		if sc.PlanTokens > 0 {
			errSum += absFloat(float64(*sc.ActualTokens-sc.PlanTokens)) / float64(sc.PlanTokens)
		}
	}

	histFrom := dayStart(now).AddDate(0, 0, -6)
	histDaily, err := st.DailyTokenTotals(histFrom)
	if err != nil {
		return Payload{}, err
	}
	p := Payload{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		History:     historyPoints(dailyMap(histDaily), now),
		ScoredWeeks: scored,
	}
	if scored == 0 {
		p.Locked = true
		p.Message = GateMessage
		return p, nil
	}
	p.ErrorBandPct = errSum / float64(scored)

	ws := weekStart(now)
	planFrom := ws.AddDate(0, 0, -28)
	planDaily, err := st.DailyTokenTotals(planFrom)
	if err != nil {
		return Payload{}, err
	}
	byDay := dailyMap(planDaily)
	p.Plan = planLine(byDay, ws)
	p.Live, p.EndOfDayTokens, p.EndOfMonthTokens = liveLine(byDay, ws, now)
	return p, nil
}

func dailyMap(daily []store.DailyTokenTotal) map[string]int64 {
	m := make(map[string]int64, len(daily))
	for _, d := range daily {
		m[d.Date] = d.Tokens
	}
	return m
}

func historyPoints(byDay map[string]int64, now time.Time) []Point {
	today := dayStart(now)
	from := today.AddDate(0, 0, -6)
	out := make([]Point, 0, 7)
	for d := from; !d.After(today); d = d.AddDate(0, 0, 1) {
		key := dateKey(d)
		out = append(out, Point{Date: key, Tokens: byDay[key]})
	}
	return out
}

// planLine is F1's weekday-aware plan: for each day of the ISO week
// starting ws (Monday first), the average of that weekday's tokens over
// its last 4 occurrences before ws. A weekday with fewer than 4 prior
// occurrences in byDay (a young store) averages over whatever it has.
func planLine(byDay map[string]int64, ws time.Time) []Point {
	out := make([]Point, 7)
	for i := 0; i < 7; i++ {
		d := ws.AddDate(0, 0, i)
		var sum int64
		var n int
		for w := 1; w <= 4; w++ {
			key := dateKey(d.AddDate(0, 0, -7*w))
			if v, ok := byDay[key]; ok {
				sum += v
				n++
			}
		}
		var avg int64
		if n > 0 {
			avg = sum / int64(n)
		}
		out[i] = Point{Date: dateKey(d), Tokens: avg}
	}
	return out
}

// liveLine is F1's current-rate line: actual tokens for every already-past
// day of the current week (today included, its own partial total), and
// today's own rate (tokens so far / fraction of the day elapsed) held flat
// for the remaining days of the week. The same rate, applied over the whole
// month elapsed so far instead of just today, gives EndOfMonthTokens;
// EndOfDayTokens is that same projection for today alone.
func liveLine(byDay map[string]int64, ws time.Time, now time.Time) (line []Point, eod, eom int64) {
	today := dayStart(now)
	elapsed := now.Sub(today)
	if elapsed <= 0 {
		elapsed = time.Minute
	}
	fracDay := elapsed.Seconds() / (24 * time.Hour).Seconds()
	todayTokens := byDay[dateKey(today)]
	if fracDay > 0 {
		eod = int64(float64(todayTokens) / fracDay)
	}
	if eod < todayTokens {
		eod = todayTokens
	}

	line = make([]Point, 7)
	for i := 0; i < 7; i++ {
		d := ws.AddDate(0, 0, i)
		key := dateKey(d)
		switch {
		case d.Before(today):
			line[i] = Point{Date: key, Tokens: byDay[key]}
		default: // today, or a day later this week: today's own rate, held flat
			line[i] = Point{Date: key, Tokens: eod}
		}
	}

	ms := monthStart(now)
	var monthSoFar int64
	for d := ms; !d.After(today); d = d.AddDate(0, 0, 1) {
		monthSoFar += byDay[dateKey(d)]
	}
	daysElapsedMonth := today.Sub(ms).Hours()/24 + fracDay
	nextMonth := time.Date(ms.Year(), ms.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
	daysInMonth := nextMonth.AddDate(0, 0, -1).Day()
	if daysElapsedMonth > 0 {
		eom = int64(float64(monthSoFar) / daysElapsedMonth * float64(daysInMonth))
	}
	if eom < monthSoFar {
		eom = monthSoFar
	}
	return line, eod, eom
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// EnsureScored is F1's scoring bookkeeping, run at the start of every Build:
// the current ISO week gets its plan recorded the first time this sees
// that week (InsertForecastPlan's own ON CONFLICT DO NOTHING makes a later
// call the same week a no-op, so nothing already on the books is
// overwritten), and every earlier week that has now fully elapsed but has
// no actual yet gets one recorded, once.
func EnsureScored(st *store.Store, now time.Time) error {
	ws := weekStart(now)
	isoYear, isoWeek := ws.ISOWeek()
	if _, ok, err := st.ForecastScore(isoYear, isoWeek); err != nil {
		return err
	} else if !ok {
		plan, err := planTokensForWeek(st, ws)
		if err != nil {
			return err
		}
		if err := st.InsertForecastPlan(isoYear, isoWeek, ws, plan); err != nil {
			return err
		}
	}

	scores, err := st.ForecastScores()
	if err != nil {
		return err
	}
	for _, sc := range scores {
		if sc.ActualTokens != nil {
			continue
		}
		weekEnd := sc.WeekStart.AddDate(0, 0, 7)
		if now.Before(weekEnd) {
			continue
		}
		actual, err := actualTokensForWeek(st, sc.WeekStart)
		if err != nil {
			return err
		}
		if err := st.RecordForecastActual(sc.ISOYear, sc.ISOWeek, actual, now); err != nil {
			return err
		}
	}
	return nil
}

func planTokensForWeek(st *store.Store, ws time.Time) (int64, error) {
	daily, err := st.DailyTokenTotals(ws.AddDate(0, 0, -28))
	if err != nil {
		return 0, err
	}
	var total int64
	for _, p := range planLine(dailyMap(daily), ws) {
		total += p.Tokens
	}
	return total, nil
}

func actualTokensForWeek(st *store.Store, ws time.Time) (int64, error) {
	daily, err := st.DailyTokenTotals(ws)
	if err != nil {
		return 0, err
	}
	end := dateKey(ws.AddDate(0, 0, 7))
	var total int64
	for _, d := range daily {
		if d.Date >= end { // string dates: YYYY-MM-DD compares lexically as chronologically
			continue
		}
		total += d.Tokens
	}
	return total, nil
}
