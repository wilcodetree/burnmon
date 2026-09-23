// Package merge combines K4 export files into one merged.json plus a static,
// offline report.html (K5): one column per input label, totals per client,
// vendor and week, no cap on the number of files or labels merged.
package merge

import (
	"fmt"
	"sort"
	"time"

	"burnmon/internal/export"
)

// LabelTotals maps an export label to a summed token count.
type LabelTotals map[string]int64

// ClientTotal, VendorTotal and WeekTotal are one merged row: the group key,
// its per-label breakdown, and the sum across every label.
type ClientTotal struct {
	Client  string      `json:"client"`
	ByLabel LabelTotals `json:"by_label"`
	Total   int64       `json:"total"`
}

type VendorTotal struct {
	Vendor  string      `json:"vendor"`
	ByLabel LabelTotals `json:"by_label"`
	Total   int64       `json:"total"`
}

type WeekTotal struct {
	Week    string      `json:"week"` // ISO week, YYYY-Www
	ByLabel LabelTotals `json:"by_label"`
	Total   int64       `json:"total"`
}

// Merged is K5's merged.json: the labels seen, in the order their files were
// given, and the three breakdowns.
type Merged struct {
	Schema   int           `json:"schema"`
	Labels   []string      `json:"labels"`
	ByClient []ClientTotal `json:"by_client"`
	ByVendor []VendorTotal `json:"by_vendor"`
	ByWeek   []WeekTotal   `json:"by_week"`
}

func tokens(r export.Row) int64 {
	return r.Input + r.CacheWrite + r.CacheRead + r.Output + r.Reasoning
}

// isoWeek converts a Row.Day ("YYYY-MM-DD") into an ISO week key
// ("YYYY-Www"), matching internal/history's own bucketKey shape. An
// unparseable day (should not happen for a row this codebase produced) sorts
// under "unknown" rather than panicking, so one bad row does not fail the
// whole merge.
func isoWeek(day string) string {
	t, err := parseDay(day)
	if err != nil {
		return "unknown"
	}
	y, w := t.ISOWeek()
	return fmt.Sprintf("%04d-W%02d", y, w)
}

// Merge combines docs (already schema-checked by the caller) into Merged.
// docs and their labels must be the same length and in the same order; a
// label repeated across two files (two exports from the same machine with
// the same --label) simply sums into the same column, which is the expected
// behaviour rather than an error.
func Merge(docs []export.Doc) (Merged, error) {
	if len(docs) == 0 {
		return Merged{}, fmt.Errorf("merge: no files given")
	}

	schema := docs[0].Schema
	for _, d := range docs[1:] {
		if d.Schema != schema {
			return Merged{}, fmt.Errorf("merge: schema mismatch: file labelled %q has schema %d, file labelled %q has schema %d",
				docs[0].Label, schema, d.Label, d.Schema)
		}
	}

	var labels []string
	seenLabel := map[string]bool{}
	clientIdx := map[string]int{}
	var clients []ClientTotal
	vendorIdx := map[string]int{}
	var vendors []VendorTotal
	weekIdx := map[string]int{}
	var weeks []WeekTotal

	for _, d := range docs {
		if !seenLabel[d.Label] {
			seenLabel[d.Label] = true
			labels = append(labels, d.Label)
		}
		for _, r := range d.Rows {
			tok := tokens(r)

			client := r.Client
			if client == "" {
				client = "unassigned"
			}
			ci, ok := clientIdx[client]
			if !ok {
				ci = len(clients)
				clientIdx[client] = ci
				clients = append(clients, ClientTotal{Client: client, ByLabel: LabelTotals{}})
			}
			clients[ci].ByLabel[d.Label] += tok
			clients[ci].Total += tok

			vi, ok := vendorIdx[r.Vendor]
			if !ok {
				vi = len(vendors)
				vendorIdx[r.Vendor] = vi
				vendors = append(vendors, VendorTotal{Vendor: r.Vendor, ByLabel: LabelTotals{}})
			}
			vendors[vi].ByLabel[d.Label] += tok
			vendors[vi].Total += tok

			week := isoWeek(r.Day)
			wi, ok := weekIdx[week]
			if !ok {
				wi = len(weeks)
				weekIdx[week] = wi
				weeks = append(weeks, WeekTotal{Week: week, ByLabel: LabelTotals{}})
			}
			weeks[wi].ByLabel[d.Label] += tok
			weeks[wi].Total += tok
		}
	}

	sortClients(clients)
	sortVendors(vendors)
	sortWeeks(weeks)

	return Merged{
		Schema:   schema,
		Labels:   labels,
		ByClient: clients,
		ByVendor: vendors,
		ByWeek:   weeks,
	}, nil
}

func parseDay(day string) (time.Time, error) {
	return time.Parse("2006-01-02", day)
}

func sortClients(rows []ClientTotal) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Client < rows[j].Client })
}

func sortVendors(rows []VendorTotal) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Vendor < rows[j].Vendor })
}

func sortWeeks(rows []WeekTotal) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Week < rows[j].Week })
}
