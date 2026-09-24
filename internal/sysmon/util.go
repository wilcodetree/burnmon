package sysmon

// deltaU64 returns cur-old, or 0 when a counter went backwards (a driver
// reset, a device or process counter that disappeared and came back under
// the same key): a wrapped uint64 subtraction would otherwise report a
// spurious multi-exabyte spike for one tick. Shared by sample_windows.go
// (disk/network counters) and process.go (per-process IO counters).
func deltaU64(cur, old uint64) uint64 {
	if cur < old {
		return 0
	}
	return cur - old
}
