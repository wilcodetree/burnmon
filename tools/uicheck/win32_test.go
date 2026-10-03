//go:build windows

package main

import "testing"

// capToScreen is how ensureWindowSizeWH fits a requested window onto the
// monitor it sits on: the window starts at (originX, originY) and may reach
// the monitor's right and bottom edge, no further.
func TestCapToScreen(t *testing.T) {
	primary := rect{0, 0, 3440, 1440}
	laptop := rect{-3200, -225, -1600, 775} // 1600x1000 screen left of the primary
	cases := []struct {
		name         string
		w, h         int32
		ox, oy       int32
		scr          rect
		wantW, wantH int32
		wantCapped   bool
	}{
		{"fits the primary", 2000, 1200, 100, 100, primary, 2000, 1200, false},
		{"exactly fills what is left", 3340, 1340, 100, 100, primary, 3340, 1340, false},
		{"too wide", 3400, 1200, 100, 100, primary, 3340, 1200, true},
		{"too tall", 2000, 1400, 100, 100, primary, 2000, 1340, true},
		{"both", 5000, 4000, 100, 100, primary, 3340, 1340, true},
		{"origin on a second monitor", 1700, 900, -3100, -125, laptop, 1500, 900, true},
		{"fits the second monitor", 1400, 800, -3100, -125, laptop, 1400, 800, false},
	}
	for _, c := range cases {
		w, h, capped := capToScreen(c.w, c.h, c.ox, c.oy, c.scr)
		if w != c.wantW || h != c.wantH || capped != c.wantCapped {
			t.Errorf("%s: capToScreen(%d,%d) = %d,%d,%v, want %d,%d,%v", c.name, c.w, c.h, w, h, capped, c.wantW, c.wantH, c.wantCapped)
		}
	}
}

func TestParseOrigin(t *testing.T) {
	cases := []struct {
		in     string
		x, y   int32
		wantOK bool
	}{
		{"", 100, 100, false},
		{"-3100,-125", -3100, -125, true},
		{" 50 , 60 ", 50, 60, true},
		{"nonsense", 100, 100, false},
		{"1,2,3", 100, 100, false},
	}
	for _, c := range cases {
		x, y, ok := parseOrigin(c.in)
		if x != c.x || y != c.y || ok != c.wantOK {
			t.Errorf("parseOrigin(%q) = %d,%d,%v, want %d,%d,%v", c.in, x, y, ok, c.x, c.y, c.wantOK)
		}
	}
}
