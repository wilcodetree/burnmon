package merge

import (
	"testing"

	"burnmon/internal/export"
)

func TestMerge_RefusesSchemaMismatch(t *testing.T) {
	docs := []export.Doc{
		{Schema: 1, Label: "dev-1"},
		{Schema: 2, Label: "dev-2"},
	}
	if _, err := Merge(docs); err == nil {
		t.Fatal("Merge with mismatched schema returned nil error, want one")
	}
}

func TestMerge_RefusesEmpty(t *testing.T) {
	if _, err := Merge(nil); err == nil {
		t.Fatal("Merge with no files returned nil error, want one")
	}
}

func TestMerge_OneColumnPerLabelTotalsPerClientVendorWeek(t *testing.T) {
	docs := []export.Doc{
		{
			Schema: export.Schema, Label: "dev-1",
			Rows: []export.Row{
				{Day: "2026-09-21", Client: "burnmon", Vendor: "anthropic", Input: 100, Output: 10},
				{Day: "2026-09-22", Client: "burnmon", Vendor: "anthropic", Input: 50, Output: 5},
			},
		},
		{
			Schema: export.Schema, Label: "dev-2",
			Rows: []export.Row{
				{Day: "2026-09-21", Client: "siteoffice", Vendor: "openai", Input: 200, Output: 20},
			},
		},
	}
	m, err := Merge(docs)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Labels) != 2 || m.Labels[0] != "dev-1" || m.Labels[1] != "dev-2" {
		t.Fatalf("Labels = %v, want [dev-1 dev-2] in file order", m.Labels)
	}

	var burnmon *ClientTotal
	for i := range m.ByClient {
		if m.ByClient[i].Client == "burnmon" {
			burnmon = &m.ByClient[i]
		}
	}
	if burnmon == nil {
		t.Fatal("no burnmon client row")
	}
	if burnmon.ByLabel["dev-1"] != 165 || burnmon.Total != 165 {
		t.Fatalf("burnmon dev-1 total = %d, want 165 (100+10+50+5)", burnmon.ByLabel["dev-1"])
	}
	if burnmon.ByLabel["dev-2"] != 0 {
		t.Fatalf("burnmon dev-2 total = %d, want 0", burnmon.ByLabel["dev-2"])
	}

	var anthropic *VendorTotal
	for i := range m.ByVendor {
		if m.ByVendor[i].Vendor == "anthropic" {
			anthropic = &m.ByVendor[i]
		}
	}
	if anthropic == nil || anthropic.Total != 165 {
		t.Fatalf("anthropic vendor total = %+v, want 165", anthropic)
	}

	if len(m.ByWeek) != 1 {
		t.Fatalf("got %d week rows, want 1 (2026-09-21 and 2026-09-22 are the same ISO week)", len(m.ByWeek))
	}
	if m.ByWeek[0].Total != 385 {
		t.Fatalf("week total = %d, want 385 (all three rows: 110+55+220)", m.ByWeek[0].Total)
	}
}
