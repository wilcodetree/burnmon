package sysmon

import "time"

// Sample is one system-wide tick, persisted at 10s resolution in
// burnmon-dev.db's sysmon_samples table (design doc section 6). Live
// sampling itself runs every 2s in memory; only every 5th tick is stored.
type Sample struct {
	Ts           time.Time
	CPUPct       float64
	Cores        []float64 // per-core percent, in core order
	MemUsedMB    float64
	MemTotalMB   float64
	DiskReadBps  float64
	DiskWriteBps float64
	NetDownBps   float64
	NetUpBps     float64
	GPUPct       float64 // busiest GPU engine, Task Manager style

	// Pressure signals (phase 3), the same PDH counters perfadvisor's live
	// TUI reads (internal/tui/sysmetrics_windows.go), English counter names
	// so the tool works on any Windows display language. -1 means the
	// counter could not be read this tick, not that it was zero.
	CPUQueue     float64 // runnable threads waiting, System\Processor Queue Length
	CPUPerfPct   float64 // percent of base clock; <100 throttled, >100 turbo
	DiskQueueLen float64 // PhysicalDisk(_Total)\Avg. Disk Queue Length
	DiskLatMs    float64 // PhysicalDisk(_Total)\Avg. Disk sec/Transfer, in ms
	HardFaults   float64 // Memory\Pages Input/sec
}

// ProcessGroupSample is one harness's summed load at one tick, persisted in
// burnmon-dev.db's process_group_samples table so the per-harness
// sparklines and the heatmap (phase 3) read from stored history instead of
// a fresh process scan per chart render.
type ProcessGroupSample struct {
	Ts      time.Time
	Harness Harness
	CPUPct  float64
	MemMB   float64
	IOBps   float64
}
