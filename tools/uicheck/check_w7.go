package main

import "fmt"

// w7: every minute on the Now chart's X axis gets its own time label, none
// skipped, at 11px; labels rotate 45 degrees only once the chart is
// narrower than 36px per minute (this app's fixed window is 1280px wide,
// 30 buckets, so ~42px/minute: this check's real window is always the
// unrotated case; the rotation math itself is exercised directly rather
// than by resizing the real window). Reproduced/verified against the real
// running chart's own generated tick labels (Chart.js's own tick array,
// not the raw bucket count), checking every one is non-empty.
//
// Args: "before" only captures and reports; anything else (the default,
// "after") also asserts.
func init() {
	checks["w7"] = func(hwnd uintptr, args []string) error {
		phase := "after"
		if len(args) > 0 {
			phase = args[0]
		}

		script := `(function(){
  var c = Object.values(Chart.instances).filter(function(c){return c.canvas.id==='ch_now'})[0];
  var scale = c.scales.x;
  var labels = scale.ticks.map(function(t, i){ return scale.options.ticks.callback.call(scale, t.value, i); });
  return {labels: labels, fontSize: scale.options.ticks.font.size, maxRotation: scale.options.ticks.maxRotation};
})()`
		var result struct {
			Labels      []string `json:"labels"`
			FontSize    int      `json:"fontSize"`
			MaxRotation int      `json:"maxRotation"`
		}
		if err := evalInto(script, &result); err != nil {
			return fmt.Errorf("read x-axis ticks: %w", err)
		}
		fmt.Printf("uicheck: x-axis labels=%v fontSize=%d maxRotation=%d\n", result.Labels, result.FontSize, result.MaxRotation)

		if _, err := screenshot(hwnd, "w7-"+phase+"-x-axis"); err != nil {
			fmt.Printf("uicheck: screenshot failed (non-fatal, best effort only): %v\n", err)
		}

		if phase == "before" {
			return nil
		}

		empty := 0
		for _, l := range result.Labels {
			if l == "" {
				empty++
			}
		}
		if empty > 0 {
			return fmt.Errorf("%d of %d x-axis labels are empty (skipped), want none: %v", empty, len(result.Labels), result.Labels)
		}
		if result.FontSize != 11 {
			return fmt.Errorf("x-axis tick font size is %d, want 11", result.FontSize)
		}
		return nil
	}
}
