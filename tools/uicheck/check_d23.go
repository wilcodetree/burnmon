package main

import (
	"fmt"
	"strings"
	"time"
)

// d23: the Station, BurnMon Dev's secret screen
// (02_roadmap\2026-10-01_station_secret_screen.md, step 6). Fake mode feeds
// three sessions with stages (never live data), then real key presses:
// P opens the full overlay and its canvas draws, a Station click opens the
// turn popup on top, Esc closes the popup first and the Station on the
// next press; O replaces the whole burn zone (chart to turn ticker, all of
// it above System) with the plan view filling that zone, and it draws; O
// brings the zone back. Screenshots of both views.
const (
	vkP = 0x50
	vkO = 0x4F
)

// d23State is one read of the Station's DOM. Lit counts canvas pixels
// (sampled every 4th pixel each way) brighter than the deck background, so
// an empty or never-drawn canvas reads near 0.
type d23State struct {
	Overlay    bool `json:"overlay"`
	FullCanvas bool `json:"fullCanvas"`
	Panel      bool `json:"panel"`
	BarsShown  bool `json:"barsShown"`
	// TickerShown is the turn ticker, the burn zone's last panel above System.
	TickerShown bool    `json:"tickerShown"`
	AxisShown   bool    `json:"axisShown"`
	Lit         int     `json:"lit"`
	Hud         string  `json:"hud"`
	Popup       bool    `json:"popup"`
	PopupZ      float64 `json:"popupZ"`
	OverlayZ    float64 `json:"overlayZ"`
	PanelH      float64 `json:"panelH"`
	ZoneH       float64 `json:"zoneH"`
}

const d23ReadJS = `(function(){
  var ov = document.getElementById('stationOverlay');
  var box = document.getElementById('stationPanel');
  var cv = (ov && ov.classList.contains('on') ? ov.querySelector('canvas') : null) || (box ? box.querySelector('canvas') : null);
  var lit = 0;
  if(cv && cv.width && cv.height){
    var d = cv.getContext('2d').getImageData(0, 0, cv.width, cv.height).data;
    for(var y = 0; y < cv.height; y += 4) for(var x = 0; x < cv.width; x += 4){
      var i = (y * cv.width + x) * 4;
      if(d[i] + d[i + 1] + d[i + 2] > 90) lit++;
    }
  }
  var hud = document.querySelector('.bms-hud');
  var pop = document.getElementById('turnPopupOverlay');
  // Shown means it has a box: a child of a display:none parent keeps its
  // own computed display, so check the rendered size instead.
  function shown(id){ var el = document.getElementById(id); return !!el && el.getClientRects().length > 0; }
  return {
    overlay: !!ov && ov.classList.contains('on'), fullCanvas: !!(ov && ov.querySelector('canvas')),
    panel: !!box, barsShown: shown('burnBars'), tickerShown: shown('turnTicker'), axisShown: shown('burnAxis'), lit: lit,
    hud: hud ? hud.textContent : '', popup: !pop.classList.contains('hidden'),
    popupZ: parseFloat(getComputedStyle(pop).zIndex) || 0, overlayZ: parseFloat(getComputedStyle(ov).zIndex) || 0,
    panelH: box ? box.getBoundingClientRect().height : 0,
    zoneH: document.querySelector('.zone.burn').getBoundingClientRect().height
  };
})()`

const d23FakeJS = `(function(){
  var nowMs = Date.now(), ws = nowMs - 30 * 60000, chart = [];
  function iso(ms){ return new Date(ms).toISOString(); }
  for(var i = 0; i < 30; i++) chart.push({ at: iso(ws + i * 60000), by_session: {}, cost: 0 });
  function s(id, vendor, agent, stage, tool){
    return {session_id: id, vendor: vendor, agent: agent, surface: agent, model: '', project: 'proj-' + id, start: iso(nowMs - 600000),
      last_turn: iso(nowMs - 5000), turn_count: 4, context: 1000, context_window: 200000, tokens: 5000, cost: 0.1, cache_hit_ratio: 0,
      stage: stage, stage_since: iso(nowMs - 5000), stage_tool: tool};
  }
  var sessions = [s('fake-st-1', 'anthropic', 'claude-code', 'reading', 'Read'),
                  s('fake-st-2', 'openai', 'codex', 'running', 'exec'),
                  s('fake-st-3', 'anthropic', 'claude-code', 'waiting', '')];
  var snap = { now: nowMs, hist_gap_ms: 25000, sysmon: { Ts: iso(nowMs), pressure_score: 10, CPUPct: 10, Cores: [10, 10], MemUsedMB: 1000, MemTotalMB: 32000,
      SwapUsedMB: 0, SwapTotalMB: 1000, NetDownBps: 0, NetUpBps: 0, Disks: [], Wifi: { OK: true, Connected: false } },
    sysmon_history: [], process_groups_now: [], process_groups_history: [], vendor_strip: { rows: [] }, activity_heatmap: { rows: [] },
    todo: { enabled: false }, todo_tasks: { items: [] }, headline_today: 0,
    burn: { generated_at: iso(nowMs), running_window_seconds: 600, sessions: sessions, window_start: iso(ws), bucket_seconds: 60, chart: chart, turns: [] } };
  window.__bdevEnterFakeMode();
  window.__bdevPaintFake(snap);
  return true;
})()`

func d23Read() (d23State, error) {
	var s d23State
	err := evalInto(d23ReadJS, &s)
	return s, err
}

// d23Wait polls until ok holds or 4 s pass, returning the last read.
func d23Wait(ok func(d23State) bool) (d23State, bool, error) {
	var s d23State
	var err error
	for deadline := time.Now().Add(4 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		if s, err = d23Read(); err != nil {
			return s, false, err
		}
		if ok(s) {
			return s, true, nil
		}
	}
	return s, false, nil
}

func init() {
	checks["d23"] = func(hwnd uintptr, args []string) error {
		if err := ensureWindowSizeWH(hwnd, 1600, 900); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
		defer evalRaw(`(window.__bdevExitFakeMode && window.__bdevExitFakeMode())`)
		if _, err := evalRaw(d23FakeJS); err != nil {
			return fmt.Errorf("d23: fake paint: %w", err)
		}
		bringToFront(hwnd)
		// A real click grants the window input focus before the key presses
		// (d7's own finding).
		if err := clickSelectorAt(hwnd, ".logo", 0.5, 0.5); err != nil {
			fmt.Println("uicheck: d23: could not click .logo to establish focus:", err)
		}
		var errs []string
		step := func(name string, vk uint16, ok func(d23State) bool) d23State {
			if vk != 0 {
				if err := pressKey(vk); err != nil {
					errs = append(errs, fmt.Sprintf("%s: press: %v", name, err))
					return d23State{}
				}
			}
			s, good, err := d23Wait(ok)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: eval: %v", name, err))
			} else if !good {
				errs = append(errs, fmt.Sprintf("%s: got %+v", name, s))
			}
			fmt.Printf("uicheck: d23 %s: %+v\n", name, s)
			return s
		}

		step("P opens the full Station and it draws", vkP, func(s d23State) bool {
			return s.Overlay && s.FullCanvas && s.Lit > 500 && strings.Contains(s.Hud, "3 agents")
		})
		if _, err := screenshot(hwnd, "d23-station-full"); err != nil {
			return err
		}
		if _, err := evalRaw(`(window.__bdevStationClick('fake-st-1'), true)`); err != nil {
			errs = append(errs, fmt.Sprintf("station click: %v", err))
		}
		step("a Station click opens the turn popup above it", 0, func(s d23State) bool {
			return s.Popup && s.Overlay && s.PopupZ > s.OverlayZ
		})
		step("Esc closes the popup first, the Station stays", vkEscape, func(s d23State) bool {
			return !s.Popup && s.Overlay
		})
		step("Esc again closes the Station", vkEscape, func(s d23State) bool {
			return !s.Overlay && !s.FullCanvas && s.BarsShown
		})
		step("O swaps the chart for the plan view and it draws", vkO, func(s d23State) bool {
			return s.Panel && !s.BarsShown && !s.AxisShown && !s.TickerShown && s.PanelH >= s.ZoneH-4 && s.Lit > 500 && strings.Contains(s.Hud, "3 agents")
		})
		if _, err := screenshot(hwnd, "d23-station-panel"); err != nil {
			return err
		}
		step("O again brings the chart back", vkO, func(s d23State) bool {
			return !s.Panel && s.BarsShown && s.AxisShown && s.TickerShown && !s.Overlay
		})
		if len(errs) > 0 {
			return fmt.Errorf("d23: %d problem(s):\n%s", len(errs), strings.Join(errs, "\n"))
		}
		fmt.Println("uicheck: d23: Station opens on P and O, draws in both views, popup above it, Esc order popup then Station")
		return nil
	}
}
