package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// u5: v0.3 V3-3c's real-window check for U5's braille history chart
// (02_roadmap\2026-09-23_v0.3_monitor_view_patch.md). Self-managed like v3b:
// it needs to control burnmon.json before launch so it starts straight into
// dev-mode monitor view. Screenshots and eval-only DOM assertions both run;
// per house rule (never a proxy check alone), a screenshot is always taken,
// but the eval assertions are the check's real pass/fail, since a locked
// console session (documented in SESSION_LOG.md against check_v3b.go) makes
// BitBlt capture the lock screen instead of the real window on this laptop
// some of the time.
//
// State is read entirely from the DOM (querySelector/getComputedStyle),
// never from a bare `window.someGlobal` the page's own inline script set: an
// earlier version of this check read window.LAST_NOW_SNAP.sessions.length
// and it was always 0 even when .mt-session-box/.mt-chart-legend-row
// reliably showed 4 real running sessions, because the dev eval channel
// (cmd\burnmon\uicheck_devserver.go) runs scripts in an isolated JS world:
// it shares the page's DOM but not its plain JS globals. SessionBoxes below
// is the DOM-based replacement.
func init() {
	checks["u5"] = checkU5
	selfManaged["u5"] = true
}

const u5CfgPath = `..\..\burnmon.json`

type u5ChartState struct {
	Title          string `json:"title"`
	LegendRows     int    `json:"legendRows"`
	DotChars       int    `json:"dotChars"`
	NonBlankDots   int    `json:"nonBlankDots"`
	DotRows        int    `json:"dotRows"`
	MinuteLabel    string `json:"minuteLabel"`
	SessionBoxes   int    `json:"sessionBoxes"`
	BoxWidthsEqual bool   `json:"boxWidthsEqual"`
	FontFamily     string `json:"fontFamily"`
	VendorFontPx   string `json:"vendorFontPx"`
	PeakRowIndex   int    `json:"peakRowIndex"`
	PeakRowCount   int    `json:"peakRowCount"`
}

func u5ReadState() (u5ChartState, error) {
	var s u5ChartState
	script := `(function(){
  var panel = document.querySelector('.mt-chart-panel');
  var title = panel ? panel.querySelector('.mt-chart-title') : null;
  var rows = panel ? panel.querySelectorAll('.mt-chart-legend-row') : [];
  var pre = panel ? panel.querySelector('.mt-chart-dots') : null;
  var dotText = pre ? pre.textContent : '';
  var dotRows = dotText ? dotText.split('\n').length : 0;
  var nonBlank = 0;
  for (var i=0;i<dotText.length;i++){ if(dotText[i] !== ' ' && dotText[i] !== '\n') nonBlank++; }
  var minutes = panel ? panel.querySelector('.mt-chart-minutes') : null;
  var boxes = document.querySelectorAll('.mt-session-box');
  var widths = {};
  boxes.forEach(function(b){
    var lines = b.textContent.split('\n');
    widths[lines[0] ? lines[0].length : 0] = true;
  });
  var peakIdx = -1, peakCount = 0;
  rows.forEach(function(r, idx){
    var p = r.querySelector('.mt-chart-peak');
    if(p && p.textContent.trim() !== ''){ peakIdx = idx; peakCount++; }
  });
  return {
    title: title ? title.textContent : '',
    legendRows: rows.length,
    dotChars: dotText.replace(/\n/g,'').length,
    nonBlankDots: nonBlank,
    dotRows: dotRows,
    minuteLabel: minutes ? minutes.textContent : '',
    sessionBoxes: boxes.length,
    boxWidthsEqual: Object.keys(widths).length <= 1,
    fontFamily: getComputedStyle(document.getElementById('monitor_text_view')).fontFamily,
    vendorFontPx: getComputedStyle(document.getElementById('mt_vendorstrip')).fontSize,
    peakRowIndex: peakIdx,
    peakRowCount: peakCount
  };
})()`
	if err := evalInto(script, &s); err != nil {
		return s, err
	}
	return s, nil
}

func checkU5(_ uintptr, _ []string) error {
	if h := findBurnmonWindowFast(); h != 0 && isRealBurnmonWindow(h) {
		return fmt.Errorf("burnmon.exe is already running (window %q); close it first so this check can control burnmon.json before launch", windowTitleText(h))
	}
	if _, err := os.Stat(u5CfgPath); err == nil {
		return fmt.Errorf("%s already exists; this check writes and removes its own, move any real one aside first", u5CfgPath)
	}
	if err := os.WriteFile(u5CfgPath, []byte("{\n  \"view\": \"monitor\"\n}\n"), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", u5CfgPath, err)
	}
	defer os.Remove(u5CfgPath)

	hwnd, cmd, err := v3bLaunch()
	if err != nil {
		return err
	}
	defer v3bKill(cmd)

	// Give bmLive(10)'s first poll (2s cadence) time to land and
	// renderMonitorText time to run before reading DOM state.
	time.Sleep(3 * time.Second)

	// Poll up to 90s for at least 2 running sessions (spec: "at least two
	// running sessions"), so the screenshot and DOM read reflect that case
	// when the real trail on this laptop provides it; reports the actual
	// count either way rather than failing silently on fewer.
	deadline := time.Now().Add(90 * time.Second)
	var state u5ChartState
	for time.Now().Before(deadline) {
		state, err = u5ReadState()
		if err != nil {
			return fmt.Errorf("read chart state: %w", err)
		}
		if state.SessionBoxes >= 2 {
			break
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Printf("uicheck: u5 chart state = %+v\n", state)

	v3bShot(hwnd, "u5-chart-monitor-dev")

	if state.Title != "live burn, newest right" {
		return fmt.Errorf("chart title = %q, want \"live burn, newest right\"", state.Title)
	}
	if state.SessionBoxes == 0 {
		return fmt.Errorf("no running sessions found on this laptop at check time; the chart cannot be visually proven against 0 series. Re-run with at least one real coding session active")
	}
	// The chart legend has one row per session that landed an event
	// anywhere in the 30-minute chart window (live.go's buildChart, unchanged
	// by this patch); a session card only shows while it is still "running"
	// (cfg.RunningWindowSeconds()). A session that went quiet a few minutes
	// ago can still hold a chart/legend row with no matching card, so legend
	// rows is always >= cards, never required to equal it.
	if state.LegendRows < state.SessionBoxes {
		return fmt.Errorf("legend rows = %d, session boxes = %d, want at least one legend row per running session", state.LegendRows, state.SessionBoxes)
	}
	if state.PeakRowCount != 1 || state.PeakRowIndex != state.LegendRows-1 {
		return fmt.Errorf("\"scaled to peak\" appeared on %d row(s) at index %d, want exactly 1, on the last row (index %d)", state.PeakRowCount, state.PeakRowIndex, state.LegendRows-1)
	}
	if state.NonBlankDots == 0 {
		return fmt.Errorf("braille chart drew zero non-blank dots with %d running session(s); expected at least some burn in the window", state.SessionBoxes)
	}
	if state.DotRows < 4 {
		return fmt.Errorf("braille chart has only %d row(s), want a real multi-row grid (~35%% of window height)", state.DotRows)
	}
	if strings.TrimSpace(state.MinuteLabel) == "" {
		return fmt.Errorf("no minute labels rendered under the chart")
	}
	if !state.BoxWidthsEqual {
		return fmt.Errorf("session box top-border widths are not all equal, want one shared width across the grid")
	}
	if !strings.Contains(state.FontFamily, "Cascadia Mono") {
		return fmt.Errorf("monitor_text_view font-family = %q, want Cascadia Mono first in the stack", state.FontFamily)
	}
	if state.SessionBoxes < 2 {
		fmt.Printf("uicheck: WARNING only %d running session(s) at check time (wanted >= 2 to match the U5 spec's own screenshot bar); chart/legend/session-box structure is proven above but the multi-series overlay is not visually exercised\n", state.SessionBoxes)
	}
	return nil
}
