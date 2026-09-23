package main

import (
	"fmt"
	"time"
)

// w6: the Now chart's left axis ticks use two decimals (1.25M, 850.00K),
// never a whole-number round that repeats consecutive ticks (3M, 3M, 2M,
// 2M). Reproduced/verified by overriding window.bmLive with a session
// whose token total forces the axis max into a range where consecutive
// tick values would collide under whole-number rounding, then reading the
// real Chart.js scale's own generated tick labels.
//
// Args: "before" only captures and reports; anything else (the default,
// "after") also asserts.
func init() {
	checks["w6"] = func(hwnd uintptr, args []string) error {
		phase := "after"
		if len(args) > 0 {
			phase = args[0]
		}

		// 2,600,000 tokens: whole-number tok() rounds to "3M", and several
		// of Chart.js's own evenly-spaced ticks below that (e.g. 5/6 of
		// max = 2,166,666 and 4/6 = 1,733,333) round to the same "2M".
		overrideScript := `(function(){
  window.bmLive = function(){
    return Promise.resolve({
      chart: [{at: new Date().toISOString(), by_session: {
        w6sess1: {fresh: 2600000, cache_write: 0, cache_read: 0, output: 0}
      }, cost: 0}],
      sessions: [{session_id:'w6sess1', vendor:'claude', agent:'claude-code', surface:'cli', model:'m', project:'p'}],
      turns: []
    });
  };
  return true;
})()`
		if _, err := evalRaw(overrideScript); err != nil {
			return fmt.Errorf("override bmLive: %w", err)
		}
		time.Sleep(5 * time.Second)

		ticksScript := `(function(){
  var c = Object.values(Chart.instances).filter(function(c){return c.canvas.id==='ch_now'})[0];
  var scale = c.scales.y;
  return scale.ticks.map(function(t){ return scale.options.ticks.callback(t.value); });
})()`
		var ticks []string
		if err := evalInto(ticksScript, &ticks); err != nil {
			return fmt.Errorf("read axis ticks: %w", err)
		}
		fmt.Printf("uicheck: y-axis ticks = %v\n", ticks)

		if _, err := screenshot(hwnd, "w6-"+phase+"-axis-ticks"); err != nil {
			fmt.Printf("uicheck: screenshot failed (non-fatal, best effort only): %v\n", err)
		}

		if phase == "before" {
			return nil
		}

		seen := map[string]bool{}
		for _, t := range ticks {
			if t == "" {
				continue
			}
			if seen[t] {
				return fmt.Errorf("duplicate axis tick label %q in %v", t, ticks)
			}
			seen[t] = true
		}
		return nil
	}
}
