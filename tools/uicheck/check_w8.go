package main

import "fmt"

// w8: the right (cost) axis is hidden while the cost series is hidden
// (the default), and appears once it is shown, driven by the legend
// toggle. Reproduced/verified by toggling the cost dataset's visibility
// through the exact same Chart.js API (setDatasetVisibility) the legend's
// own built-in click handler calls, then reading the real chart's own
// cost scale visibility back.
//
// Args: "before" only captures and reports; anything else (the default,
// "after") also asserts.
func init() {
	checks["w8"] = func(hwnd uintptr, args []string) error {
		phase := "after"
		if len(args) > 0 {
			phase = args[0]
		}

		getState := func() (hiddenByDefault bool, costVisibleAfterShow bool, costHiddenAfterHide bool, err error) {
			script := `(function(){
  var c = Object.values(Chart.instances).filter(function(c){return c.canvas.id==='ch_now'})[0];
  var idx = c.data.datasets.findIndex(function(d){return d.label==='Cost/min'});
  var out = {};
  out.defaultVisible = c.isDatasetVisible(idx);
  out.defaultAxisVisible = c.scales.cost.axis && c.scales.cost.width > 0;
  c.setDatasetVisibility(idx, true);
  c.update();
  out.shownAxisVisible = c.scales.cost.width > 0;
  c.setDatasetVisibility(idx, false);
  c.update();
  out.hiddenAxisVisible = c.scales.cost.width > 0;
  return out;
})()`
			var r struct {
				DefaultVisible     bool `json:"defaultVisible"`
				DefaultAxisVisible bool `json:"defaultAxisVisible"`
				ShownAxisVisible   bool `json:"shownAxisVisible"`
				HiddenAxisVisible  bool `json:"hiddenAxisVisible"`
			}
			if e := evalInto(script, &r); e != nil {
				return false, false, false, e
			}
			fmt.Printf("uicheck: cost dataset default-visible=%v, axis default-visible=%v, axis-when-shown=%v, axis-when-hidden=%v\n",
				r.DefaultVisible, r.DefaultAxisVisible, r.ShownAxisVisible, r.HiddenAxisVisible)
			return r.DefaultAxisVisible, r.ShownAxisVisible, r.HiddenAxisVisible, nil
		}

		defaultAxisVisible, shownAxisVisible, hiddenAxisVisible, err := getState()
		if err != nil {
			return fmt.Errorf("toggle cost visibility: %w", err)
		}

		if _, err := screenshot(hwnd, "w8-"+phase+"-cost-axis"); err != nil {
			fmt.Printf("uicheck: screenshot failed (non-fatal, best effort only): %v\n", err)
		}

		if phase == "before" {
			return nil
		}
		if defaultAxisVisible {
			return fmt.Errorf("cost axis is visible by default (cost series is hidden by default)")
		}
		if !shownAxisVisible {
			return fmt.Errorf("cost axis stayed hidden after showing the cost series")
		}
		if hiddenAxisVisible {
			return fmt.Errorf("cost axis stayed visible after hiding the cost series again")
		}
		return nil
	}
}
