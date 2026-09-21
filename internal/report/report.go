// Package report renders the dashboard: the same template the Cowork
// "Claude Cost (User)" artifact uses, with the dataset injected at the
// __USAGE_DATA__ placeholder. Chart.js is vendored inside the template, so
// the page needs no network access at all.
package report

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
)

//go:embed template.html
var tpl string

// Render returns the rendered dashboard HTML for jsonBlob, without writing
// anything to disk. jsonBlob must be compact JSON produced by encoding/json,
// whose default HTML escaping already makes a literal </script> impossible;
// the </ replacement is kept as a second line of defence for any future
// encoder change.
func Render(jsonBlob []byte) (string, error) {
	if !strings.Contains(tpl, "__USAGE_DATA__") {
		return "", fmt.Errorf("embedded template has no __USAGE_DATA__ placeholder")
	}
	blob := strings.ReplaceAll(string(jsonBlob), "</", "<\\/")
	return strings.Replace(tpl, "__USAGE_DATA__", blob, 1), nil
}

// Write renders the report and writes it to path.
func Write(path string, jsonBlob []byte) error {
	out, err := Render(jsonBlob)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), 0o644)
}
