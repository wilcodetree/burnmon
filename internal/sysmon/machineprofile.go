package sysmon

// MachineProfile is a one-time snapshot of the machine's hardware and power
// state, collected once per export (internal/devexport, phase 4's export
// bundle): the model reading summary.md needs to know what hardware
// produced these numbers, not just the numbers themselves.
type MachineProfile struct {
	CPUModel    string  `json:"cpu_model"`
	CPULogical  int     `json:"cpu_logical_cores"`
	CPUPhysical int     `json:"cpu_physical_cores"`
	RAMTotalMB  float64 `json:"ram_total_mb"`
	GPU         string  `json:"gpu"`
	OS          string  `json:"os"`
	PowerPlan   string  `json:"power_plan"`
	// PowerSource is "ac", "battery" or "unknown".
	PowerSource string `json:"power_source"`
}
