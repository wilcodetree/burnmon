package pricing

import "testing"

// TestOwnerForEmptyRulesMeansNoOwner guards P6's default: an empty Owners
// table means one owner, no owner column shown anywhere, so OwnerFor must
// return "" for every path rather than "personal".
func TestOwnerForEmptyRulesMeansNoOwner(t *testing.T) {
	cfg := Defaults()
	if got := cfg.OwnerFor(`C:\dev\Work\project`); got != "" {
		t.Fatalf("OwnerFor with no rules = %q, want empty", got)
	}
}

// TestOwnerForOrderedRulesFirstMatchWins guards the ordered-rule matching
// and the "personal" default once rules exist.
func TestOwnerForOrderedRulesFirstMatchWins(t *testing.T) {
	cfg := Defaults()
	cfg.Owners = []OwnerRule{
		{Match: `C:\dev\Work\*`, Owner: "Valona"},
		{Match: `C:\ZND\*`, Owner: "ZND"},
	}

	cases := []struct {
		path string
		want string
	}{
		{`C:\dev\Work\project\file.go`, "Valona"},
		{`C:\ZND\projects\burnmon`, "ZND"},
		// Case-insensitive match, same as the rest of the codebase's
		// Windows-path handling (dataset.adapterForPath and friends).
		{`c:\znd\projects\burnmon`, "ZND"},
		{`C:\Users\wilco\other`, "personal"},
	}
	for _, c := range cases {
		if got := cfg.OwnerFor(c.path); got != c.want {
			t.Errorf("OwnerFor(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
