package sysmon

import "testing"

// TestCollectMachineProfile is a smoke test only: the real collectors are
// OS syscalls and shell-outs (machineprofile_windows.go), not something a
// unit test can assert exact values against. This just proves the call
// never panics and always returns a non-empty PowerSource, on every
// platform this package builds for (the stub included).
func TestCollectMachineProfile(t *testing.T) {
	p := CollectMachineProfile()
	if p.PowerSource == "" {
		t.Fatalf("expected a non-empty PowerSource, got MachineProfile %+v", p)
	}
}
