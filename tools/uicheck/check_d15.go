package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// d15: WS2 follow-up (2026-09-28), one colour family per vendor - a
// uicheck case that drives paintTick with a synthetic burn snapshot (two
// Cowork sessions, two Claude Code sessions, one Copilot CLI session, never
// live data) via page.html's own __bdevEnterFakeMode/__bdevPaintFake test
// hooks, then proves: a session's burn-chart segment colour equals its
// session-card colour; every session of one vendor shares that vendor's
// live vendor-strip hue (read from the DOM, not hardcoded, so this tracks
// whatever HARNESS_HUE actually is); the two shades within each vendor
// differ by a real lightness step; and no session's hue drifts into
// another vendor's range. Also exercises item 3's legend label-collision
// fix (04_assets\...\2026-09-28_ws2_vendor_colour_families.md's own item
// 4): the two Cowork sessions share one project name on purpose.
func init() {
	checks["d15"] = func(hwnd uintptr, args []string) error {
		if err := ensureWindowSizeWH(hwnd, 1280, 860); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)

		script := `(function(){
  var nowMs = Date.now();
  function iso(ms){ return new Date(ms).toISOString(); }
  var sessions = [
    {session_id:'fake-cw-1', vendor:'cowork', agent:'cowork', surface:'cowork', model:'', project:'host-cwd', start:iso(nowMs-600000), last_turn:iso(nowMs-60000), turn_count:3, context:1000, context_window:200000, tokens:5000, cost:0.12, cache_hit_ratio:0},
    {session_id:'fake-cw-2', vendor:'cowork', agent:'cowork', surface:'cowork', model:'', project:'host-cwd', start:iso(nowMs-500000), last_turn:iso(nowMs-30000), turn_count:2, context:800, context_window:200000, tokens:4000, cost:0.10, cache_hit_ratio:0},
    {session_id:'fake-cc-1', vendor:'claude-code', agent:'claude-code', surface:'claude-code', model:'', project:'proj-alpha', start:iso(nowMs-400000), last_turn:iso(nowMs-20000), turn_count:5, context:1200, context_window:200000, tokens:7000, cost:0.20, cache_hit_ratio:0},
    {session_id:'fake-cc-2', vendor:'claude-code', agent:'claude-code', surface:'claude-code', model:'', project:'proj-beta', start:iso(nowMs-300000), last_turn:iso(nowMs-10000), turn_count:4, context:900, context_window:200000, tokens:6000, cost:0.15, cache_hit_ratio:0},
    {session_id:'fake-cli-1', vendor:'copilot-cli', agent:'copilot-cli', surface:'copilot-cli', model:'', project:'proj-gamma', start:iso(nowMs-200000), last_turn:iso(nowMs-5000), turn_count:2, context:500, context_window:200000, tokens:3000, cost:0.05, cache_hit_ratio:0}
  ];
  var bySession = {};
  sessions.forEach(function(s, i){ bySession[s.session_id] = {fresh: 1000+i*100, cache_write:0, cache_read:0, output:200}; });
  var snap = {
    now: nowMs,
    sysmon_history: [], process_groups_now: [], process_groups_history: [],
    vendor_strip: { rows: [
      {agent:'cowork', agent_label:'Cowork', today:0, week:0, month:0},
      {agent:'claude-code', agent_label:'Claude Code', today:0, week:0, month:0},
      {agent:'copilot-cli', agent_label:'Copilot CLI', today:0, week:0, month:0}
    ] },
    activity_heatmap: {rows: []},
    todo: {enabled:false}, todo_tasks: {items: []}, headline_today: 0,
    burn: {
      generated_at: iso(nowMs), running_window_seconds: 1800,
      sessions: sessions, window_start: iso(nowMs-1800000), bucket_seconds: 60,
      chart: [{at: iso(nowMs), by_session: bySession, cost: 0}],
      turns: []
    }
  };
  window.__bdevEnterFakeMode();
  window.__bdevPaintFake(snap);

  var out = { chart:{}, card:{}, vendorBase:{}, legend:[] };
  sessions.forEach(function(s){
    var seg = document.querySelector('#burnBars .bar i[data-session="' + s.session_id + '"]');
    out.chart[s.session_id] = seg ? getComputedStyle(seg).backgroundColor : null;
    var fill = document.querySelector('#sessionCards .card[data-session="' + s.session_id + '"] .ctxbar i');
    out.card[s.session_id] = fill ? getComputedStyle(fill).backgroundColor : null;
  });
  ['cowork', 'claude-code', 'copilot-cli'].forEach(function(agent){
    var dot = document.querySelector('#vendorStrip tr[data-agent="' + agent + '"] .dot');
    out.vendorBase[agent] = dot ? getComputedStyle(dot).backgroundColor : null;
  });
  document.querySelectorAll('#burnLegend .legenditem').forEach(function(el){ out.legend.push(el.textContent); });
  return out;
})()`

		// Deferred unconditionally, not after the eval/screenshot calls
		// below: a JS exception partway through script (after
		// __bdevEnterFakeMode already ran) would otherwise return early and
		// leave the real page's own tick timer stopped forever. The guard
		// inside is a no-op if fake mode was never actually entered.
		defer evalRaw(`(window.__bdevExitFakeMode && window.__bdevExitFakeMode())`)

		var out struct {
			Chart      map[string]string `json:"chart"`
			Card       map[string]string `json:"card"`
			VendorBase map[string]string `json:"vendorBase"`
			Legend     []string          `json:"legend"`
		}
		if err := evalInto(script, &out); err != nil {
			return fmt.Errorf("d15: eval fake-session paint: %w", err)
		}
		// __bdevPaintFake updates the DOM synchronously, but WebView2's own
		// compositor can lag a frame or two behind before BitBlt's screen
		// capture actually sees it (found this session: without this wait,
		// the screenshot still showed the previous real-data frame even
		// though evalInto's own DOM reads above already reflected the fake
		// one) - give it a moment to actually paint before capturing.
		time.Sleep(300 * time.Millisecond)
		if _, err := screenshot(hwnd, "d15-vendor-colours"); err != nil {
			return err
		}

		vendorOf := map[string]string{
			"fake-cw-1": "cowork", "fake-cw-2": "cowork",
			"fake-cc-1": "claude-code", "fake-cc-2": "claude-code",
			"fake-cli-1": "copilot-cli",
		}
		var errs []string

		// 1. chart colour == card colour, for every session.
		for sid := range vendorOf {
			if out.Chart[sid] == "" || out.Card[sid] == "" {
				errs = append(errs, fmt.Sprintf("%s: missing chart (%q) or card (%q) colour", sid, out.Chart[sid], out.Card[sid]))
				continue
			}
			if out.Chart[sid] != out.Card[sid] {
				errs = append(errs, fmt.Sprintf("%s: chart colour %s != card colour %s", sid, out.Chart[sid], out.Card[sid]))
			}
		}

		// 2. every session's hue sits within a few degrees of its own
		// vendor's live vendor-strip hue, and clear of the other two
		// vendors present in this run.
		vendorHue := map[string]float64{}
		for agent, rgb := range out.VendorBase {
			h, _, _, err := parseRGBHSL(rgb)
			if err != nil {
				errs = append(errs, fmt.Sprintf("vendor strip %s: %v", agent, err))
				continue
			}
			vendorHue[agent] = h
		}
		const hueTolerance = 6.0   // "within a few degrees" of its own vendor
		const hueSeparation = 20.0 // clear of any other vendor present
		sessionHue := map[string]float64{}
		for sid, agent := range vendorOf {
			rgb := out.Chart[sid]
			if rgb == "" {
				continue
			}
			h, _, _, err := parseRGBHSL(rgb)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", sid, err))
				continue
			}
			sessionHue[sid] = h
			own, ok := vendorHue[agent]
			if !ok {
				continue
			}
			if d := hueDelta(h, own); d > hueTolerance {
				errs = append(errs, fmt.Sprintf("%s (%s): hue %.1f is %.1f deg from its own vendor's hue %.1f, want <=%.1f", sid, agent, h, d, own, hueTolerance))
			}
			for otherAgent, otherHue := range vendorHue {
				if otherAgent == agent {
					continue
				}
				if d := hueDelta(h, otherHue); d < hueSeparation {
					errs = append(errs, fmt.Sprintf("%s (%s): hue %.1f is only %.1f deg from %s's hue %.1f, want >=%.1f", sid, agent, h, d, otherAgent, otherHue, hueSeparation))
				}
			}
		}

		// 3. the two shades within each vendor differ by a real lightness
		// step (report it), same hue.
		pairs := [][2]string{{"fake-cw-1", "fake-cw-2"}, {"fake-cc-1", "fake-cc-2"}}
		for _, pair := range pairs {
			h1, _, l1, err1 := parseRGBHSL(out.Chart[pair[0]])
			h2, _, l2, err2 := parseRGBHSL(out.Chart[pair[1]])
			if err1 != nil || err2 != nil {
				errs = append(errs, fmt.Sprintf("%s/%s: could not parse for shade check", pair[0], pair[1]))
				continue
			}
			step := math.Abs(l1 - l2)
			fmt.Printf("uicheck: d15: %s/%s lightness step %.1f pp (L %.2f vs %.2f), hue delta %.1f deg\n", pair[0], pair[1], step*100, l1, l2, hueDelta(h1, h2))
			if step < 0.10 {
				errs = append(errs, fmt.Sprintf("%s/%s: lightness step only %.1f pp, want a clearly visible step (>=10pp)", pair[0], pair[1], step*100))
			}
		}

		// 4. item 4: the two same-project Cowork legend entries must not
		// render identical text.
		seen := map[string]int{}
		for _, l := range out.Legend {
			seen[l]++
		}
		for l, n := range seen {
			if n > 1 {
				errs = append(errs, fmt.Sprintf("legend: %d entries render identical text %q, want a session id appended on collision", n, l))
			}
		}

		if len(errs) > 0 {
			return fmt.Errorf("d15: %d problem(s):\n%s", len(errs), strings.Join(errs, "\n"))
		}
		fmt.Println("uicheck: d15: vendor colour families check clean (chart==card, hue held per vendor, shades separated, legend collision resolved)")
		return nil
	}
}

// parseRGBHSL parses a CSS "rgb(r, g, b)" / "rgba(r, g, b, a)" string (what
// getComputedStyle returns for a colour this page ever sets) into hue
// (degrees), saturation and lightness (both 0..1).
func parseRGBHSL(s string) (h, sat, l float64, err error) {
	s = strings.TrimSpace(s)
	open := strings.Index(s, "(")
	closeIdx := strings.Index(s, ")")
	if !strings.HasPrefix(s, "rgb") || open < 0 || closeIdx < 0 || closeIdx < open {
		return 0, 0, 0, fmt.Errorf("not an rgb() colour: %q", s)
	}
	parts := strings.Split(s[open+1:closeIdx], ",")
	if len(parts) < 3 {
		return 0, 0, 0, fmt.Errorf("not enough components in %q", s)
	}
	var rgb [3]float64
	for i := 0; i < 3; i++ {
		v, perr := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
		if perr != nil {
			return 0, 0, 0, fmt.Errorf("bad component %q in %q: %w", parts[i], s, perr)
		}
		rgb[i] = v / 255
	}
	r, g, b := rgb[0], rgb[1], rgb[2]
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l = (max + min) / 2
	if max == min {
		return 0, 0, l, nil // grey: hue undefined
	}
	d := max - min
	if l > 0.5 {
		sat = d / (2 - max - min)
	} else {
		sat = d / (max + min)
	}
	switch max {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, sat, l, nil
}

// hueDelta is the shortest distance between two hues on the 360-degree
// wheel (e.g. 350 and 5 are 15 apart, not 345).
func hueDelta(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > 180 {
		d = 360 - d
	}
	return d
}
