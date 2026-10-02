//go:build windows

package main

import (
	"strings"
	"testing"
)

// TestAssemblePageSplicesStation: page.html carries the marker exactly once
// and the assembled page carries both Station scripts in its place.
func TestAssemblePageSplicesStation(t *testing.T) {
	if n := strings.Count(pageHTML, stationMarker); n != 1 {
		t.Fatalf("page.html has %d %s markers, want 1", n, stationMarker)
	}
	page := assemblePage()
	if strings.Contains(page, stationMarker) {
		t.Fatal("marker left in the assembled page")
	}
	for _, want := range []string{"window.BM_STATION_ATLAS", "window.BMStation"} {
		if !strings.Contains(page, want) {
			t.Fatalf("assembled page lacks %s", want)
		}
	}
	if strings.Index(page, "window.BM_STATION_ATLAS") > strings.Index(page, "window.BMStation =") {
		t.Fatal("atlas must load before station.js")
	}
}
