package main

import (
	"fmt"
	"time"
)

// w5: legend order is vendor, then client type (surface), then model,
// then project, alphabetical inside each level; the cost item stays last.
// Reproduced/verified by overriding window.bmLive with four sessions
// whose vendor/surface/model/project fields are deliberately out of
// alphabetical order in the snapshot's own Sessions list, waiting for a
// real poll, then reading the actual Chart.js dataset order back.
//
// Args: "before" only captures and reports; anything else (the default,
// "after") also asserts.
func init() {
	checks["w5"] = func(hwnd uintptr, args []string) error {
		phase := "after"
		if len(args) > 0 {
			phase = args[0]
		}

		overrideScript := `(function(){
  var mk = function(id, vendor, surface, model, project){
    return {session_id:id, vendor:vendor, agent:vendor, surface:surface, model:model, project:project};
  };
  var sessions = [
    mk('w5sessZZZZ1', 'zvendor', 'cli', 'zmodel', 'zproj'),
    mk('w5sessAAAA2', 'avendor', 'desktop', 'amodel', 'aproj'),
    mk('w5sessAAAA3', 'avendor', 'cli', 'zmodel', 'zproj'),
    mk('w5sessAAAA4', 'avendor', 'cli', 'amodel', 'zproj')
  ];
  var bySess = {};
  sessions.forEach(function(s){ bySess[s.session_id] = {fresh:1,cache_write:1,cache_read:1,output:1}; });
  window.bmLive = function(){
    return Promise.resolve({
      chart: [{at: new Date().toISOString(), by_session: bySess, cost: 0.01}],
      sessions: sessions,
      turns: []
    });
  };
  return true;
})()`
		if _, err := evalRaw(overrideScript); err != nil {
			return fmt.Errorf("override bmLive: %w", err)
		}
		time.Sleep(5 * time.Second)

		idsScript := `(function(){
  var c = Object.values(Chart.instances).filter(function(c){return c.canvas.id==='ch_now'})[0];
  return c.data.datasets.map(function(d){return d.sessionId || d.label});
})()`
		var order []string
		if err := evalInto(idsScript, &order); err != nil {
			return fmt.Errorf("read dataset order: %w", err)
		}
		fmt.Printf("uicheck: dataset order = %v\n", order)

		if _, err := screenshot(hwnd, "w5-"+phase+"-legend-order"); err != nil {
			fmt.Printf("uicheck: screenshot failed (non-fatal, best effort only): %v\n", err)
		}

		if phase == "before" {
			return nil
		}

		// avendor/cli/amodel < avendor/cli/zmodel < avendor/desktop/* < zvendor/*, cost last
		want := []string{"w5sessAAAA4", "w5sessAAAA3", "w5sessAAAA2", "w5sessZZZZ1", "Cost/min"}
		if len(order) != len(want) {
			return fmt.Errorf("expected %d entries, got %d: %v", len(want), len(order), order)
		}
		for i := range want {
			if order[i] != want[i] {
				return fmt.Errorf("legend order wrong at position %d: got %v, want %v", i, order, want)
			}
		}
		return nil
	}
}
