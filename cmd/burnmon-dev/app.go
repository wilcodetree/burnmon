//go:build windows

// app.go holds the app struct and the small helpers shared by main.go: data
// directory, logging, single-instance mutex, and the sysmon sampling loop.
// Windows-only: BurnMon Dev is a WebView2 window for Wilco's own two
// screens (04_assets 2026-09-24_burnmon_dev_design.md), not a portable tool
// like burnmon/burnmon-cli, so unlike those there is no browser-mode
// fallback build for darwin/linux (see main_other.go).
package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"burnmon/internal/adapter/copilotcli"
	"burnmon/internal/adapter/copilotvsc"
	"burnmon/internal/adapter/hermes"
	"burnmon/internal/dataset"
	"burnmon/internal/pricing"
	"burnmon/internal/store"
	"burnmon/internal/sysmon"
	"burnmon/internal/watch"
)

const (
	version     = "0.4.0-alpha.1"
	windowTitle = "BurnMon Dev"
	mutexName   = `Local\burnmon-dev-app`
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// appDataDir is %LOCALAPPDATA%\burnmon, the same folder burnmon.exe and
// burnmon-cli.exe already use: burnmon-dev.exe's own log and burnmon-dev.db
// land next to burnmon.db and burnmon.json, not in a folder of their own,
// since it reads and writes the same shared store (design doc section 1).
func appDataDir() string {
	if lad := os.Getenv("LOCALAPPDATA"); lad != "" {
		return filepath.Join(lad, "burnmon")
	}
	return filepath.Join(os.TempDir(), "burnmon")
}

func setupLog(dataDir string) {
	path := filepath.Join(dataDir, "burnmon-dev-app.log")
	if fi, err := os.Stat(path); err == nil && fi.Size() > 512*1024 {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(f)
}

// app holds both stores: burnmon.db (shared with burnmon.exe, read by the
// burn zone) and burnmon-dev.db (this app's own, system samples only). Per
// the design doc's ingest decision (section 1), this app runs its own
// ingest into the shared store rather than only reading what burnmon.exe
// happens to have written, so it works whether or not burnmon.exe is open.
type app struct {
	st    *store.Store
	cache dataset.Cache

	sys     *sysmon.Store
	sampler *sysmon.Sampler

	liveWatcher *watch.Watcher

	mu     sync.Mutex
	cfg    pricing.Config
	latest sysmon.Sample
}

// startLiveWatch starts an fsnotify watcher on the native Claude/Codex
// roots, same recipe as cmd\burnmon\app.go's own startLiveWatch: a live
// write calls IngestFile directly. Because dataset.Cache.IngestFile reads
// and advances its per-file cursor through a.st, which is the same
// burnmon.db file burnmon.exe writes to, a cursor burnmon.exe already
// advanced is resumed from there, not re-read from byte 0 (design doc
// section 1, point 2: idempotent, not a full re-parse).
func (a *app) startLiveWatch(nativeClaudeRoots, nativeCodexRoots []string) {
	if a.liveWatcher != nil {
		return
	}
	nativeRoots := append(append([]string{}, nativeClaudeRoots...), nativeCodexRoots...)
	wt, err := watch.New(nativeRoots, nil, func(path string) {
		a.mu.Lock()
		cfg := a.cfg
		a.mu.Unlock()
		if err := a.cache.IngestFile(&cfg, path); err != nil {
			log.Println("live watch: ingest", path, ":", err)
		}
	})
	if err != nil {
		log.Println("could not start the live watcher:", err)
		return
	}
	wt.Start()
	a.liveWatcher = wt
}

// startHermesPoll, startCopilotCLIPoll and startCopilotVSCPoll: the same
// 5-second poll cmd\burnmon\app.go runs, ported verbatim (a SQLite WAL file
// or an OTel file does not fit watch.Watcher's fsnotify-plus-offset design,
// see that file's own doc comments). Each is a no-op when the adapter has
// nothing to poll (no install found, or no copilot_vscode_otel_file
// configured).
func startHermesPoll(a *app, st *store.Store) {
	dbPath := hermes.DefaultDBPath()
	if dbPath == "" {
		return
	}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			events, err := hermes.PollOnce(dbPath)
			if err != nil {
				log.Println("hermes poll:", err)
				continue
			}
			if len(events) == 0 {
				continue
			}
			a.mu.Lock()
			cfg := a.cfg
			a.mu.Unlock()
			for i := range events {
				events[i].Owner = cfg.OwnerFor(events[i].Project)
			}
			if err := st.UpsertEvents(events); err != nil {
				log.Println("hermes poll: upsert:", err)
			}
		}
	}()
}

func startCopilotCLIPoll(a *app, st *store.Store) {
	dbPath := copilotcli.DefaultDBPath()
	if dbPath == "" {
		return
	}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			events, err := copilotcli.PollOnce(dbPath)
			if err != nil {
				log.Println("copilot cli poll:", err)
				continue
			}
			if len(events) == 0 {
				continue
			}
			a.mu.Lock()
			cfg := a.cfg
			a.mu.Unlock()
			for i := range events {
				events[i].Owner = cfg.OwnerFor(events[i].Project)
			}
			if err := st.UpsertEvents(events); err != nil {
				log.Println("copilot cli poll: upsert:", err)
			}
		}
	}()
}

func startCopilotVSCPoll(a *app, st *store.Store) {
	a.mu.Lock()
	path := a.cfg.CopilotVSCodeOtelFile
	a.mu.Unlock()
	if path == "" {
		return
	}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			a.mu.Lock()
			cfg := a.cfg
			a.mu.Unlock()
			events, err := copilotvsc.PollOnce(cfg.CopilotVSCodeOtelFile)
			if err != nil {
				log.Println("copilot vscode poll:", err)
				continue
			}
			if len(events) == 0 {
				continue
			}
			for i := range events {
				events[i].Owner = cfg.OwnerFor(events[i].Project)
				events[i].Client = cfg.ClientFor(events[i].Project)
			}
			if err := st.UpsertEvents(events); err != nil {
				log.Println("copilot vscode poll: upsert:", err)
			}
		}
	}()
}

// loadConfig mirrors cmd\burnmon's own two-tier lookup (exe-adjacent
// burnmon.json/claudecost.json first, then dataDir), so burnmon-dev.exe
// shares the same owner/client map and price overrides burnmon.exe already
// reads rather than a config of its own.
func loadConfig(dataDir string) (pricing.Config, string) {
	var path string
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if cand := filepath.Join(exeDir, "burnmon.json"); fileExists(cand) {
			path = cand
		} else if cand := filepath.Join(exeDir, "claudecost.json"); fileExists(cand) {
			path = cand
		}
	}
	if path == "" {
		if cand := filepath.Join(dataDir, "burnmon.json"); fileExists(cand) {
			path = cand
		} else if cand := filepath.Join(dataDir, "claudecost.json"); fileExists(cand) {
			path = cand
		}
	}
	cfg, err := pricing.Load(path)
	if err != nil {
		log.Println("config error:", err)
	}
	return cfg, path
}

// startSampling ticks the live sampler every 2s, keeping the latest sample
// in memory for bdevSysmonNow, and persists every 5th tick (10s) to
// burnmon-dev.db, per the design doc's sampling cadence.
func (a *app) startSampling() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		tick := 0
		for range ticker.C {
			sm, err := a.sampler.Tick()
			if err != nil {
				log.Println("sysmon sample:", err)
				continue
			}
			a.mu.Lock()
			a.latest = sm
			a.mu.Unlock()

			tick++
			if tick%5 == 0 {
				if err := a.sys.InsertSample(sm); err != nil {
					log.Println("sysmon insert:", err)
				}
			}
		}
	}()
}

// startInitialCollect runs one full backfill pass at startup: the same
// dataset.Cache.Collect call cmd\burnmon's own main() makes on its first
// run (main.go's startup goroutine, "the live watcher is registered before
// Collect is called"), so this app's vendor strip, heatmap and headline
// totals carry real history from the very first launch rather than only
// whatever the live watcher happens to see from here on. Without this,
// standalone mode (no burnmon.exe running) never backfills anything the
// watcher did not itself see fire, which contradicts the design doc's own
// ingest decision (section 1: "works whether or not burnmon.exe is open").
// Seat mirrors cmd\burnmon's own effectiveSeat precedence (an explicit
// config wins, else the "Standard" default); burnmon-dev has no -seat flag
// of its own to override it with. MonthsN matches burnmon.exe's own
// default (2); RefreshSlow true matches its first pass, which also
// resolves and walks WSL roots rather than waiting for a later refresh.
func (a *app) startInitialCollect() {
	go func() {
		a.mu.Lock()
		cfg := a.cfg
		a.mu.Unlock()
		seat := cfg.Subscription.YourSeat
		if seat != "Standard" && seat != "Premium" {
			seat = "Standard"
		}
		opts := dataset.CollectOpts{Seat: seat, MonthsN: 2, RefreshSlow: true}
		if _, err := a.cache.Collect(&cfg, opts, nil); err != nil {
			log.Println("initial collection failed:", err)
		}
	}()
}

// startRetentionPrune runs Prune once at startup and once a day after that,
// against burnmon-dev.json's retention_days (default 7; not read from
// config yet in phase 1, so the default is hard-coded here until phase 3
// wires burnmon-dev.json).
func (a *app) startRetentionPrune(retentionDays int) {
	prune := func() {
		cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
		if err := a.sys.Prune(cutoff); err != nil {
			log.Println("sysmon prune:", err)
		}
	}
	prune()
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			prune()
		}
	}()
}

// ---------------------------------------------------------------------------
// Single instance, same recipe as cmd\burnmon\main.go's own alreadyRunning/
// bringExistingToFront, a distinct mutex and window title so the two apps
// never mistake each other for a second instance of themselves.
// ---------------------------------------------------------------------------

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW         = user32.NewProc("FindWindowW")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

func alreadyRunning() bool {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return false
	}
	_, err = windows.CreateMutex(nil, false, name)
	return errors.Is(err, windows.ERROR_ALREADY_EXISTS)
}

func bringExistingToFront() {
	title, err := windows.UTF16PtrFromString(windowTitle)
	if err != nil {
		return
	}
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	if hwnd == 0 {
		return
	}
	procSetForegroundWindow.Call(hwnd)
}
