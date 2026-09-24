//go:build windows

package sysmon

import (
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
)

// Sampler holds the delta state one tick needs against the previous one:
// disk and network counters are cumulative since boot, so only their rate
// over the tick window is useful. Not safe for concurrent use; call Tick
// from one goroutine, matching perfadvisor's own sample() (internal/tui/
// sample.go), the source this file's approach is copied from (perfadvisor
// main 2ed8046). pdh holds one PDH query for the process's lifetime
// (pressure_windows.go), opened lazily on the first Tick.
type Sampler struct {
	prevIO  map[string]disk.IOCountersStat
	prevNet *gnet.IOCountersStat
	prevAt  time.Time
	pdh     pdhState
}

func NewSampler() *Sampler { return &Sampler{} }

// Tick takes one system-wide sample. The first call after NewSampler has
// zero disk/network rates, since a rate needs two points; every later call
// has a real one.
func (s *Sampler) Tick() (Sample, error) {
	now := time.Now()
	elapsed := now.Sub(s.prevAt).Seconds()
	if s.prevAt.IsZero() {
		elapsed = 0
	}

	var sm Sample
	sm.Ts = now

	if v, err := cpu.Percent(0, false); err == nil && len(v) > 0 {
		sm.CPUPct = v[0]
	}
	if v, err := cpu.Percent(0, true); err == nil {
		sm.Cores = v
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		sm.MemUsedMB = float64(vm.Used) / (1 << 20)
		sm.MemTotalMB = float64(vm.Total) / (1 << 20)
	}

	if io, err := disk.IOCounters(); err == nil {
		if elapsed > 0 && s.prevIO != nil {
			var readB, writeB uint64
			for name, cur := range io {
				if old, ok := s.prevIO[name]; ok {
					readB += deltaU64(cur.ReadBytes, old.ReadBytes)
					writeB += deltaU64(cur.WriteBytes, old.WriteBytes)
				}
			}
			sm.DiskReadBps = float64(readB) / elapsed
			sm.DiskWriteBps = float64(writeB) / elapsed
		}
		s.prevIO = io
	}

	if nics, err := gnet.IOCounters(false); err == nil && len(nics) > 0 {
		cur := nics[0]
		if elapsed > 0 && s.prevNet != nil {
			sm.NetDownBps = float64(deltaU64(cur.BytesRecv, s.prevNet.BytesRecv)) / elapsed
			sm.NetUpBps = float64(deltaU64(cur.BytesSent, s.prevNet.BytesSent)) / elapsed
		}
		s.prevNet = &cur
	}

	pt := s.pdh.readTick()
	if pt.ok {
		sm.CPUQueue = pt.cpuQueue
		sm.CPUPerfPct = pt.cpuPerfPct
		sm.DiskQueueLen = pt.diskQueue
		sm.DiskLatMs = pt.diskLatMs
		sm.HardFaults = pt.hardFaults
		sm.GPUPct = pt.gpuPct
	} else {
		sm.CPUQueue, sm.CPUPerfPct, sm.DiskQueueLen, sm.DiskLatMs, sm.HardFaults, sm.GPUPct = -1, -1, -1, -1, -1, -1
	}

	s.prevAt = now
	return sm, nil
}
