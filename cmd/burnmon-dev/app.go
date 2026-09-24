//go:build windows

// app.go holds the app struct and the small helpers shared by main.go: data
// directory, logging, single-instance mutex, and the sysmon sampling loop.
// Windows-only: BurnMon Dev is a WebView2 window for Wilco's own two
// screens (04_assets 2026-09-24_burnmon_dev_design.md), not a portable tool
// like burnmon/burnmon-cli, so unlike those there is no browser-mode
// fallback build for darwin/linux (see main_other.go).
package main

import (
	"encoding/json"
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
	"burnmon/internal/history"
	"burnmon/internal/pricing"
	"burnmon/internal/schema"
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

	sys         *sysmon.Store
	sampler     *sysmon.Sampler
	procSampler *sysmon.ProcessSampler

	liveWatcher *watch.Watcher

	mu           sync.Mutex
	cfg          pricing.Config
	latest       sysmon.Sample
	latestGroups []sysmon.ProcessGroupSample

	// heatmapMu guards the activity heatmap's closed-day cache (Wilco's own
	// finding, 2026-09-24: bdevActivityHeatmap ran a fresh 182-day
	// EventsSince plus history.Build on every 1-minute poll, 3.34s/52,728
	// rows measured against the real store, about 5.5 percent of one core
	// on average, alone over this workstream's 2 percent CPU target). Every
	// day except today is closed (its events never change once the day is
	// over), so that 182-day pass only needs to run once per UTC day; each
	// minute now only recomputes today's own (much smaller) row and merges
	// it onto the cached closed days. See closedDayRows below.
	heatmapMu     sync.Mutex
	heatmapClosed []history.Row
	heatmapAsOf   string // "YYYY-MM-DD" UTC: the day heatmapClosed was computed for

	// initialCollectDone is closed by startInitialCollect once its one-time
	// backfill (dataset.Cache.Collect) returns. closedDayRows must not cache
	// its result before that: the backfill is still writing past-day events
	// into the store while the page's very first heatmap poll fires (fixed
	// after review, 2026-09-24 - the first version cached whatever
	// closedDayRows saw on that first call, potentially a half-backfilled
	// store, and then never rebuilt until the next UTC midnight).
	initialCollectDone chan struct{}
}

// closedDayRows returns the per-day token/cost rows for every day strictly
// before today (UTC), caching the whole 182-day pass and recomputing it
// only when the UTC date has rolled over since the last computation (once
// a day, not once a minute) - but only once the app's one-time startup
// backfill (startInitialCollect) has finished; before that, every call
// recomputes fresh rather than locking in a result built from a store that
// is still being backfilled. Returns today's own date string and UTC
// midnight alongside, so the caller derives "today"'s own window from the
// exact same now() this call used, rather than calling time.Now() a second
// time and risking a day-rollover mismatch between the two (also fixed
// after review).
func (a *app) closedDayRows(st *store.Store, cfg *pricing.Config) (closed []history.Row, today string, dayStart time.Time, cacheHit bool, err error) {
	now := time.Now().UTC()
	dayStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	today = dayStart.Format("2006-01-02")

	a.heatmapMu.Lock()
	if a.heatmapAsOf == today {
		closed = a.heatmapClosed
		a.heatmapMu.Unlock()
		return closed, today, dayStart, true, nil
	}
	a.heatmapMu.Unlock()

	since := dayStart.AddDate(0, 0, -181)
	events, err := st.EventsSince(since)
	if err != nil {
		return nil, "", time.Time{}, false, err
	}
	closedEvents := make([]schema.Event, 0, len(events))
	for _, e := range events {
		if e.At.Before(dayStart) {
			closedEvents = append(closedEvents, e)
		}
	}
	payload := history.Build(closedEvents, cfg, history.Filter{Period: "day"})

	backfillDone := false
	select {
	case <-a.initialCollectDone:
		backfillDone = true
	default:
	}
	if backfillDone {
		a.heatmapMu.Lock()
		a.heatmapClosed = payload.Rows
		a.heatmapAsOf = today
		a.heatmapMu.Unlock()
	}
	return payload.Rows, today, dayStart, false, nil
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

// devConfig is burnmon-dev.exe's own small config file, burnmon-dev.json,
// separate from the shared burnmon.json (design doc section 6): settings
// only this app cares about. Absent or unparsable is not an error, same
// convention as an absent burnmon.json: every field keeps its default.
type devConfig struct {
	RetentionDays int `json:"retention_days"`
}

func loadDevConfig(dataDir string) devConfig {
	cfg := devConfig{RetentionDays: 7}
	path := filepath.Join(dataDir, "burnmon-dev.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	var loaded devConfig
	if err := json.Unmarshal(b, &loaded); err != nil {
		log.Println("burnmon-dev.json: parse error, using defaults:", err)
		return cfg
	}
	if loaded.RetentionDays > 0 {
		cfg.RetentionDays = loaded.RetentionDays
	}
	return cfg
}

// startSampling ticks the live sampler every 2s, keeping the latest sample
// in memory for bdevSysmonNow, and persists every 5th tick (10s) to
// burnmon-dev.db, per the design doc's sampling cadence. The same tick
// also runs the process-group sampler (harness CPU/RAM/IO), on the same
// cadence: perfadvisor's own live TUI already proves a full process scan
// once a second is affordable, so once every 2s is not a new cost class.
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
			cfg := a.cfg
			a.mu.Unlock()

			groups, err := a.procSampler.Tick(cfg.CopilotVSCodeOtelFile != "")
			if err != nil {
				log.Println("process sample:", err)
			} else {
				a.mu.Lock()
				a.latestGroups = groups
				a.mu.Unlock()
			}

			tick++
			if tick%5 == 0 {
				if err := a.sys.InsertSample(sm); err != nil {
					log.Println("sysmon insert:", err)
				}
				if len(groups) > 0 {
					if err := a.sys.InsertProcessGroupSamples(groups); err != nil {
						log.Println("process group insert:", err)
					}
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
		defer close(a.initialCollectDone)
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
