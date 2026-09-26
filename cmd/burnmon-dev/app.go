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
	"sync/atomic"
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
	"burnmon/internal/todo"
	"burnmon/internal/vendorstrip"
	"burnmon/internal/watch"
)

const (
	version     = "0.4.0-alpha.2"
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

	// hidden is WS2 item 2's "pause when nobody looks": true while the
	// window is minimized, set by bdevSetHidden (main.go), which page.html
	// calls from its own visibilitychange listener. startSampling's cheap
	// system-sample ticker (below) reads this to slow from refresh_ms down
	// to a fixed 10s while true; the page itself stops calling
	// bdevSnapshotNow at all while hidden (paintTick never runs against a
	// window nobody can see), and fires one immediate tick the instant it
	// un-hides, rather than waiting for the next scheduled one.
	hidden atomic.Bool

	mu           sync.Mutex
	cfg          pricing.Config
	latest       sysmon.Sample
	latestGroups []sysmon.ProcessGroupSample

	// headlineDay/headlineFloor back headlineTodayMonotonic (headline.go):
	// a per-day floor on the headline total, guarded by mu like everything
	// else above. See headlineTodayMonotonic's own doc comment for why.
	headlineDay   time.Time
	headlineFloor int64

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

	// slowMu guards every "slow data" cache the shared render tick
	// (no-scroll patch section 5, bdevSnapshotNow in main.go) reads but
	// never itself recomputes: vendor strip and the activity heatmap
	// refresh every 60s, process-groups history (also the harness heatmap's
	// own source) every 10s, Microsoft To Do status every 30s and its task
	// list every 300s (startSlowRefreshers, this file) - the same cadences
	// this app used to poll each of these from the page directly, just
	// moved server-side so every panel still paints from one snapshot per
	// tick instead of resolving independently.
	slowMu                 sync.Mutex
	vendorStripCache       vendorstrip.Payload
	heatmapRowsCache       []history.Row
	processGroupsHistCache []sysmon.ProcessGroupSample
	todoStatusCache        todoStatusPayload
	todoTasksCache         todoTasksPayload
	// todoStatusGen guards a real (if narrow) race between the 30s status
	// loop below and bdevTodoLogin's own success path (main.go): both write
	// todoStatusCache from an independent read of the outside world (a disk
	// check here, "FinishLogin just returned nil" there), so a loop pass
	// that started its own disk read moments before a fresh sign-in could
	// otherwise still land after it and overwrite Signed:true with a stale
	// Signed:false. Bumped on every write; a writer that finds this has
	// moved since it started skips its own write rather than clobbering a
	// fresher one (found by review, 2026-09-25).
	todoStatusGen int
}

// closedDayRows returns the per-day token/cost rows for every day strictly
// before today (local, WS2 item 5 - this used to be a UTC boundary, the
// same class of bug WS3 fixed in vendorstrip/forecast/agg: a store event
// from the early local morning could still read as "yesterday" here for as
// long as the machine's own UTC offset, silently missing from the heatmap's
// "today" row and one day early in the closed-day cache), caching the
// whole 182-day pass and recomputing it only when the local date has
// rolled over since the last computation (once a day, not once a minute) -
// but only once the app's one-time startup backfill (startInitialCollect)
// has finished; before that, every call recomputes fresh rather than
// locking in a result built from a store that is still being backfilled.
// Returns today's own date string and local midnight alongside, so the
// caller derives "today"'s own window from the exact same now() this call
// used, rather than calling time.Now() a second time and risking a
// day-rollover mismatch between the two (also fixed after review).
func (a *app) closedDayRows(st *store.Store, cfg *pricing.Config) (closed []history.Row, today string, dayStart time.Time, cacheHit bool, err error) {
	now := time.Now().In(time.Local)
	dayStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
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
	// MicrosoftTodoEnabled turns on section 9's Microsoft To Do panel.
	// Off by default (the zero value): the panel makes network calls to
	// Microsoft Graph on the user's own credentials, so it must be an
	// explicit opt-in, not something a fresh install starts doing.
	MicrosoftTodoEnabled bool `json:"microsoft_todo_enabled"`
	// RefreshMs (no-scroll patch section 5) is the page's one shared
	// render tick, default 1000, floored at 1000: every panel repaints
	// from one bdevSnapshotNow call on this cadence, no faster. Phase 5b
	// measured a 10-minute refresh_ms 1000 run at 1.89 percent average
	// whole-machine CPU against refresh_ms 2000's own 2.22 percent
	// (process walk and persistence run on their own fixed cadences
	// regardless, see processWalkInterval/persistInterval below, so
	// halving the paint cadence did not double the cost of sampling);
	// under the 2 percent bar and not worse than the old default, so the
	// design doc's own flip condition made 1000 the new default.
	RefreshMs int `json:"refresh_ms"`
}

func loadDevConfig(dataDir string) devConfig {
	cfg := devConfig{RetentionDays: 7, RefreshMs: 1000}
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
	cfg.MicrosoftTodoEnabled = loaded.MicrosoftTodoEnabled
	if loaded.RefreshMs > 0 {
		cfg.RefreshMs = loaded.RefreshMs
	}
	if cfg.RefreshMs < 1000 {
		cfg.RefreshMs = 1000
	}
	return cfg
}

// processWalkInterval and persistInterval are phase 5b's own split
// cadences, both fixed and independent of refreshInterval (unlike the
// single shared tick this replaced): a full process.Processes() walk
// (ProcessSampler.Tick, classify plus a per-process Percent/MemoryInfo/
// IOCounters read) is the expensive part of sampling, so it no longer
// speeds up just because refresh_ms was lowered for a snappier paint
// cadence; persistence to burnmon-dev.db is by wall clock for the same
// reason, so a faster paint or process-walk cadence never multiplies
// write volume to the store. 10 minutes measured at refresh_ms 2000 and
// refresh_ms 1000 both against these same two fixed cadences, per
// 02_roadmap\2026-09-24_ws2_burnmon_dev.md's phase 5 verify pass.
const (
	processWalkInterval = 3 * time.Second
	persistInterval     = 10 * time.Second
	// hiddenSampleInterval is WS2 item 2's own number ("slow system
	// sampling to 10s") while the window is minimized; the process walk
	// and persistence above already run on their own fixed cadences
	// independent of refresh_ms (item 1), and item 2 does not ask to slow
	// those further, only the cheap per-tick system sample that would
	// otherwise keep running at refresh_ms against a window nobody can see.
	hiddenSampleInterval = 10 * time.Second
)

// startSampling runs three independent tickers instead of one shared one
// (phase 5b): refreshInterval paints the cheap, whole-machine system
// sample (a.sampler.Tick, CPU/mem/disk/net) that bdevSnapshotNow reads on
// every render tick, matching the page's own paint cadence; a fixed
// processWalkInterval runs the process-group sampler (harness CPU/RAM/IO)
// on its own clock; a fixed persistInterval writes whatever the other two
// goroutines most recently produced to burnmon-dev.db. Each ticker only
// ever touches its own sampler (both Sampler and ProcessSampler require
// single-goroutine use), and all three exchange state through a.latest/
// a.latestGroups under a.mu.
func (a *app) startSampling(refreshInterval time.Duration) {
	firstSample := true
	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		// lastSampleAt gates the hidden-window slowdown below: item 2 only
		// wants the cheap system sample itself to slow to 10s while
		// nobody's looking, not this ticker's own tick rate (a
		// refreshInterval below 10s would otherwise mean the "still
		// hidden" branch keeps re-checking the clock every tick, which is
		// cheap - a lock and a time comparison, not a real sample - so
		// that part is not worth avoiding on its own).
		var lastSampleAt time.Time
		for range ticker.C {
			if a.hidden.Load() && time.Since(lastSampleAt) < hiddenSampleInterval {
				continue
			}
			sm, err := a.sampler.Tick()
			if err != nil {
				log.Println("sysmon sample:", err)
				continue
			}
			lastSampleAt = time.Now()
			if firstSample {
				firstSample = false
				startupMark("first system sample")
			}
			a.mu.Lock()
			a.latest = sm
			a.mu.Unlock()
		}
	}()

	go func() {
		ticker := time.NewTicker(processWalkInterval)
		defer ticker.Stop()
		for range ticker.C {
			a.mu.Lock()
			cfg := a.cfg
			a.mu.Unlock()
			groups, err := a.procSampler.Tick(cfg.CopilotVSCodeOtelFile != "")
			if err != nil {
				log.Println("process sample:", err)
				continue
			}
			a.mu.Lock()
			a.latestGroups = groups
			a.mu.Unlock()
		}
	}()

	go func() {
		ticker := time.NewTicker(persistInterval)
		defer ticker.Stop()
		for range ticker.C {
			a.mu.Lock()
			sm := a.latest
			groups := a.latestGroups
			a.mu.Unlock()
			if err := a.sys.InsertSample(sm); err != nil {
				log.Println("sysmon insert:", err)
			}
			if len(groups) > 0 {
				if err := a.sys.InsertProcessGroupSamples(groups); err != nil {
					log.Println("process group insert:", err)
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
// progress, when non-nil, is section 11's own loading-screen callback
// ("Reading sessions: N of M files"): dataset.Cache.Collect already reports
// (done, total) across its own file scan, previously passed nil and never
// surfaced anywhere.
func (a *app) startInitialCollect(progress func(done, total int)) {
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
		if _, err := a.cache.Collect(&cfg, opts, progress); err != nil {
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

// startSlowRefreshers runs the background loop behind every "slow data"
// cache bdevSnapshotNow reads (no-scroll patch section 5, "may be fetched
// less often in the background, but is only painted on the shared tick"):
// vendor strip and the activity heatmap refresh every 60s, process-groups
// history (also the harness heatmap's own source) every 10s, and Microsoft
// To Do status every 30s / its task list every 300s - the same cadences
// this app used to poll each of these from the page directly, just moved
// server-side so every panel still paints from one snapshot per render
// tick instead of resolving independently. Each refresher runs once
// synchronously here before its own ticker starts, so the very first
// render tick already has real data rather than an empty cache for up to a
// whole cadence; the status loop is registered (and so runs its own first
// synchronous pass) before the task loop, so that first task refresh
// already sees a populated todoStatusCache rather than the zero value.
func (a *app) startSlowRefreshers(st *store.Store, devCfg devConfig) {
	loop := func(interval time.Duration, fn func()) {
		fn()
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for range ticker.C {
				fn()
			}
		}()
	}

	loop(60*time.Second, func() {
		a.mu.Lock()
		cfg := a.cfg
		a.mu.Unlock()
		payload, err := vendorstrip.Build(st, &cfg, time.Now())
		if err != nil {
			log.Println("slow refresh: vendor strip:", err)
			return
		}
		a.slowMu.Lock()
		a.vendorStripCache = payload
		a.slowMu.Unlock()
	})

	loop(60*time.Second, func() {
		a.mu.Lock()
		cfg := a.cfg
		a.mu.Unlock()
		rows, err := a.activityHeatmapRows(st, &cfg)
		if err != nil {
			log.Println("slow refresh: activity heatmap:", err)
			return
		}
		a.slowMu.Lock()
		a.heatmapRowsCache = rows
		a.slowMu.Unlock()
	})

	loop(10*time.Second, func() {
		since := time.Now().Add(-60 * time.Minute)
		rows, err := a.sys.RecentProcessGroups(since)
		if err != nil {
			log.Println("slow refresh: process groups history:", err)
			return
		}
		a.slowMu.Lock()
		a.processGroupsHistCache = rows
		a.slowMu.Unlock()
	})

	if !devCfg.MicrosoftTodoEnabled {
		return
	}
	loop(30*time.Second, func() {
		a.slowMu.Lock()
		genBefore := a.todoStatusGen
		wasSignedIn := a.todoStatusCache.SignedIn
		a.slowMu.Unlock()

		signedIn := todo.SignedIn() // a disk read, deliberately outside the lock

		a.slowMu.Lock()
		if a.todoStatusGen == genBefore {
			// Nothing else (bdevTodoLogin) wrote in the meantime; this
			// read is still the freshest information available.
			a.todoStatusCache = todoStatusPayload{Enabled: true, SignedIn: signedIn}
			a.todoStatusGen++
		} else {
			// A fresher write landed while signedIn was being read (most
			// likely bdevTodoLogin's own success path); trust that one
			// instead of overwriting it with this now-stale read.
			signedIn = a.todoStatusCache.SignedIn
		}
		a.slowMu.Unlock()
		if signedIn && !wasSignedIn {
			a.refreshTodoTasks() // a fresh sign-in shows tasks immediately, not up to 300s later
		}
	})
	loop(300*time.Second, func() {
		a.slowMu.Lock()
		signedIn := a.todoStatusCache.SignedIn
		a.slowMu.Unlock()
		if signedIn {
			a.refreshTodoTasks()
		}
	})
}

// activityHeatmapRows is bdevActivityHeatmap's own former body (UI review
// patch phase 2b): closedDayRows' own cached 182-day pass plus a fresh
// query for today alone, merged. Now run by startSlowRefreshers'
// background ticker instead of once per page poll.
func (a *app) activityHeatmapRows(st *store.Store, cfg *pricing.Config) ([]history.Row, error) {
	closed, today, dayStart, _, err := a.closedDayRows(st, cfg)
	if err != nil {
		return nil, err
	}
	todayEvents, err := st.EventsSince(dayStart)
	if err != nil {
		return nil, err
	}
	todayPayload := history.Build(todayEvents, cfg, history.Filter{Period: "day"})

	rows := make([]history.Row, 0, len(closed)+1)
	rows = append(rows, closed...)
	foundToday := false
	for _, r := range todayPayload.Rows {
		if r.Key == today {
			rows = append(rows, r)
			foundToday = true
		}
	}
	if !foundToday {
		rows = append(rows, history.Row{Key: today})
	}
	return rows, nil
}

// refreshTodoTasks fetches today's Microsoft To Do tasks and caches them
// for the shared render tick to paint. Unconditional (callers decide
// whether signed in first): called by startSlowRefreshers' own 300s loop
// and its post-sign-in transition above, and directly by bdevTodoLogin
// (main.go) right after a successful interactive sign-in, so the page
// shows real tasks on its very next tick instead of waiting up to 300s.
func (a *app) refreshTodoTasks() todoTasksPayload {
	items, err := todo.TodayTasks()
	payload := todoTasksPayload{Items: items}
	if err != nil {
		payload.Error = err.Error()
	}
	a.slowMu.Lock()
	a.todoTasksCache = payload
	a.slowMu.Unlock()
	return payload
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
