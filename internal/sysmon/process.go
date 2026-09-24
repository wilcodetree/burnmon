package sysmon

import (
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// resolveHarnesses classifies every process in infoByPID via Classify,
// resolving parent-chain inheritance (Classify's own last resort) with
// memoized recursion so a long chain is only ever walked once per process.
// depth is capped so a cycle (a process somehow its own ancestor, or two
// processes pointing at each other) bottoms out at HarnessOther instead of
// looping forever; a real process tree never gets close to that depth.
func resolveHarnesses(infoByPID map[int32]Proc, copilotVSCodeConfigured bool) map[int32]Harness {
	const maxDepth = 32
	resolved := make(map[int32]Harness, len(infoByPID))

	var resolve func(pid int32, depth int) Harness
	resolve = func(pid int32, depth int) Harness {
		if h, ok := resolved[pid]; ok {
			return h
		}
		if depth > maxDepth {
			return HarnessOther
		}
		info, ok := infoByPID[pid]
		if !ok {
			return HarnessOther
		}
		h := Classify(info, copilotVSCodeConfigured, func(ppid int32) (Harness, bool) {
			if _, ok := infoByPID[ppid]; !ok {
				return "", false
			}
			// A parent that itself resolved to HarnessOther is not a real
			// ancestor classification to inherit: report "not found" so
			// Classify falls through to its own name-based fallback (e.g.
			// a plain node.exe under an unclassified shell still lands in
			// HarnessNodeUnclassified) instead of silently freezing at
			// Other before ever reaching that fallback (found by review,
			// 2026-09-24: nearly every process has *some* live parent, so
			// without this, unclassified processes almost never reached
			// their own bucket).
			if parentH := resolve(ppid, depth+1); parentH != HarnessOther {
				return parentH, true
			}
			return "", false
		})
		resolved[pid] = h
		return h
	}

	out := make(map[int32]Harness, len(infoByPID))
	for pid := range infoByPID {
		out[pid] = resolve(pid, 0)
	}
	return out
}

type trackedProc struct {
	p         *process.Process
	lastRead  uint64
	lastWrite uint64
	hasIO     bool
	// isNew is true only for the tick a process was first registered:
	// Percent(0) was only just primed a few lines above in that same Tick
	// call, so a read this same tick is over a near-zero interval and
	// gopsutil can report a wildly inflated percentage for it (found by
	// review, 2026-09-24). Cleared after being skipped once.
	isNew bool
}

// ProcessSampler holds per-process delta state (CPU% and IO rates both
// need two points) across ticks, matching perfadvisor's own registry
// pattern (internal/tui/sample.go's procReg). Not safe for concurrent use;
// call Tick from one goroutine.
type ProcessSampler struct {
	reg    map[int32]*trackedProc
	lastAt time.Time
}

func NewProcessSampler() *ProcessSampler {
	return &ProcessSampler{reg: map[int32]*trackedProc{}}
}

// Tick samples every running process, classifies each into a harness
// (Proc's own precedence: cmdline, then exact name, then its parent
// chain), and returns one ProcessGroupSample per harness with the CPU/RAM/
// IO summed across every process in it. HarnessOther is dropped: the
// process groups panel only ever shows named harnesses (design doc
// section 4). The first call after NewProcessSampler has zero IO rates,
// since a rate needs two points.
func (ps *ProcessSampler) Tick(copilotVSCodeConfigured bool) ([]ProcessGroupSample, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}

	seen := make(map[int32]bool, len(procs))
	infoByPID := make(map[int32]Proc, len(procs))
	for _, p := range procs {
		seen[p.Pid] = true
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		cmdline, _ := p.Cmdline()
		ppid, _ := p.Ppid()
		infoByPID[p.Pid] = Proc{PID: p.Pid, PPID: ppid, Name: name, Cmdline: cmdline}

		if _, ok := ps.reg[p.Pid]; !ok {
			tp := &trackedProc{p: p, isNew: true}
			_, _ = p.Percent(0) // prime the CPU delta
			if ioc, err := p.IOCounters(); err == nil && ioc != nil {
				tp.lastRead, tp.lastWrite, tp.hasIO = ioc.ReadBytes, ioc.WriteBytes, true
			}
			ps.reg[p.Pid] = tp
		}
	}
	for pid := range ps.reg {
		if !seen[pid] {
			delete(ps.reg, pid)
		}
	}

	harnessByPID := resolveHarnesses(infoByPID, copilotVSCodeConfigured)

	now := time.Now()
	elapsed := now.Sub(ps.lastAt).Seconds()
	if ps.lastAt.IsZero() || elapsed <= 0 {
		elapsed = 0
	}
	ps.lastAt = now

	sums := map[Harness]*ProcessGroupSample{}
	for pid, tp := range ps.reg {
		h := harnessByPID[pid]
		if h == "" || h == HarnessOther {
			continue
		}
		pct, err := tp.p.Percent(0)
		if err != nil {
			continue
		}
		if tp.isNew {
			pct = 0
			tp.isNew = false
		}
		var memMB float64
		if mi, err := tp.p.MemoryInfo(); err == nil && mi != nil {
			memMB = float64(mi.RSS) / (1 << 20)
		}
		var ioBps float64
		if tp.hasIO && elapsed > 0 {
			if ioc, err := tp.p.IOCounters(); err == nil && ioc != nil {
				ioBps = float64(deltaU64(ioc.ReadBytes, tp.lastRead)+deltaU64(ioc.WriteBytes, tp.lastWrite)) / elapsed
				tp.lastRead, tp.lastWrite = ioc.ReadBytes, ioc.WriteBytes
			}
		}

		s := sums[h]
		if s == nil {
			s = &ProcessGroupSample{Ts: now, Harness: h}
			sums[h] = s
		}
		s.CPUPct += pct
		s.MemMB += memMB
		s.IOBps += ioBps
	}

	out := make([]ProcessGroupSample, 0, len(sums))
	for _, s := range sums {
		out = append(out, *s)
	}
	return out, nil
}
