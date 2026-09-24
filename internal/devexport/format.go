package devexport

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
)

// dataJSONDoc is data.json's exact shape: schema version, the 10s system
// timeline, every turn, the process-group timeline and the machine
// profile (plan, Export section), plus the derived aggregates and findings
// a consumer would otherwise have to recompute from the raw turns.
type dataJSONDoc struct {
	Schema      int    `json:"schema"`
	GeneratedAt string `json:"generated_at"`
	Since       string `json:"since"`
	Until       string `json:"until"`
	Redacted    bool   `json:"redacted"`

	Machine interface{} `json:"machine"`

	SystemTimeline       interface{} `json:"system_timeline"`
	ProcessGroupTimeline interface{} `json:"process_group_timeline"`

	Turns              []TurnRow           `json:"turns"`
	HarnessModelTotals []HarnessModelTotal `json:"harness_model_totals"`
	PressureEpisodes   []PressureEpisode   `json:"pressure_episodes"`
	Findings           interface{}         `json:"findings,omitempty"`
}

// BuildDataJSON renders b as data.json: the machine-readable counterpart
// to summary.md, everything data.json's own spec asks for plus the
// aggregates Assemble already derived, so a consumer never has to re-derive
// per-day or per-harness totals from the raw turn list itself.
func BuildDataJSON(b Bundle) ([]byte, error) {
	doc := dataJSONDoc{
		Schema: b.Schema, GeneratedAt: b.GeneratedAt.Format(rfc3339), Since: b.Since.Format(rfc3339), Until: b.Until.Format(rfc3339),
		Redacted: b.Redacted, Machine: b.Machine,
		SystemTimeline: b.SystemTimeline, ProcessGroupTimeline: b.ProcessGroupTimeline,
		Turns: b.Turns, HarnessModelTotals: b.HarnessModelTotals, PressureEpisodes: b.PressureEpisodes,
	}
	if len(b.Findings) > 0 {
		doc.Findings = b.Findings
	}
	return json.MarshalIndent(doc, "", " ")
}

const rfc3339 = "2006-01-02T15:04:05Z07:00"

// BuildDailyCSV renders b.Daily (date, harness, model, token kinds, cost:
// the plan's own fixed column list) as CSV.
func BuildDailyCSV(b Bundle) string {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"date", "harness", "model", "fresh", "cache_write", "cache_read", "output", "cost_usd"})
	for _, r := range b.Daily {
		_ = w.Write([]string{
			r.Date, r.Harness, r.Model,
			fmt.Sprintf("%d", r.Fresh), fmt.Sprintf("%d", r.CacheWrite), fmt.Sprintf("%d", r.CacheRead), fmt.Sprintf("%d", r.Output),
			fmt.Sprintf("%.4f", r.CostUSD),
		})
	}
	w.Flush()
	return buf.String()
}
