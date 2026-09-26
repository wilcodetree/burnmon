//go:build windows

package sysmon

// PDH pressure and GPU counters, ported from perfadvisor's own live TUI
// (internal/tui/sysmetrics_windows.go, perfadvisor main 2ed8046): the same
// five English-named counters (locale-independent) plus the GPU engine
// array. Kept as a small duplication rather than a shared package for the
// same reason perfadvisor's own collect/pressure_windows.go gives: each
// caller opens its own query and this file's behaviour must never risk the
// already-proven TUI code.

import (
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	pdhDLL                   = syscall.NewLazyDLL("pdh.dll")
	procPdhOpenQuery         = pdhDLL.NewProc("PdhOpenQueryW")
	procPdhAddEnglishCounter = pdhDLL.NewProc("PdhAddEnglishCounterW")
	procPdhCollectQueryData  = pdhDLL.NewProc("PdhCollectQueryData")
	procPdhGetFormattedValue = pdhDLL.NewProc("PdhGetFormattedCounterValue")
	procPdhGetFormattedArray = pdhDLL.NewProc("PdhGetFormattedCounterArrayW")
)

const (
	pdhFmtDouble = 0x00000200
	pdhMoreData  = 0x800007D2
)

type pdhFmtCounterValue struct {
	CStatus     uint32
	_           uint32
	DoubleValue float64
}

type pdhFmtCounterValueItem struct {
	SzName   *uint16
	FmtValue pdhFmtCounterValue
}

// pdhState is one PDH query, held on Sampler so it lives for the process's
// lifetime (opening a new query every tick would leak handles). Only ever
// touched from Sampler.Tick, which the app's own sampling loop calls from a
// single goroutine (see app.startSampling), so it needs no lock of its own.
type pdhState struct {
	query    uintptr
	counters map[string]uintptr
	gpu      uintptr
	ok       bool
	inited   bool
}

func (s *pdhState) init() {
	s.inited = true
	s.counters = map[string]uintptr{}
	if r, _, _ := procPdhOpenQuery.Call(0, 0, uintptr(unsafe.Pointer(&s.query))); r != 0 {
		return
	}
	paths := map[string]string{
		"cpuQueue":   `\System\Processor Queue Length`,
		"cpuPerf":    `\Processor Information(_Total)\% Processor Performance`,
		"diskQueue":  `\PhysicalDisk(_Total)\Avg. Disk Queue Length`,
		"diskLat":    `\PhysicalDisk(_Total)\Avg. Disk sec/Transfer`,
		"hardFaults": `\Memory\Pages Input/sec`,
	}
	for key, path := range paths {
		p, err := windows.UTF16PtrFromString(path)
		if err != nil {
			continue
		}
		var h uintptr
		if r, _, _ := procPdhAddEnglishCounter.Call(s.query, uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&h))); r == 0 {
			s.counters[key] = h
		}
	}
	if p, err := windows.UTF16PtrFromString(`\GPU Engine(*)\Utilization Percentage`); err == nil {
		var h uintptr
		if r, _, _ := procPdhAddEnglishCounter.Call(s.query, uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&h))); r == 0 {
			s.gpu = h
		}
	}
	// Prime rate counters; real values flow from the second collect on.
	procPdhCollectQueryData.Call(s.query)
	s.ok = true
}

func (s *pdhState) read(key string) float64 {
	h, found := s.counters[key]
	if !found {
		return -1
	}
	var v pdhFmtCounterValue
	if r, _, _ := procPdhGetFormattedValue.Call(h, pdhFmtDouble, 0, uintptr(unsafe.Pointer(&v))); r != 0 {
		return -1
	}
	return v.DoubleValue
}

// readGPU sums utilization per engine type across all GPU engine instances
// and returns the busiest type, matching Task Manager's headline GPU%.
func (s *pdhState) readGPU() float64 {
	if s.gpu == 0 {
		return -1
	}
	var bufSize, count uint32
	r, _, _ := procPdhGetFormattedArray.Call(s.gpu, pdhFmtDouble,
		uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&count)), 0)
	if (r != 0 && r != pdhMoreData) || bufSize == 0 {
		return -1
	}
	buf := make([]uint64, (bufSize+7)/8) // uint64 backing keeps the items 8-byte aligned
	r, _, _ = procPdhGetFormattedArray.Call(s.gpu, pdhFmtDouble,
		uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&buf[0])))
	if r != 0 || count == 0 {
		return -1
	}
	items := unsafe.Slice((*pdhFmtCounterValueItem)(unsafe.Pointer(&buf[0])), count)
	sums := map[string]float64{}
	for _, it := range items {
		if it.FmtValue.CStatus > 1 { // 0 valid, 1 new data
			continue
		}
		name := windows.UTF16PtrToString(it.SzName)
		typ := name
		if i := strings.LastIndex(name, "engtype_"); i >= 0 {
			typ = name[i+8:]
		}
		sums[typ] += it.FmtValue.DoubleValue
	}
	var top float64
	for _, v := range sums {
		if v > top {
			top = v
		}
	}
	if top > 100 {
		top = 100
	}
	return top
}

// pressureTick is one PDH read; ok is false when the query itself never
// initialized (e.g. PDH unavailable), in which case every field is left at
// its zero value by the caller, not this -1 "counter missing" convention.
type pressureTick struct {
	ok         bool
	cpuQueue   float64
	cpuPerfPct float64
	diskQueue  float64
	diskLatMs  float64
	hardFaults float64
	gpuPct     float64
}

func (s *pdhState) readTick() pressureTick {
	if !s.inited {
		s.init()
	}
	if !s.ok {
		return pressureTick{}
	}
	if r, _, _ := procPdhCollectQueryData.Call(s.query); r != 0 {
		return pressureTick{}
	}
	t := pressureTick{ok: true}
	t.cpuQueue = s.read("cpuQueue")
	t.cpuPerfPct = s.read("cpuPerf")
	t.diskQueue = s.read("diskQueue")
	if v := s.read("diskLat"); v >= 0 {
		t.diskLatMs = v * 1000
	} else {
		t.diskLatMs = -1
	}
	t.hardFaults = s.read("hardFaults")
	t.gpuPct = s.readGPU()
	return t
}
