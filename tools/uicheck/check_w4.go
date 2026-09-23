package main

import (
	"fmt"
	"time"
)

// w4: two sessions with chart bars but no entry in Sessions (the
// "recently went idle, still inside the chart's 30-minute window but
// outside live.BuildSnapshot's shorter runningWindow" case W4 traces the
// duplicate legend to) must not both render the same "recent session"
// text; the fix uses each session id's own first 8 characters, which are
// unique per session by construction. Reproduced/verified by overriding
// window.bmLive with two such sessions and waiting for the app's own
// 2-second poll to pick it up and redraw the real chart, then reading the
// Chart.js instance's own dataset labels.
//
// Args: "before" only captures and reports; anything else (the default,
// "after") also asserts.
func init() {
	checks["w4"] = func(hwnd uintptr, args []string) error {
		phase := "after"
		if len(args) > 0 {
			phase = args[0]
		}

		overrideScript := `(function(){
  var makeSnap = function(){
    return {
      chart: [{at: new Date().toISOString(), by_session: {
        idleAAAAsessAAAAAAAA: {fresh:1,cache_write:1,cache_read:1,output:1},
        idleBBBBsessBBBBBBBB: {fresh:2,cache_write:2,cache_read:2,output:2}
      }, cost: 0.01}],
      sessions: [],
      turns: []
    };
  };
  window.bmLive = function(){ return Promise.resolve(makeSnap()); };
  return true;
})()`
		if _, err := evalRaw(overrideScript); err != nil {
			return fmt.Errorf("override bmLive: %w", err)
		}

		// pollNow runs every 2s; give it two ticks to land and settle.
		time.Sleep(5 * time.Second)

		labelsScript := `(function(){
  var c = Object.values(Chart.instances).filter(function(c){return c.canvas.id==='ch_now'})[0];
  return c.data.datasets.filter(function(d){return d.sessionId}).map(function(d){return d.label});
})()`
		var labels []string
		if err := evalInto(labelsScript, &labels); err != nil {
			return fmt.Errorf("read legend labels: %w", err)
		}
		fmt.Printf("uicheck: legend labels = %v\n", labels)

		if _, err := screenshot(hwnd, "w4-"+phase+"-legend"); err != nil {
			fmt.Printf("uicheck: screenshot failed (non-fatal, best effort only): %v\n", err)
		}

		if phase == "before" {
			return nil
		}
		if len(labels) != 2 {
			return fmt.Errorf("expected 2 session legend entries, got %d: %v", len(labels), labels)
		}
		if labels[0] == labels[1] {
			return fmt.Errorf("both idle sessions rendered the same legend label %q", labels[0])
		}
		if labels[0] == "recent session" || labels[1] == "recent session" {
			return fmt.Errorf("legend still shows the generic \"recent session\" fallback: %v", labels)
		}
		return nil
	}
}
