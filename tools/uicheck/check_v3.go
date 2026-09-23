package main

import (
	"fmt"
	"strings"
	"time"
)

// v3: v0.3 V3-3's real-window check. C3's dev/business toggle flips the Now
// page in place; U2's startup fix means neither the vendor strip nor the
// live view ever shows the "only available ... not in a saved report" text
// once the window has actually finished its first poll (scripts\uicheck.ps1
// already waits for the first Collect to finish before running any check).
func init() {
	checks["v3"] = func(hwnd uintptr, args []string) error {
		// Startup state: dev mode (the compiled default, no burnmon.json on
		// this machine), first poll should already have landed
		// (scripts\uicheck.ps1 already waited for the first Collect log
		// line); a short settle here is just for a clean reference frame,
		// not for correctness.
		time.Sleep(500 * time.Millisecond)
		if _, err := screenshot(hwnd, "v3-startup-dev"); err != nil {
			fmt.Printf("uicheck: screenshot failed (non-fatal): %v\n", err)
		}

		var startup struct {
			NowEmptyHidden  bool   `json:"nowEmptyHidden"`
			NowEmptyText    string `json:"nowEmptyText"`
			VsEmptyHidden   bool   `json:"vsEmptyHidden"`
			VsEmptyText     string `json:"vsEmptyText"`
			ForecastNoteTxt string `json:"forecastNoteTxt"`
			ModeLabel       string `json:"modeLabel"`
		}
		startupScript := `(function(){
  var ne = document.getElementById('now_empty');
  var ve = document.getElementById('vs_empty');
  var fn = document.getElementById('now_forecast_note');
  return {
    nowEmptyHidden: ne ? ne.classList.contains('hidden') : null,
    nowEmptyText: ne ? ne.textContent : '',
    vsEmptyHidden: ve ? ve.classList.contains('hidden') : null,
    vsEmptyText: ve ? ve.textContent : '',
    forecastNoteTxt: fn ? fn.textContent : '',
    modeLabel: document.getElementById('btn_mode') ? document.getElementById('btn_mode').textContent : ''
  };
})()`
		if err := evalInto(startupScript, &startup); err != nil {
			return fmt.Errorf("read startup DOM state: %w", err)
		}
		fmt.Printf("uicheck: startup state = %+v\n", startup)
		if startup.ModeLabel != "Dev" {
			return fmt.Errorf("btn_mode label at startup = %q, want %q (dev is the compiled default)", startup.ModeLabel, "Dev")
		}
		for _, txt := range []string{startup.NowEmptyText, startup.VsEmptyText, startup.ForecastNoteTxt} {
			if strings.Contains(txt, "saved report") {
				return fmt.Errorf("U2 regression: %q still shows the saved-report text after the first Collect finished", txt)
			}
		}
		if !startup.VsEmptyHidden && startup.VsEmptyText == "" {
			return fmt.Errorf("vendor strip's vs_empty is visible with no text: header with no rows and no explanation (the original U2 bug)")
		}

		// Flip to business mode with a real click (not an eval-driven
		// .click(): SendInput, same as every other uicheck check).
		if err := clickSelector(hwnd, "#btn_mode"); err != nil {
			return fmt.Errorf("click #btn_mode: %w", err)
		}
		time.Sleep(1000 * time.Millisecond)
		if _, err := screenshot(hwnd, "v3-business"); err != nil {
			fmt.Printf("uicheck: screenshot failed (non-fatal): %v\n", err)
		}

		var business struct {
			ModeLabel    string `json:"modeLabel"`
			CardsHTML    string `json:"cardsHTML"`
			YAxisDisplay bool   `json:"yAxisDisplay"`
			CostHidden   bool   `json:"costHidden"`
		}
		businessScript := `(function(){
  var c = Object.values(Chart.instances||{}).filter(function(c){return c.canvas && c.canvas.id==='ch_now'})[0];
  var idx = c ? c.data.datasets.findIndex(function(d){return d.label==='Cost/min'}) : -1;
  return {
    modeLabel: document.getElementById('btn_mode') ? document.getElementById('btn_mode').textContent : '',
    cardsHTML: (document.getElementById('now_cards')||{}).innerHTML || '',
    yAxisDisplay: c ? !!c.options.scales.y.display : null,
    costHidden: (c && idx>=0) ? !!c.data.datasets[idx].hidden : null
  };
})()`
		if err := evalInto(businessScript, &business); err != nil {
			return fmt.Errorf("read business-mode DOM state: %w", err)
		}
		fmt.Printf("uicheck: business mode label=%q, y-axis display=%v, cost line hidden=%v\n",
			business.ModeLabel, business.YAxisDisplay, business.CostHidden)
		if business.ModeLabel != "Business" {
			return fmt.Errorf("btn_mode label after click = %q, want %q", business.ModeLabel, "Business")
		}
		if business.YAxisDisplay {
			return fmt.Errorf("token y-axis still displayed in business mode")
		}
		if business.CostHidden {
			return fmt.Errorf("cost line still hidden in business mode")
		}
		if strings.Contains(business.CardsHTML, "context window") {
			return fmt.Errorf("now_cards still shows dev-mode context text after switching to business mode: %s", business.CardsHTML)
		}

		// Flip back to dev, leaving the window in its default state.
		if err := clickSelector(hwnd, "#btn_mode"); err != nil {
			return fmt.Errorf("click #btn_mode (revert): %w", err)
		}
		time.Sleep(1000 * time.Millisecond)
		if _, err := screenshot(hwnd, "v3-back-to-dev"); err != nil {
			fmt.Printf("uicheck: screenshot failed (non-fatal): %v\n", err)
		}
		modeLabel, err := evalString(`document.getElementById('btn_mode').textContent`)
		if err != nil {
			return fmt.Errorf("read btn_mode after revert: %w", err)
		}
		if modeLabel != "Dev" {
			return fmt.Errorf("btn_mode label after reverting = %q, want %q", modeLabel, "Dev")
		}
		return nil
	}
}
