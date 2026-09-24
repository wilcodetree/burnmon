//go:build windows

// Command burnmon-dev is BurnMon Dev: a second WebView2 window, alongside
// burnmon.exe, that shows what running AI agents does to the token/cost
// side (burn) and the machine (system) on one screen, one time axis.
// 04_assets\2026-09-24_burnmon_dev_design.md is the design this follows.
// Phase 1 wired the system-side sampler and both viewports' empty panels;
// phase 2 (this one) starts the same live watch and adapter polls
// cmd\burnmon\app.go runs, into the same shared burnmon.db, and wires the
// burn zone (chart, session cards, vendor strip, turn ticker) against it.
package main

import (
	_ "embed"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	webview2 "github.com/jchv/go-webview2"

	"burnmon/internal/adapter/codex"
	"burnmon/internal/live"
	"burnmon/internal/scan"
	"burnmon/internal/store"
	"burnmon/internal/sysmon"
	"burnmon/internal/vendorstrip"
)

//go:embed page.html
var pageHTML string

func init() {
	// go-webview2 pumps a Win32 message loop tied to the thread that
	// created the window; keep that on one OS thread for the life of the
	// process, same as cmd\burnmon\main.go.
	runtime.LockOSThread()

	// Same fix as cmd\burnmon\app.go's own init() (v0.2.1 hang patch,
	// SESSION_LOG.md): measured live against this build, WorkingSet passed
	// 1 GB within a couple of minutes of 2s polling (bdevBurnNow's
	// live.BuildSnapshot/ApplySessionTotals, bdevSysmonHistory) before the
	// default GC pacer (GOGC 100, no soft limit) ever collected. A 400 MB
	// soft memory limit makes the runtime collect proactively as usage
	// approaches it, matching this workstream's own under-250-MB target,
	// rather than waiting for the heap to double from whatever the last
	// collection's live size happened to be.
	debug.SetMemoryLimit(400 << 20)
}

func main() {
	dataDir := appDataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return
	}
	setupLog(dataDir)
	log.Printf("burnmon-dev %s starting", version)

	if alreadyRunning() {
		log.Println("another instance is already running; bringing it to the front")
		bringExistingToFront()
		return
	}

	sysPath, err := sysmon.DefaultPath()
	if err != nil {
		log.Println("could not resolve the sysmon store path:", err)
		return
	}
	sys, err := sysmon.Open(sysPath)
	if err != nil {
		log.Println("could not open the sysmon store at", sysPath, ":", err)
		return
	}
	defer sys.Close()

	// The shared store: same file burnmon.exe reads and writes (design doc
	// section 1). Opening it here, with its own writer connection, is safe
	// whether or not burnmon.exe is running at the same time (WAL, 5s busy
	// timeout, idempotent upsert; see store.Open's own doc comment and the
	// design doc's ingest decision).
	storePath, err := store.DefaultPath()
	if err != nil {
		log.Println("could not resolve the burnmon store path:", err)
		return
	}
	st, err := store.Open(storePath)
	if err != nil {
		log.Println("could not open the burnmon store at", storePath, ":", err)
		return
	}
	defer st.Close()

	cfg, cfgFile := loadConfig(dataDir)
	if cfgFile != "" {
		log.Println("using config overrides from", cfgFile)
	}

	a := &app{sys: sys, sampler: sysmon.NewSampler(), st: st, cfg: cfg}
	a.cache.Store = st
	a.startSampling()
	a.startRetentionPrune(7)

	nativeClaudeRoots := scan.DefaultSourcesWithOptions(false)
	nativeCodexRoots := codex.NativeSources()
	a.cache.SeedNativeRoots(nativeClaudeRoots, nativeCodexRoots)
	a.startLiveWatch(nativeClaudeRoots, nativeCodexRoots)
	startHermesPoll(a, st)
	startCopilotCLIPoll(a, st)
	startCopilotVSCPoll(a, st)
	a.startInitialCollect()

	wv2Dir := filepath.Join(dataDir, "wv2-dev")
	_ = os.MkdirAll(wv2Dir, 0o755)

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		DataPath: wv2Dir,
		WindowOptions: webview2.WindowOptions{
			Title:  windowTitle,
			Width:  1152,
			Height: 2048,
			Center: true,
			IconId: 1, // matches the "#1" icon group winres/burnmon-dev.json embeds
		},
	})
	if w == nil {
		log.Println("could not create the WebView2 window; BurnMon Dev needs the WebView2 runtime")
		return
	}

	if err := w.Bind("bdevSysmonNow", func() (sysmon.Sample, error) {
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.latest, nil
	}); err != nil {
		log.Println("could not bind bdevSysmonNow:", err)
	}

	if err := w.Bind("bdevSysmonHistory", func(sinceSeconds int) ([]sysmon.Sample, error) {
		if sinceSeconds <= 0 {
			sinceSeconds = 1800
		}
		since := time.Now().Add(-time.Duration(sinceSeconds) * time.Second)
		samples, err := sys.RecentSamples(since)
		if err != nil {
			log.Println("bdevSysmonHistory:", err)
			return nil, err
		}
		return samples, nil
	}); err != nil {
		log.Println("could not bind bdevSysmonHistory:", err)
	}

	// bdevBurnNow: the burn zone's chart, session cards and turn ticker, all
	// from live.Snapshot, reused directly against the shared store (same
	// windowed EventsSince plus ApplySessionTotals cmd\burnmon\main.go's own
	// bmLive binding runs). Polled every 2s by the page, same cadence as
	// bmLive on the Now page.
	if err := w.Bind("bdevBurnNow", func() (live.Snapshot, error) {
		a.mu.Lock()
		cfg := a.cfg
		a.mu.Unlock()
		now := time.Now()
		events, err := st.EventsSince(now.Add(-live.ChartWindow))
		if err != nil {
			return live.Snapshot{}, err
		}
		snap := live.BuildSnapshot(events, &cfg, now)
		if err := live.ApplySessionTotals(snap.Sessions, st, &cfg); err != nil {
			log.Println("bdevBurnNow: session totals:", err)
		}
		return snap, nil
	}); err != nil {
		log.Println("could not bind bdevBurnNow:", err)
	}

	// bdevVendorStrip: today/week/month per vendor, reused directly, same
	// 1-minute-cadence data cmd\burnmon's own bmVendorStrip binding builds.
	if err := w.Bind("bdevVendorStrip", func() (vendorstrip.Payload, error) {
		a.mu.Lock()
		cfg := a.cfg
		a.mu.Unlock()
		return vendorstrip.Build(st, &cfg, time.Now())
	}); err != nil {
		log.Println("could not bind bdevVendorStrip:", err)
	}

	w.SetHtml(pageHTML)
	w.Run()
}
