// profiling.go: step 0 of WS3
// (02_roadmap\2026-09-26_ws3_shared_ingest_performance.md), an opt-in
// diagnostic switch for the shared-ingest CPU/memory investigation. Off by
// default; set BURNMON_PPROF=1 to write a heap profile, a 60s CPU profile,
// and a second heap profile once the CPU capture ends, into the app's data
// folder, plus a MemStats/handle-count/watch-count line to the log every
// 10s for the life of the process. No build tag: only processHandleCount
// (profiling_windows.go / profiling_other.go) is platform-specific.
package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"
)

func startProfiling(dataDir string, watchCount func() int) {
	if os.Getenv("BURNMON_PPROF") != "1" {
		return
	}
	log.Println("BURNMON_PPROF=1: profiling enabled, writing to", dataDir)

	writeHeap := func(name string) {
		f, err := os.Create(filepath.Join(dataDir, name))
		if err != nil {
			log.Println("pprof: heap profile:", err)
			return
		}
		defer f.Close()
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Println("pprof: write heap profile:", err)
		}
	}
	writeHeap("heap-start.pprof")

	if cpuFile, err := os.Create(filepath.Join(dataDir, "cpu-60s.pprof")); err != nil {
		log.Println("pprof: cpu profile:", err)
	} else if err := pprof.StartCPUProfile(cpuFile); err != nil {
		log.Println("pprof: start cpu profile:", err)
		cpuFile.Close()
	} else {
		go func() {
			time.Sleep(60 * time.Second)
			pprof.StopCPUProfile()
			cpuFile.Close()
			log.Println("pprof: wrote cpu-60s.pprof")
			writeHeap("heap-after-60s.pprof")
		}()
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			watches := -1
			if watchCount != nil {
				watches = watchCount()
			}
			log.Printf("pprof: HeapAlloc=%d Sys=%d NumGC=%d GCCPUFraction=%.4f handles=%d fsnotify_watches=%d goroutines=%d",
				mem.HeapAlloc, mem.Sys, mem.NumGC, mem.GCCPUFraction, processHandleCount(), watches, runtime.NumGoroutine())
		}
	}()
}
