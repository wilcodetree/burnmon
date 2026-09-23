package main

import "fmt"

// w3: the tool-call table's Path column and the files-read list must wrap
// a long, unbroken string (a deep Windows path, a WSL path with no
// spaces) instead of overflowing the drawer's 560px width into a
// horizontal scrollbar; vertical scroll is fine when content is taller.
// Reproduced/verified by opening the drawer on a real turn, then feeding
// it a synthetic TurnDetail with a genuinely unbreakable string (no
// spaces or slashes at all, so no natural line-break point exists) through
// window.__bmTurnResolve, the same resolver bmTurn's real async result
// goes through, and checking #turn_drawer's scrollWidth against its
// clientWidth.
//
// Args: "before" only captures and reports; anything else (the default,
// "after") also asserts.
func init() {
	checks["w3"] = func(hwnd uintptr, args []string) error {
		phase := "after"
		if len(args) > 0 {
			phase = args[0]
		}

		script := `(function(){
  var line = document.querySelector('.ticker-line');
  var sess = line.getAttribute('data-session');
  var turn = line.getAttribute('data-turn');
  line.click();
  var longNoBreak = 'abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz';
  var detail = {
    turn: parseInt(turn,10), session_id: sess, model: 'test', at: new Date().toISOString(),
    has_gap: false, fresh: 1, cache_write: 1, cache_read: 1, output: 1,
    findings: [],
    tool_calls: [{tool:'Read', path: longNoBreak, input_bytes: 100, result_bytes: 200}],
    files: [longNoBreak]
  };
  window.__bmTurnResolve(sess+':'+turn, detail);
  var d = document.getElementById('turn_drawer');
  return {scrollWidth: d.scrollWidth, clientWidth: d.clientWidth};
})()`
		var result struct {
			ScrollWidth int `json:"scrollWidth"`
			ClientWidth int `json:"clientWidth"`
		}
		if err := evalInto(script, &result); err != nil {
			return fmt.Errorf("open+inject: %w", err)
		}
		fmt.Printf("uicheck: turn_drawer scrollWidth=%d clientWidth=%d\n", result.ScrollWidth, result.ClientWidth)

		if _, err := screenshot(hwnd, "w3-"+phase+"-long-path"); err != nil {
			fmt.Printf("uicheck: screenshot failed (non-fatal, best effort only): %v\n", err)
		}

		// Close the drawer so a later check doesn't inherit it open.
		if _, err := evalRaw(`document.getElementById('turn_drawer_close').click()`); err != nil {
			return fmt.Errorf("close drawer: %w", err)
		}

		if phase == "before" {
			return nil
		}
		if result.ScrollWidth > result.ClientWidth {
			return fmt.Errorf("horizontal overflow: scrollWidth %d > clientWidth %d", result.ScrollWidth, result.ClientWidth)
		}
		return nil
	}
}
