package sysmon

import "math"

// PressureScore maps one Sample to a single 0-100 severity score, the
// header's pressure chip (design doc section 8: "needs the real
// PressureInfo ranges from a live machine to calibrate against" - this is
// a documented first-pass formula, meant to be recalibrated once real
// usage patterns are seen, not a precise scientific instrument). Higher
// means more system pressure.
//
// Each component normalizes its own signal against a reference point where
// that signal is considered fully saturated, then the five components are
// combined by weighted average. A component whose signal could not be
// read this tick (Sample's own -1 "unavailable" convention, or a core
// count of zero for the CPU-queue component, which needs one to normalize
// against) is excluded entirely rather than treated as zero pressure: the
// remaining components' weights are renormalized so their sum is still
// out of 100, instead of a missing counter silently dragging the score
// toward "calm".
func PressureScore(s Sample) int {
	var weightedSum, totalWeight float64
	add := func(weight, value float64) {
		weightedSum += weight * clamp0to100(value)
		totalWeight += weight
	}

	// CPU load itself is always a real reading (0 is idle, not "unknown").
	add(0.30, s.CPUPct)

	// A processor queue sustained above 2x the core count indicates CPU
	// saturation (Microsoft's own perfmon guidance for this counter).
	if cores := len(s.Cores); s.CPUQueue >= 0 && cores > 0 {
		add(0.20, 100*s.CPUQueue/(2*float64(cores)))
	}

	// 20ms+ average disk latency is poor even for a spinning disk; an SSD
	// under real pressure crosses this well before that.
	if s.DiskLatMs >= 0 {
		add(0.20, 100*s.DiskLatMs/20)
	}

	if s.MemTotalMB > 0 {
		add(0.20, 100*s.MemUsedMB/s.MemTotalMB)
	}

	if s.GPUPct >= 0 {
		add(0.10, s.GPUPct)
	}

	if totalWeight == 0 {
		return 0
	}
	return int(math.Round(clamp0to100(weightedSum / totalWeight)))
}

func clamp0to100(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
