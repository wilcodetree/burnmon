package sysmon

import "testing"

func TestResolveHarnesses(t *testing.T) {
	// A small process tree:
	//   1  claude.exe                 -> Claude, by exact name
	//   2  node.exe, ppid 1           -> inherits Claude from its parent
	//   3  msedgewebview2.exe, ppid 2 -> inherits Claude, two hops up
	//   4  node.exe, ppid 999         -> no known parent, falls to "node"
	//   5  explorer.exe               -> other
	//   6  conhost.exe, ppid 6        -> self-parented; must not loop forever
	//   7  node.exe, ppid 5           -> parent (explorer.exe) resolves to
	//                                    Other, which is not a real
	//                                    ancestor classification to
	//                                    inherit: must still fall through
	//                                    to "node", the same as pid 4
	//                                    (regression test for a review
	//                                    finding, 2026-09-24: an Other
	//                                    parent used to freeze the child
	//                                    at Other too, before it ever
	//                                    reached this fallback).
	infoByPID := map[int32]Proc{
		1: {PID: 1, PPID: 0, Name: "claude.exe"},
		2: {PID: 2, PPID: 1, Name: "node.exe"},
		3: {PID: 3, PPID: 2, Name: "msedgewebview2.exe"},
		4: {PID: 4, PPID: 999, Name: "node.exe"},
		5: {PID: 5, PPID: 0, Name: "explorer.exe"},
		6: {PID: 6, PPID: 6, Name: "conhost.exe"},
		7: {PID: 7, PPID: 5, Name: "node.exe"},
	}

	got := resolveHarnesses(infoByPID, false)

	want := map[int32]Harness{
		1: HarnessClaude,
		2: HarnessClaude,
		3: HarnessClaude,
		4: HarnessNodeUnclassified,
		5: HarnessOther,
		6: HarnessOther,
		7: HarnessNodeUnclassified,
	}
	for pid, wantH := range want {
		if got[pid] != wantH {
			t.Errorf("pid %d: resolveHarnesses = %q, want %q", pid, got[pid], wantH)
		}
	}
	if len(got) != len(infoByPID) {
		t.Errorf("resolveHarnesses returned %d entries, want %d", len(got), len(infoByPID))
	}
}
