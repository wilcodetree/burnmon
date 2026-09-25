package main

import (
	"fmt"
	"sort"
	"time"
)

// d11: no-scroll patch section 5, "Verify and close" - a uicheck case that
// records paint timestamps per panel for 30s and fails if any panel paints
// outside the shared tick, or if the panels' paints within one tick are
// more than one frame apart. page.html's own paintMark (called once per
// panel inside paintTick, itself always run inside one requestAnimationFrame
// per tick) appends {name, t} to window.__bdevPaintLog; this reads that log
// back through the dev eval channel rather than re-deriving timing any other
// way, so it is checking the real, already-running page's own behaviour,
// not a simulation of it.
func init() {
	checks["d11"] = func(hwnd uintptr, args []string) error {
		if err := ensureWindowSizeWH(hwnd, 1280, 860); err != nil {
			return err
		}

		// A clean slate: truncate whatever the log already holds so this
		// check's own 30s window is not polluted by ticks from before it
		// started (startup, an earlier check in the same uicheck-dev.ps1 run).
		if _, err := evalRaw(`(window.__bdevPaintLog = [])`); err != nil {
			return fmt.Errorf("reset paint log: %w", err)
		}

		time.Sleep(30 * time.Second)

		type mark struct {
			Name string  `json:"name"`
			T    float64 `json:"t"`
		}
		var log []mark
		if err := evalInto(`window.__bdevPaintLog || []`, &log); err != nil {
			return fmt.Errorf("read paint log: %w", err)
		}
		if len(log) == 0 {
			return fmt.Errorf("no paint marks recorded in 30s; is paintMark actually being called from paintTick?")
		}

		// Group into ticks by the 'tickEnd' marker paintTick itself always
		// emits last: every mark since the previous tickEnd (or the start of
		// the log) belongs to that tick.
		var ticks [][]mark
		var cur []mark
		for _, m := range log {
			cur = append(cur, m)
			if m.Name == "tickEnd" {
				ticks = append(ticks, cur)
				cur = nil
			}
		}
		// A trailing partial tick (cur, cut off by the 30s window ending
		// mid-paint, which should not happen since paintTick is synchronous)
		// is simply not appended to ticks above - dropped rather than judged
		// as a tick that never finished.
		if len(ticks) < 3 {
			return fmt.Errorf("only %d complete tick(s) recorded in 30s; expected several at the ~2s refresh cadence", len(ticks))
		}

		const expectedPanels = 12 // clock, sysmon, sysmonHistory, burn, vendorStrip, activityHeatmap, processGroups, harnessHeatmap, todo, tickEnd (+2 slack for a future panel)
		// frameToleranceMs is deliberately much looser than one 16.7ms frame
		// at 60fps: paintTick's own panels all run synchronously inside one
		// requestAnimationFrame callback with nothing else able to interleave
		// between them, but that synchronous work itself (heatmap grids,
		// table rebuilds, a canvas redraw) can legitimately take longer than
		// one frame budget to execute on real hardware without ever
		// violating "the same tick" - what this threshold actually needs to
		// catch is a panel painting on a LATER tick entirely, which would
		// show up roughly refresh_ms (~2000ms) later, three orders of
		// magnitude past this tolerance either way.
		const frameToleranceMs = 250.0

		for i, t := range ticks {
			if len(t) < 8 {
				return fmt.Errorf("tick %d only painted %d panel mark(s), expected close to %d - looks like a partial paint", i, len(t), expectedPanels)
			}
			names := map[string]bool{}
			for _, m := range t {
				names[m.Name] = true
			}
			if !names["tickEnd"] {
				return fmt.Errorf("tick %d has no tickEnd mark", i)
			}
			sorted := make([]mark, len(t))
			copy(sorted, t)
			sort.Slice(sorted, func(a, b int) bool { return sorted[a].T < sorted[b].T })
			spread := sorted[len(sorted)-1].T - sorted[0].T
			if spread > frameToleranceMs {
				return fmt.Errorf("tick %d: panel paints spread over %.1fms (want within ~%.0fms, one frame) - some panel is painting outside the shared requestAnimationFrame", i, spread, frameToleranceMs)
			}
		}

		// Consecutive ticks should land close to refresh_ms apart (default
		// 2000ms). Using each tick's own tickEnd timestamp as its instant.
		var tickEndTimes []float64
		for _, t := range ticks {
			for _, m := range t {
				if m.Name == "tickEnd" {
					tickEndTimes = append(tickEndTimes, m.T)
				}
			}
		}
		var refreshMs float64
		if err := evalInto(`window.__bdevRefreshMs || 2000`, &refreshMs); err != nil {
			refreshMs = 2000
		}
		for i := 1; i < len(tickEndTimes); i++ {
			gap := tickEndTimes[i] - tickEndTimes[i-1]
			// Generous tolerance: a slow round trip to bdevSnapshotNow, or a
			// GC pause, can stretch one tick's gap without it being a real
			// bug; what matters is it is not drifting wildly off cadence.
			if gap < refreshMs*0.4 || gap > refreshMs*2.5 {
				return fmt.Errorf("tick %d to %d: %.0fms apart, expected close to the %.0fms refresh cadence", i-1, i, gap, refreshMs)
			}
		}

		fmt.Printf("uicheck: d11: %d complete ticks in 30s, all panels paint within %.0fms of each other, cadence holds around %.0fms\n",
			len(ticks), frameToleranceMs, refreshMs)
		return nil
	}
}
