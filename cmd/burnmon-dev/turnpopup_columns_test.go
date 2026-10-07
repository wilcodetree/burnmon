//go:build windows

package main

import (
	"strings"
	"testing"
)

// TestTurnPopupToolColumnsFit: in the compact Station card (320px,
// overflow-wrap:anywhere) the Tool, In and Out columns of "Tool calls this
// turn" collapsed to one letter per line. Those cells carry class "fit",
// and a rule that outranks the over-station td rule keeps them on one line.
// Path stays the only column that wraps.
func TestTurnPopupToolColumnsFit(t *testing.T) {
	rule := `#turnPopupOverlay.over-station #turnPopup th.fit, #turnPopupOverlay.over-station #turnPopup td.fit{white-space:nowrap; overflow-wrap:normal; width:1%}`
	if n := strings.Count(pageHTML, rule); n != 1 {
		t.Fatalf("page.html has %d copies of the fit-column rule, want 1", n)
	}
	head := `<th class="fit">Tool</th><th>Path</th><th class="fit">In</th><th class="fit">Out</th>`
	if !strings.Contains(pageHTML, head) {
		t.Fatal("tool calls table head lacks fit classes on Tool, In and Out")
	}
	for _, cell := range []string{`'<tr><td class="fit">' + esc(c.tool)`, `'<td class="num fit">' + fmtTokens(c.input_bytes)`, `'</td><td class="num fit">' +`} {
		if !strings.Contains(pageHTML, cell) {
			t.Fatalf("tool calls row lacks %s", cell)
		}
	}
}
