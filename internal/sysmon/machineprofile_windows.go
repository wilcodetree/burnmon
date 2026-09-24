//go:build windows

package sysmon

import (
	"os/exec"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// CollectMachineProfile gathers a snapshot of this machine's hardware and
// power state for the export bundle, best-effort: a field that could not be
// read is left at its zero value rather than failing the whole collection,
// since the export must still succeed on a machine missing one signal (no
// GPU, wmic unavailable, a locked-down power policy).
func CollectMachineProfile() MachineProfile {
	p := MachineProfile{PowerSource: "unknown"}

	if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
		p.CPUModel = infos[0].ModelName
	}
	if n, err := cpu.Counts(true); err == nil {
		p.CPULogical = n
	}
	if n, err := cpu.Counts(false); err == nil {
		p.CPUPhysical = n
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		p.RAMTotalMB = float64(vm.Total) / (1 << 20)
	}
	if hi, err := host.Info(); err == nil {
		p.OS = strings.TrimSpace(hi.Platform + " " + hi.PlatformVersion)
	}
	p.GPU = gpuName()
	p.PowerPlan, p.PowerSource = powerState()
	return p
}

// gpuName shells out to wmic (present on every Windows install this app
// targets) for the video controller's own name string: the PDH-based GPU%
// counter (pressure_windows.go) reports a utilization number per engine,
// never a device name, so there is nothing to reuse from it here.
func gpuName() string {
	out, err := exec.Command("wmic", "path", "win32_VideoController", "get", "name").Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r", ""), "\n")
	for _, l := range lines[1:] {
		l = strings.TrimSpace(l)
		if l != "" {
			return l
		}
	}
	return ""
}

// powerState shells out to powercfg and wmic rather than the raw Win32
// power APIs pressure_windows.go uses for its own PDH counters: this runs
// once per export, not once a tick, so the extra process-start cost of a
// shell-out is not worth a second hand-rolled DLL binding for.
func powerState() (plan, source string) {
	if out, err := exec.Command("powercfg", "/getactivescheme").Output(); err == nil {
		s := string(out)
		if i := strings.Index(s, "("); i >= 0 {
			if j := strings.Index(s[i:], ")"); j >= 0 {
				plan = s[i+1 : i+j]
			}
		}
	}

	source = "ac"
	if out, err := exec.Command("wmic", "path", "Win32_Battery", "get", "BatteryStatus").Output(); err == nil {
		fields := strings.Fields(string(out))
		// fields[0] is the header "BatteryStatus"; a second field means a
		// battery device was reported at all (a desktop has none, so no
		// second field, leaving source at its "ac" default above).
		// BatteryStatus 2 is "AC/charging" (Win32_Battery's own MSDN table);
		// any other present value means running on battery power.
		if len(fields) >= 2 {
			if fields[1] == "2" {
				source = "ac"
			} else {
				source = "battery"
			}
		}
	} else {
		source = "unknown"
	}
	return plan, source
}
