package sysmon

// resolveHarnesses classifies every process in infoByPID via Classify,
// resolving parent-chain inheritance (Classify's own last resort) with
// memoized recursion so a long chain is only ever walked once per process.
// depth is capped so a cycle (a process somehow its own ancestor, or two
// processes pointing at each other) bottoms out at HarnessOther instead of
// looping forever; a real process tree never gets close to that depth.
//
// OS-specific in this package: NewProcessSampler/ProcessSampler/Tick live in
// process_windows.go (a single NtQuerySystemInformation snapshot per tick,
// WS2 item 1) and process_other.go (gopsutil per-process queries, unchanged,
// for the untested darwin/linux browser-mode build - this function is the
// only part shared by both).
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
