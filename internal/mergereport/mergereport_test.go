package mergereport

import (
	"strings"
	"testing"

	"burnmon/internal/merge"
)

func TestRender_NoNetworkScript(t *testing.T) {
	m := merge.Merged{
		Labels: []string{"dev-1", "dev-2"},
		ByClient: []merge.ClientTotal{
			{Client: "burnmon", ByLabel: merge.LabelTotals{"dev-1": 100, "dev-2": 50}, Total: 150},
		},
		ByVendor: []merge.VendorTotal{
			{Vendor: "anthropic", ByLabel: merge.LabelTotals{"dev-1": 100}, Total: 100},
		},
		ByWeek: []merge.WeekTotal{
			{Week: "2026-W39", ByLabel: merge.LabelTotals{"dev-1": 100}, Total: 100},
		},
	}
	out, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "<script") {
		t.Fatal("report.html contains a <script> tag, want none (offline, no chart)")
	}
	if strings.Contains(out, "http://") || strings.Contains(out, "https://") {
		t.Fatal("report.html references a network URL")
	}
	for _, want := range []string{"dev-1", "dev-2", "burnmon", "anthropic", "2026-W39", "150", "100"} {
		if !strings.Contains(out, want) {
			t.Fatalf("report.html missing %q:\n%s", want, out)
		}
	}
}
