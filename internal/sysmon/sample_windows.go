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
// main 2ed8046). GPU and PDH pressure counters are not wired yet: Sample's
// GPUPct stays 0 until phase 3 adds them alongside the system zone.
type Sampler struct {
	prevIO  map[string]disk.IOCountersStat
	prevNet *gnet.IOCountersStat
	prevAt  time.Time
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

	s.prevAt = now
	return sm, nil
}

// deltaU64 returns cur-old, or 0 when a counter went backwards (a driver
// reset, a device that disappeared and came back under the same key): a
// wrapped uint64 subtraction would otherwise report a spurious multi-
// exabyte spike for one tick.
func deltaU64(cur, old uint64) uint64 {
	if cur < old {
		return 0
	}
	return cur - old
}
