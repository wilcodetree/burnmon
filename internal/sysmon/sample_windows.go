//go:build windows

package sysmon

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
)

// baseClockOnce/baseClockGHz cache cpu.Info()'s rated clock speed: a static
// property of the CPU, not a per-tick reading, read once (perfadvisor's own
// tui.go does the same, in its model's init rather than every sample()).
var (
	baseClockOnce sync.Once
	baseClockGHz  float64
)

// BaseClockGHz is the CPU box's "base 2.6 GHz" label (design doc UI review
// patch section 6). 0 when cpu.Info() could not read it.
func BaseClockGHz() float64 {
	baseClockOnce.Do(func() {
		if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
			baseClockGHz = infos[0].Mhz / 1000
		}
	})
	return baseClockGHz
}

// slowRefreshEvery ticks: readWifi (netsh) and the per-drive breakdown
// (disk.Partitions plus disk.Usage per volume, which can block for seconds
// against an unreachable mapped network drive) are both too costly, and in
// the per-drive case too risky, to run on every 2s sample (perfadvisor's own
// live TUI refreshes wifi every 4th tick of its own ~1.5s cadence, about 6s;
// this app samples every 2s, so every 5th tick is the same order of
// magnitude, about 10s). Tick 0 (the very first sample) always skips both,
// so the app's own "first system sample" startup mark is never delayed
// waiting on a shell-out or a stalled drive (found by review, 2026-09-25).
const slowRefreshEvery = 5

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
	tick    int

	lastWifi WifiSample

	// lastDisks, prevSlowIO and prevSlowAt back the per-drive breakdown's own
	// slower cadence (slowRefreshEvery): prevSlowIO/prevSlowAt are the
	// IOCounters snapshot and timestamp from the last time that breakdown
	// ran, not from the previous 2s tick, so its own read/write rates are
	// computed over its own (longer) interval.
	lastDisks  []DiskSample
	prevSlowIO map[string]disk.IOCountersStat
	prevSlowAt time.Time
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
	if sw, err := mem.SwapMemory(); err == nil {
		sm.SwapUsedMB = float64(sw.Used) / (1 << 20)
		sm.SwapTotalMB = float64(sw.Total) / (1 << 20)
	}

	io, ioErr := disk.IOCounters()
	if ioErr == nil {
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
	}

	// Per-drive breakdown and wifi (section 6's "disks"/"network" boxes):
	// both run only every slowRefreshEvery ticks (see that constant's own
	// comment), never on tick 0. Between refreshes, the samples carry
	// whatever the last slow refresh found.
	if s.tick > 0 && s.tick%slowRefreshEvery == 0 {
		slowElapsed := now.Sub(s.prevSlowAt).Seconds()
		if s.prevSlowAt.IsZero() {
			slowElapsed = 0
		}
		s.lastDisks = collectDiskSamples(io, ioErr, s.prevSlowIO, slowElapsed)
		if ioErr == nil {
			s.prevSlowIO = io
		}
		s.prevSlowAt = now
		s.lastWifi = readWifi()
	}
	sm.Disks = s.lastDisks
	sm.Wifi = s.lastWifi
	s.tick++
	if ioErr == nil {
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

// collectDiskSamples is the per-drive breakdown (design doc UI review patch,
// section 6's "disks" box: one bar per drive, free space, read and write
// rates), ported from perfadvisor's own sample() disk loop (internal/tui/
// sample.go). Only ever called from the slow-refresh branch of Tick above
// (disk.Partitions/disk.Usage can block for seconds against an unreachable
// mapped network drive, found by review, 2026-09-25), so io/ioErr and
// prevIO/elapsed here are that branch's own slower-cadence snapshot and
// interval, not the 2s tick's.
func collectDiskSamples(io map[string]disk.IOCountersStat, ioErr error, prevIO map[string]disk.IOCountersStat, elapsed float64) []DiskSample {
	var out []DiskSample
	sysDrive := strings.ToUpper(os.Getenv("SystemDrive"))
	parts, err := disk.Partitions(false)
	if err != nil {
		return out
	}
	for _, part := range parts {
		u, err := disk.Usage(part.Mountpoint)
		if err != nil || u.Total == 0 {
			continue
		}
		key := strings.ToUpper(strings.TrimRight(part.Mountpoint, `\/`))
		d := DiskSample{
			Mount: key, System: key == sysDrive,
			FreeGB: float64(u.Free) / (1 << 30), TotalGB: float64(u.Total) / (1 << 30),
			UsedPercent: u.UsedPercent,
		}
		if elapsed > 0 && prevIO != nil && ioErr == nil {
			for name, cur := range io {
				if !strings.EqualFold(strings.TrimRight(name, `\/`), key) {
					continue
				}
				if old, ok := prevIO[name]; ok {
					d.ReadBps = float64(deltaU64(cur.ReadBytes, old.ReadBytes)) / elapsed
					d.WriteBps = float64(deltaU64(cur.WriteBytes, old.WriteBytes)) / elapsed
				}
			}
		}
		out = append(out, d)
	}
	return out
}
