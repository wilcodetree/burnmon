//go:build !windows

package sysmon

// CollectMachineProfile stub: BurnMon Dev is Windows-only, this keeps the
// package portable for `go vet`/`go test` on a non-Windows dev machine.
func CollectMachineProfile() MachineProfile {
	return MachineProfile{PowerSource: "unknown"}
}
