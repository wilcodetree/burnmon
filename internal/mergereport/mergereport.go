// Package mergereport renders K5's report.html: a static page built entirely
// server-side from a merge.Merged value, so it opens offline with no script
// at all, network or otherwise.
package mergereport

import (
	"fmt"
	"html/template"
	"os"
	"strings"

	"burnmon/internal/merge"
)

const tplSource = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>BurnMon merge report</title>
<style>
  body { font-family: -apple-system, Segoe UI, Arial, sans-serif; margin: 2rem; color: #1a1a1a; background: #fff; }
  h1 { font-size: 1.4rem; }
  h2 { font-size: 1.1rem; margin-top: 2rem; }
  table { border-collapse: collapse; width: 100%; margin-top: 0.5rem; }
  th, td { border: 1px solid #ccc; padding: 0.35rem 0.6rem; text-align: right; font-variant-numeric: tabular-nums; }
  th:first-child, td:first-child { text-align: left; }
  th { background: #f2f2f2; }
  tr.total td { font-weight: bold; background: #f8f8f8; }
  .meta { color: #555; font-size: 0.9rem; }
</style>
</head>
<body>
<h1>BurnMon merge report</h1>
<p class="meta">Labels merged: {{range $i, $l := .Labels}}{{if $i}}, {{end}}{{$l}}{{end}}. Tokens (input + cache write + cache read + output + reasoning), no cap.</p>

<h2>Totals per client</h2>
<table>
<tr><th>Client</th>{{range .Labels}}<th>{{.}}</th>{{end}}<th>Total</th></tr>
{{range .ByClient}}<tr><td>{{.Client}}</td>{{$row := .}}{{range $.Labels}}<td>{{index $row.ByLabel .}}</td>{{end}}<td>{{.Total}}</td></tr>
{{end}}
</table>

<h2>Totals per vendor</h2>
<table>
<tr><th>Vendor</th>{{range .Labels}}<th>{{.}}</th>{{end}}<th>Total</th></tr>
{{range .ByVendor}}<tr><td>{{.Vendor}}</td>{{$row := .}}{{range $.Labels}}<td>{{index $row.ByLabel .}}</td>{{end}}<td>{{.Total}}</td></tr>
{{end}}
</table>

<h2>Totals per week</h2>
<table>
<tr><th>Week</th>{{range .Labels}}<th>{{.}}</th>{{end}}<th>Total</th></tr>
{{range .ByWeek}}<tr><td>{{.Week}}</td>{{$row := .}}{{range $.Labels}}<td>{{index $row.ByLabel .}}</td>{{end}}<td>{{.Total}}</td></tr>
{{end}}
</table>

</body>
</html>
`

var tpl = template.Must(template.New("report").Parse(tplSource))

// Render returns report.html's contents for m. Pure Go html/template, no
// embedded script, so the result opens offline in any browser with no
// network access at all.
func Render(m merge.Merged) (string, error) {
	var b strings.Builder
	if err := tpl.Execute(&b, m); err != nil {
		return "", fmt.Errorf("mergereport: render: %w", err)
	}
	return b.String(), nil
}

// Write renders m and writes it to path.
func Write(path string, m merge.Merged) error {
	out, err := Render(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), 0o644)
}
