//go:build windows

// Command burnmon is the single-window desktop face of burnmon: a
// WebView2 window that collects your Claude usage on startup, on an
// interval, and on demand, and shows the same dashboard template as the CLI,
// live, in place. No server, no open port, no tray, no browser tab.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"

	"burnmon/internal/adapter/codex"
	"burnmon/internal/adapter/copilotcli"
	"burnmon/internal/adapter/hermes"
	"burnmon/internal/dataset"
	"burnmon/internal/forecast"
	"burnmon/internal/history"
	"burnmon/internal/live"
	"burnmon/internal/pricing"
	"burnmon/internal/report"
	"burnmon/internal/scan"
	"burnmon/internal/store"
	"burnmon/internal/vendorstrip"
	"burnmon/internal/watch"
)

const (
	version        = "0.2.3"
	windowTitle    = "BurnMon"
	mutexName      = `Local\burnmon-app`
	minInterval    = 5 * time.Minute
	minWSLInterval = 15 * time.Minute
)

func init() {
	// go-webview2 pumps a Win32 message loop tied to the thread that created
	// the window; keep that on one OS thread for the life of the process.
	runtime.LockOSThread()

	// v0.2.1 hang patch (SESSION_LOG.md): Refresh's own live (reachable)
	// footprint against Wilco's real store measured at well under 100 MB
	// (isolated AllEvents + a forced GC), but Go's default GC pacer (GOGC
	// 100, no soft limit) lets HeapAlloc run up to whatever the last
	// collection's live size implies before the next one, and a rebuild's
	// burst of allocation (Collect's own event/session slices, plus however
	// many bmSessionInsight/bmHistory goroutines the page fires at once)
	// measured that at 1+ GB before a GC ever ran. A 400 MB soft memory
	// limit (matching the Done-when target) makes the runtime collect
	// proactively as usage approaches it rather than waiting for the heap
	// to double; a burst that briefly needs more than 400 MB of genuinely
	// live data still gets it (this is a soft target, not a hard cap), the
	// difference is only how eagerly garbage gets reclaimed under load.
	debug.SetMemoryLimit(400 << 20)
}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ", ") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func main() {
	interval := flag.Duration("interval", 15*time.Minute, "how often to re-read transcripts while the window is open")
	wslInterval := flag.Duration("wsl-interval", 4*time.Hour, "how often to re-read WSL transcripts while the window is open (native transcripts stay on -interval)")
	monthsN := flag.Int("months", 2, "how many months to include, counting the current one")
	seat := flag.String("seat", "Standard", "your own seat tier (Standard or Premium)")
	cfgPath := flag.String("config", "", "config file overriding the compiled-in prices and subscription (default: burnmon.json in the app's data folder, if present)")
	var sources multiFlag
	flag.Var(&sources, "source", "extra folder to scan; repeatable, overrides auto-detect")
	flag.Parse()

	if *interval < minInterval {
		*interval = minInterval
	}

	// -wsl-interval wins over wsl_interval_hours in config; whether it was
	// actually passed (as opposed to sitting at its flag default) is only
	// knowable via flag.Visit, since flag.Duration itself cannot tell "not
	// set" from "set to the default". Same trick for -seat against
	// your_seat: an explicit -seat always wins, otherwise a your_seat in
	// burnmon.json beats the flag's own "Standard" default, so a shared
	// config for a team on one tier (see the Groundwork Kit) never needs
	// anyone to open Settings on first run.
	wslIntervalFlagSet := false
	seatFlagSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "wsl-interval" {
			wslIntervalFlagSet = true
		}
		if f.Name == "seat" {
			seatFlagSet = true
		}
	})

	dataDir := appDataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		// No window, no console, and now nowhere to log either: nothing
		// sensible left to do.
		return
	}
	setupLog(dataDir)
	log.Printf("burnmon %s starting", version)

	// Two-tier lookup, exe-adjacent first: this makes burnmon.exe plus
	// burnmon.json droppable together into one portable folder (the
	// ZeroNonsense.dev Groundwork Kit ships exactly this pair), the same
	// "drop one file next to the exe" pattern the CLI already used and that
	// burnmon.json's own bundled _comment describes. Falls back to
	// %LOCALAPPDATA%\burnmon\burnmon.json, so a fixed install that
	// must run from a read-only share (nothing written or read next to the
	// exe in that case) keeps working exactly as before: place
	// burnmon.json in %LOCALAPPDATA%\burnmon\ and restart the app.
	// Whichever file loads is also where Settings saves land afterward
	// (cfgSavePath below), so a Groundwork Kit user editing Settings keeps
	// writing to their own portable folder, not %LOCALAPPDATA%.
	// claudecost.json is the pre-rename config name (v0.1.2 and earlier);
	// still read if present so an existing portable folder or fixed install
	// keeps working after the burnmon rename, but burnmon.json always wins
	// when both exist.
	cfgFile := *cfgPath
	if cfgFile == "" {
		if exe, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exe)
			if cand := filepath.Join(exeDir, "burnmon.json"); fileExists(cand) {
				cfgFile = cand
			} else if cand := filepath.Join(exeDir, "claudecost.json"); fileExists(cand) {
				cfgFile = cand
			}
		}
	}
	if cfgFile == "" {
		if cand := filepath.Join(dataDir, "burnmon.json"); fileExists(cand) {
			cfgFile = cand
		} else if cand := filepath.Join(dataDir, "claudecost.json"); fileExists(cand) {
			cfgFile = cand
		}
	}
	cfg, err := pricing.Load(cfgFile)
	if err != nil {
		log.Println("config error:", err)
		return
	}
	if cfgFile != "" {
		log.Println("using config overrides from", cfgFile)
	}

	// Zero or absent config value means the flag's own 4h default, whether
	// or not the flag was explicitly passed.
	effectiveWSLInterval := *wslInterval
	if !wslIntervalFlagSet && cfg.WSLIntervalHours > 0 {
		effectiveWSLInterval = time.Duration(cfg.WSLIntervalHours * float64(time.Hour))
	}
	effectiveSeat := *seat
	if !seatFlagSet && cfg.Subscription.YourSeat != "" {
		effectiveSeat = cfg.Subscription.YourSeat
	}
	if effectiveWSLInterval < minWSLInterval {
		effectiveWSLInterval = minWSLInterval
	}
	wslEnabled := cfg.WSLScan != "off"

	// Where Settings saves land: the file that was actually loaded, or the
	// default location next to the store if none existed yet, so the very
	// first save from the window creates it rather than erroring.
	cfgSavePath := cfgFile
	if cfgSavePath == "" {
		cfgSavePath = filepath.Join(dataDir, "burnmon.json")
	}

	if alreadyRunning() {
		log.Println("another instance is already running; bringing it to the front")
		bringExistingToFront()
		return
	}

	wv2Dir := filepath.Join(dataDir, "wv2")
	_ = os.MkdirAll(wv2Dir, 0o755)

	storePath, err := store.DefaultPath()
	if err != nil {
		log.Println("could not resolve the store path:", err)
		return
	}
	st, err := store.Open(storePath)
	if err != nil {
		log.Println("could not open the local store at", storePath, ":", err)
		return
	}
	defer st.Close()

	// Best-effort cleanup of the old gob-based parse cache, replaced by the
	// SQLite store above; ignore any error, missing is the expected case on
	// every run after the first.
	_ = os.Remove(filepath.Join(dataDir, "parsecache.gob"))

	a := &app{
		cfg:         cfg,
		seat:        effectiveSeat,
		monthsN:     *monthsN,
		sources:     []string(sources),
		htmlPath:    filepath.Join(dataDir, "dashboard.html"),
		interval:    *interval,
		wslInterval: effectiveWSLInterval,
		cfgPath:     cfgSavePath,
	}
	a.cache.Store = st

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		DataPath: wv2Dir,
		WindowOptions: webview2.WindowOptions{
			Title:  windowTitle,
			Width:  1280,
			Height: 860,
			Center: true,
			IconId: 1, // matches the "#1" icon group winres/app.json embeds
		},
	})
	if w == nil {
		log.Println("could not create the WebView2 window; falling back to the default browser")
		runWithoutWindow(a)
		return
	}

	// A dashboard.html from a previous run is shown immediately, stale
	// "Snapshot taken" stamp and all: that stamp is already honest about
	// its own age, so no extra "refreshing" banner is needed on top of it.
	// A true first run has nothing to show yet, so it keeps the warming
	// page with the progress bar instead.
	hadDashboard := fileExists(a.htmlPath)
	if hadDashboard {
		w.Navigate(toFileURL(a.htmlPath))
	} else {
		w.SetHtml(warmingPageHTML())
	}

	// W1 (02_roadmap\2026-09-23_v0.2.3_window_check_patch.md): everything
	// from here down used to run synchronously on this goroutine, which is
	// the UI thread w.Run() below pumps messages on (runtime.LockOSThread
	// in this file's init). w.SetHtml/w.Navigate above only queue a
	// navigation; nothing paints until the message loop is actually
	// running, so scan.DefaultSourcesWithOptions and codex.NativeSources
	// (both walk the filesystem) blocked the very first paint, reading as a
	// blank, "Not Responding" window until they and the first collection
	// below finished. Moving it all into the same startup goroutine that
	// already did the first collection fixes this: w.Run() starts pumping
	// messages immediately after this call, so the warming/stale page paints
	// first and everything else happens behind it. F2 (SESSION_LOG.md,
	// v0.1.1) still holds: the live watcher is registered before Collect is
	// called below, in the same order as before, just one goroutine later.
	go func() {
		nativeClaudeRoots := scan.DefaultSourcesWithOptions(false)
		nativeCodexRoots := codex.NativeSources()
		a.cache.SeedNativeRoots(nativeClaudeRoots, nativeCodexRoots)
		a.startLiveWatch(nativeClaudeRoots, nativeCodexRoots)
		startHermesPoll(a, st)
		startCopilotCLIPoll(a, st)

		if _, err := a.rebuild(true, progressReporter(w)); err != nil {
			log.Println("initial collection failed:", err)
			if !hadDashboard {
				msg := startupFailureMessage(err)
				blob, _ := json.Marshal(msg)
				w.Dispatch(func() { w.Eval("window.ccWarmFail && ccWarmFail(" + string(blob) + ")") })
			}
			return
		}
		// The backfill above has now resolved the WSL roots (if any); add
		// them to the watcher already running on native roots.
		a.extendLiveWatchWSL(nativeClaudeRoots, nativeCodexRoots)
		if hadDashboard {
			w.Dispatch(func() { w.Eval("window.ccReload ? ccReload() : location.reload()") })
			return
		}
		fileURL := toFileURL(a.htmlPath)
		w.Dispatch(func() { w.Navigate(fileURL) })
	}()

	// wslTicker drives the slow (WSL) cadence; nil when wsl_scan is off, so
	// that case never even starts a background timer for it. resetWSLTicker
	// is what "Refresh now and Settings-save reset it" (Two refresh
	// cadences) means in practice: without this, pressing Refresh right
	// before the four-hour mark would still trigger a second full WSL walk
	// moments later.
	var wslTicker *time.Ticker
	var tickerMu sync.Mutex
	if wslEnabled {
		wslTicker = time.NewTicker(a.wslInterval)
	}
	resetWSLTicker := func() {
		if wslTicker == nil {
			return
		}
		tickerMu.Lock()
		wslTicker.Reset(a.wslInterval)
		tickerMu.Unlock()
	}

	if err := w.Bind("ccRefresh", func() {
		go func() {
			if _, err := a.rebuild(true, progressReporter(w)); err != nil {
				log.Println("refresh failed:", err)
				w.Dispatch(func() { w.Eval("window.ccRefreshFailed && window.ccRefreshFailed()") })
				return
			}
			resetWSLTicker()
			w.Dispatch(func() { w.Eval("window.ccReload ? ccReload() : location.reload()") })
		}()
	}); err != nil {
		log.Println("could not bind ccRefresh:", err)
	}

	var bmLivePolls atomic.Uint64
	if err := w.Bind("bmLive", func() (live.Snapshot, error) {
		callStart := time.Now()
		a.mu.Lock()
		cfg := a.cfg
		a.mu.Unlock()
		now := time.Now()
		// F1: bmLive is polled every 2 seconds by the Now page. It used to
		// call st.AllEvents() and re-group the whole table on every tick
		// (measured at 1,886 MB, memory thrashing); EventsSince bounds the
		// read to live.ChartWindow, and ApplySessionTotals below fills in
		// each running session's true lifetime totals from a small SQL
		// aggregate instead of the full history.
		storeStart := time.Now()
		events, err := st.EventsSince(now.Add(-live.ChartWindow))
		storeWait := time.Since(storeStart)
		if err != nil {
			log.Printf("bound bmLive: %v total, %v waiting for the store (EventsSince error: %v)", time.Since(callStart), storeWait, err)
			return live.Snapshot{}, err
		}
		snap := live.BuildSnapshot(events, &cfg, now)
		totalsStart := time.Now()
		if err := live.ApplySessionTotals(snap.Sessions, st, &cfg); err != nil {
			log.Println("bmLive: session totals:", err)
		}
		storeWait += time.Since(totalsStart)
		if n := bmLivePolls.Add(1); n%30 == 0 {
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			log.Printf("debug: bmLive poll %d, HeapAlloc=%d bytes", n, mem.HeapAlloc)
		}
		if elapsed := time.Since(callStart); elapsed > 50*time.Millisecond {
			log.Printf("bound bmLive: %v total, %v waiting for the store", elapsed, storeWait)
		}
		return snap, nil
	}); err != nil {
		log.Println("could not bind bmLive:", err)
	}

	// bmTurn (I3): the Now page's "explain this spike" drawer, called on
	// demand when a marker or ticker line is clicked, not polled. Re-reads
	// sessionID's whole event history (EventsForSession, any vendor) rather
	// than bmLive's windowed one, so a drawer opened on a turn that has
	// since scrolled out of the 30-minute chart still resolves.
	//
	// N1 (v0.2.2, SESSION_LOG.md): this used to return live.BuildTurnDetail's
	// result directly, synchronously, on go-webview2's Bind callback, which
	// v0.2.1's own hang patch documented as running on the same UI thread
	// the window paints and processes input on. On the real store a turn
	// drawer open re-reads a session's whole event history, so while that
	// call is in flight the window (including the drawer's own Close button
	// and Esc) cannot respond, which reads as "the drawer cannot be closed"
	// rather than "the window is briefly frozen". Same fix as
	// bmSessionInsight/bmHistory below: return immediately, do the real work
	// in a goroutine, resolve through asyncResolveJS.
	if err := w.Bind("bmTurn", func(sessionID string, turn int) {
		go func() {
			a.mu.Lock()
			cfg := a.cfg
			a.mu.Unlock()
			start := time.Now()
			detail, err := live.BuildTurnDetail(st, &cfg, sessionID, turn)
			if err != nil {
				log.Println("bmTurn:", sessionID, turn, ":", err)
			}
			if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
				log.Printf("bound bmTurn: %v waiting for the store", elapsed)
			}
			key := fmt.Sprintf("%s:%d", sessionID, turn)
			js, err := asyncResolveJS("__bmTurnResolve", key, detail)
			if err != nil {
				log.Println("bmTurn: encode result:", err)
				return
			}
			w.Dispatch(func() { w.Eval(js) })
		}()
	}); err != nil {
		log.Println("could not bind bmTurn:", err)
	}

	// bmSessionInsight (I3, Sessions tab): the findings column and its
	// expandable row call this on demand, one sessionID at a time, never
	// polled; the frontend caches the result in page memory so re-expanding
	// a row costs nothing further. renderSessions() (template.html) fires
	// one of these per kept session on every page load, unconditionally
	// (hundreds at once against a real history, see SESSION_LOG.md v0.2.1
	// hang patch): go-webview2's Bind runs the bound Go function
	// synchronously on the same Win32 message-pump thread the window
	// paints and processes input on, so hundreds of them in a row, even at
	// a few tens of milliseconds each, add up to the UI thread being fully
	// consumed for the whole burst. This binding therefore returns
	// immediately (case (b): "or move it to a goroutine that resolves
	// through w.Dispatch") and does the real work in a goroutine, pushing
	// the result back into the page by calling a small JS-side resolver
	// once it is ready; asyncResolveJS below is the shared plumbing both
	// this and bmHistory use for that.
	if err := w.Bind("bmSessionInsight", func(sessionID string) {
		go func() {
			a.mu.Lock()
			cfg := a.cfg
			a.mu.Unlock()
			start := time.Now()
			findings, err := live.BuildSessionInsight(st, &cfg, sessionID)
			if err != nil {
				log.Println("bmSessionInsight:", sessionID, ":", err)
				findings = nil
			}
			if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
				log.Printf("bound bmSessionInsight: %v waiting for the store", elapsed)
			}
			js, err := asyncResolveJS("__bmSessionInsightResolve", sessionID, findings)
			if err != nil {
				log.Println("bmSessionInsight: encode result:", err)
				return
			}
			w.Dispatch(func() { w.Eval(js) })
		}()
	}); err != nil {
		log.Println("could not bind bmSessionInsight:", err)
	}

	// bmHistory (P2): queried on demand from the History tab's filter
	// controls, not polled, so it re-reads the whole store on every call
	// rather than the windowed query bmLive uses for its 2-second poll.
	// Same UI-thread concern as bmSessionInsight above (a large store's
	// AllEvents plus history.Build's own aggregation can run past 50ms), so
	// it returns immediately and resolves asynchronously too; unlike
	// bmSessionInsight this fires once per filter change, not once per
	// session, so the win here is "never freezes on a big store" rather
	// than "no longer fires hundreds at once".
	if err := w.Bind("bmHistory", func(reqID string, f history.Filter) {
		go func() {
			a.mu.Lock()
			cfg := a.cfg
			a.mu.Unlock()
			start := time.Now()
			events, err := st.AllEvents()
			if err != nil {
				log.Println("bmHistory: AllEvents:", err)
				return
			}
			payload := history.Build(events, &cfg, f)
			if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
				log.Printf("bound bmHistory: %v waiting for the store, %d rows", elapsed, len(events))
			}
			js, err := asyncResolveJS("__bmHistoryResolve", reqID, payload)
			if err != nil {
				log.Println("bmHistory: encode result:", err)
				return
			}
			w.Dispatch(func() { w.Eval(js) })
		}()
	}); err != nil {
		log.Println("could not bind bmHistory:", err)
	}

	// bmVendorStrip (P3): the Now page's vendor strip calls this from its
	// own 1-minute timer, not bmLive's 2-second poll, so it runs one SQL
	// aggregate against the whole events table rather than bmLive's
	// windowed read.
	if err := w.Bind("bmVendorStrip", func() (vendorstrip.Payload, error) {
		callStart := time.Now()
		a.mu.Lock()
		cfg := a.cfg
		a.mu.Unlock()
		payload, err := vendorstrip.Build(st, &cfg, time.Now())
		if elapsed := time.Since(callStart); elapsed > 50*time.Millisecond {
			log.Printf("bound bmVendorStrip: %v total, %v waiting for the store", elapsed, elapsed)
		}
		return payload, err
	}); err != nil {
		log.Println("could not bind bmVendorStrip:", err)
	}

	// bmForecast (F1): the Now page's forecast chart calls this from its own
	// 1-minute timer, same reason as bmVendorStrip above; it also carries
	// the scoring bookkeeping (EnsureScored), so a week's plan and actual
	// get recorded even if the Now page forecast section is never scrolled
	// to, as long as the app is open at least once a minute somewhere in
	// that week.
	if err := w.Bind("bmForecast", func() (forecast.Payload, error) {
		callStart := time.Now()
		a.mu.Lock()
		cfg := a.cfg
		a.mu.Unlock()
		payload, err := forecast.Build(st, &cfg, time.Now())
		if elapsed := time.Since(callStart); elapsed > 50*time.Millisecond {
			log.Printf("bound bmForecast: %v total, %v waiting for the store", elapsed, elapsed)
		}
		return payload, err
	}); err != nil {
		log.Println("could not bind bmForecast:", err)
	}

	startUICheckServer(w)

	if err := w.Bind("ccSaveSettings", func(p settingsPayload) error {
		if err := a.applySettings(p); err != nil {
			return err
		}
		go func() {
			if _, err := a.rebuild(true, progressReporter(w)); err != nil {
				log.Println("rebuild after settings save failed:", err)
				w.Dispatch(func() { w.Eval("window.ccSettingsRebuildFailed && window.ccSettingsRebuildFailed()") })
				return
			}
			resetWSLTicker()
			w.Dispatch(func() { w.Eval("window.ccReload ? ccReload() : location.reload()") })
		}()
		return nil
	}); err != nil {
		log.Println("could not bind ccSaveSettings:", err)
	}

	// ccSaveView (U3): the monitor/full header switch persists its choice
	// immediately, through the same merge-into-JSON save path as Settings
	// (writeConfigKey), but without a rebuild: the view choice changes
	// nothing about the underlying data, only which chrome the page shows,
	// so a client-side re-render is enough.
	if err := w.Bind("ccSaveView", func(view string) error {
		if view != "monitor" {
			view = "full"
		}
		a.mu.Lock()
		a.cfg.View = view
		err := writeConfigKey(a.cfgPath, "view", view)
		a.mu.Unlock()
		return err
	}); err != nil {
		log.Println("could not bind ccSaveView:", err)
	}

	go func() {
		ticker := time.NewTicker(*interval)
		defer ticker.Stop()
		for range ticker.C {
			// The 15-minute (or whatever -interval is) tier never touches
			// WSL: RefreshSlow false, so Collect reuses the last full
			// pass's slow-tier file list instead of walking it again.
			if _, err := a.rebuild(false, progressReporter(w)); err != nil {
				log.Println("scheduled collection failed:", err)
				continue
			}
			// Reload the page so the new snapshot shows without a click.
			// ccReload (app chrome) stashes the active tab first so the
			// reload lands back on the same tab, not on Overview.
			w.Dispatch(func() { w.Eval("window.ccReload ? ccReload() : location.reload()") })
		}
	}()

	if wslTicker != nil {
		go func() {
			defer wslTicker.Stop()
			for range wslTicker.C {
				if _, err := a.rebuild(true, progressReporter(w)); err != nil {
					log.Println("scheduled WSL collection failed:", err)
					continue
				}
				w.Dispatch(func() { w.Eval("window.ccReload ? ccReload() : location.reload()") })
			}
		}()
	}

	w.Run()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// startupFailureMessage turns a Collect error from the very first pass (no
// dashboard.html yet to fall back on) into plain text for the warming page.
// Before this, any of these three errors was logged and then silently
// swallowed, leaving the window stuck on "Reading your session
// transcripts... Starting..." forever: exactly what a chat-only user, or
// anyone whose Cowork sessions all run in Anthropic's cloud, hits, since
// neither leaves a local transcript for this machine to find.
func startupFailureMessage(err error) string {
	switch {
	case errors.Is(err, dataset.ErrNoSources):
		return "No Claude transcript folders were found on this machine. Cowork sessions that run in Anthropic's cloud leave no local transcript, so there may be nothing local to read yet."
	case errors.Is(err, dataset.ErrNoFiles):
		return "Claude's folders exist but contain no transcript files. Cloud Cowork sessions and claude.ai browser chats leave no local transcript."
	case errors.Is(err, dataset.ErrNoSessions):
		return "Transcripts were found, but none with usage inside the reporting window."
	default:
		return "Could not read usage data: " + err.Error()
	}
}

// progressReporter turns a Collect-shaped (done,total) callback into live
// updates on the page via ccProgress(done, total, etaSeconds|null). Call it
// fresh immediately before each rebuild so its elapsed-time clock, which the
// estimate is based on, starts at zero every time.
func progressReporter(w webview2.WebView) func(done, total int) {
	start := time.Now()
	return func(done, total int) {
		if total <= 0 {
			return
		}
		eta := "null"
		if done > 0 {
			perFile := time.Since(start) / time.Duration(done)
			remaining := perFile * time.Duration(total-done)
			eta = strconv.Itoa(int(remaining.Seconds()))
		}
		d, t := done, total
		w.Dispatch(func() {
			w.Eval(fmt.Sprintf("window.ccProgress && window.ccProgress(%d,%d,%s)", d, t, eta))
		})
	}
}

// asyncResolveJS builds the JS statement that delivers a result computed off
// the UI thread back into the page: bmSessionInsight and bmHistory both
// return immediately from their bound Go function and finish their real
// work in a goroutine (see their own comments, v0.2.1 hang patch), then call
// this to hand the result to a small page-side resolver by key (a sessionID
// or a request id) instead of through go-webview2's own auto-generated
// per-call Promise, which only resolves with whatever the bound function
// returns synchronously. encoding/json's default HTML escaping makes a
// literal "</script>" impossible in either json.Marshal call below, the same
// property report.Render's own doc comment relies on.
func asyncResolveJS(resolverName, key string, data any) (string, error) {
	keyJSON, err := json.Marshal(key)
	if err != nil {
		return "", err
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("window.%s && window.%s(%s, %s)", resolverName, resolverName, keyJSON, dataJSON), nil
}

// ---------------------------------------------------------------------------
// app: the one rebuild path shared by startup, both tickers, Refresh now and
// Settings save
// ---------------------------------------------------------------------------

type app struct {
	cfg         pricing.Config
	seat        string
	monthsN     int
	sources     []string
	htmlPath    string
	interval    time.Duration
	wslInterval time.Duration
	cfgPath     string

	cache      dataset.Cache
	wslDistros []string
	mu         sync.Mutex
	building   atomic.Bool

	liveWatcher *watch.Watcher
}

// startLiveWatch starts the Now page's file watcher on nativeClaudeRoots and
// nativeCodexRoots: fsnotify, recursively, no WSL yet (see extendLiveWatchWSL).
// Called once, synchronously, from main() before the first backfill even
// starts (v0.1.1 F2: previously this ran only after the first full rebuild
// completed, so a live turn arriving during that backfill, which can take
// minutes over a large existing history, had no watcher to catch it at
// all). onChange calls IngestFile directly, without a.mu: dataset.Cache
// guards its own shared state (rootsMu) and the store serialises through
// its single connection, so a live ingest is never blocked behind a
// concurrently-running Collect the way it would be if this held a.mu for
// that call, as rebuild() itself no longer does either.
func (a *app) startLiveWatch(nativeClaudeRoots, nativeCodexRoots []string) {
	if a.liveWatcher != nil {
		return
	}
	nativeRoots := append(append([]string{}, nativeClaudeRoots...), nativeCodexRoots...)
	wt, err := watch.New(nativeRoots, nil, func(path string) {
		// A shallow copy, not a live pointer into a.cfg: same read-safety
		// idiom rebuild() already uses (see its own comment above), since
		// this callback runs with no lock held against a concurrent
		// Settings save.
		cfg := a.cfg
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

// extendLiveWatchWSL adds whatever WSL roots the first full backfill
// resolved (a.cache.RootsByAdapter, now populated) to the already-running
// live watcher, once, right after that backfill completes. A no-op when
// wsl_scan is off, no distro was found, or the watcher never started.
func (a *app) extendLiveWatchWSL(nativeClaudeRoots, nativeCodexRoots []string) {
	if a.liveWatcher == nil {
		return
	}
	full := a.cache.RootsSnapshot()
	fullRoots := append(append([]string{}, full["claude"]...), full["codex"]...)
	nativeRoots := append(append([]string{}, nativeClaudeRoots...), nativeCodexRoots...)
	a.liveWatcher.AddWSLRoots(diffRootsCaseInsensitive(fullRoots, nativeRoots))
}

// diffRootsCaseInsensitive returns the entries of all not present in remove,
// comparing case-insensitively (Windows paths). Local, minimal copy of
// dataset's own subtractCaseInsensitive: not worth exporting one function
// for one caller outside that package.
func diffRootsCaseInsensitive(all, remove []string) []string {
	skip := make(map[string]bool, len(remove))
	for _, r := range remove {
		skip[strings.ToLower(r)] = true
	}
	var out []string
	for _, a := range all {
		if !skip[strings.ToLower(a)] {
			out = append(out, a)
		}
	}
	return out
}

// startHermesPoll starts A1's 5-second Hermes poll, a no-op if no Hermes
// install is found (DefaultDBPath returns ""). No fsnotify watch, unlike
// startLiveWatch's Claude/Codex trails: a SQLite WAL file's own writes do
// not fit watch.Watcher's file-offset, .jsonl-only design (see
// internal/adapter/hermes's package doc). Hermes needs no cursor either:
// PollOnce returns every session's current running totals on every call,
// and UpsertEvents' own "largest output wins" upsert (RequestID fixed to
// the session id) already skips a no-op write when a session has not grown.
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

// startCopilotCLIPoll starts A2's 5-second Copilot CLI poll, a no-op if no
// Copilot CLI install is found (DefaultDBPath returns ""). Same shape as
// startHermesPoll for the same reason: session-store.db is a SQLite WAL
// file, not a .jsonl trail, so it does not fit watch.Watcher's
// fsnotify-plus-offset design; PollOnce needs no cursor either, since it
// always returns every session's current latest-row totals and the store's
// own upsert skips a no-op write when a session has not grown.
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

var errRebuildBusy = errors.New("a collection is already running")

// rebuild collects, renders and writes dashboard.html, reporting progress
// through progress if non-nil. It is guarded so only one collection runs at
// a time; a call that lands while another is already in flight (a tick
// firing mid-refresh) is skipped, not queued, and reports errRebuildBusy.
//
// refreshSlow is threaded straight through to dataset.CollectOpts.
// RefreshSlow: true means this pass is allowed to (re)resolve and walk WSL
// sources, false means it must not touch WSL at all and instead reuses
// whatever the last true pass found. See "Two refresh cadences" in
// docs/2026-08-17_wsl-source-detection-design.md.
func (a *app) rebuild(refreshSlow bool, progress func(done, total int)) (time.Time, error) {
	if !a.building.CompareAndSwap(false, true) {
		return time.Time{}, errRebuildBusy
	}
	defer a.building.Store(false)

	// F2: copy the config and take everything else Collect needs under a
	// brief lock, then run Collect (the long part: it lists, reads and
	// ingests every source file) without holding a.mu for the whole call.
	// Before this fix, a.mu was held for rebuild's entire body, so the live
	// watcher's IngestFile calls (also gated on a.mu) queued behind
	// whichever rebuild was in flight, including the multi-minute initial
	// backfill, matching the reported "Codex card minutes late" symptom
	// exactly. dataset.Cache now guards its own shared state internally
	// (rootsMu), so this is safe; Collect gets its own cfg snapshot rather
	// than a live pointer into a.cfg, so a concurrent Settings save cannot
	// race it (that save triggers its own follow-up rebuild regardless).
	a.mu.Lock()
	cfg := a.cfg
	opts := dataset.CollectOpts{
		Seat:        a.seat,
		MonthsN:     a.monthsN,
		Sources:     a.sources,
		RefreshSlow: refreshSlow,
	}
	a.mu.Unlock()

	collectStart := time.Now()
	payload, err := a.cache.Collect(&cfg, opts, progress)
	log.Printf("stage Collect (total): %v", time.Since(collectStart))
	if err != nil {
		return time.Time{}, err
	}
	{
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		log.Printf("debug: after Collect, HeapAlloc=%d bytes, Sys=%d bytes", mem.HeapAlloc, mem.Sys)
	}
	// Reflects whatever the most recent WSL probe (if any ran this pass, or
	// the last pass that did) actually found; empty when WSL is off, not
	// installed, or found nothing.
	wslDistros := scan.WSLDistroNames()
	logSourceScan(a.cache.Sources, a.cache.Files)
	marshalStart := time.Now()
	blob, err := json.Marshal(payload)
	if err != nil {
		return time.Time{}, err
	}
	log.Printf("stage json.Marshal: %v, %d bytes", time.Since(marshalStart), len(blob))
	renderStart := time.Now()
	html, err := report.Render(blob)
	if err != nil {
		return time.Time{}, err
	}
	log.Printf("stage report.Render: %v", time.Since(renderStart))

	a.mu.Lock()
	a.wslDistros = wslDistros
	built := time.Now()
	html = a.applyAppChrome(html)
	a.mu.Unlock()

	writeStart := time.Now()
	if err := os.WriteFile(a.htmlPath, []byte(html), 0o600); err != nil {
		return time.Time{}, err
	}
	log.Printf("stage WriteFile: %v, %d bytes", time.Since(writeStart), len(html))
	return built, nil
}

// logSourceScan writes one line per scanned source folder with the number of
// transcript files found under it, plus a total. Kept deliberately terse: it
// exists so a "my dashboard stopped at last month" report can be diagnosed
// from app.log alone, by seeing which folders were scanned and which were
// empty, without touching the user's machine.
func logSourceScan(sources, files []string) {
	for _, src := range sources {
		n := 0
		prefix := src + string(os.PathSeparator)
		for _, f := range files {
			if strings.HasPrefix(f, prefix) {
				n++
			}
		}
		log.Printf("source %s: %d transcript files", src, n)
	}
	log.Printf("sources scanned: %d, transcript files total: %d", len(sources), len(files))
}

// ---------------------------------------------------------------------------
// Settings: the Subscription block of the pricing config, editable from the
// window itself instead of by hand-editing burnmon.json.
// ---------------------------------------------------------------------------

// settingsPayload is the shape ccSaveSettings receives from the settings
// modal's Save button. Field names match the JS object literal exactly;
// go-webview2 unmarshals the JS argument straight into this struct.
type settingsPayload struct {
	YourSeat                  string  `json:"yourSeat"`
	MonthlySubscriptionEUR    float64 `json:"monthlySubscriptionEUR"`
	MonthlySubscriptionUSD    float64 `json:"monthlySubscriptionUSD"`
	SeatsPurchased            int     `json:"seatsPurchased"`
	StandardSeats             int     `json:"standardSeats"`
	PremiumSeats              int     `json:"premiumSeats"`
	StandardSeatPriceUSD      float64 `json:"standardSeatPriceUSD"`
	PremiumSeatPriceUSD       float64 `json:"premiumSeatPriceUSD"`
	UsageCreditsBalanceEUR    float64 `json:"usageCreditsBalanceEUR"`
	UsageCreditsSpentEUR      float64 `json:"usageCreditsSpentEUR"`
	UsageCreditsMonthlyCapEUR float64 `json:"usageCreditsMonthlyCapEUR"`
	CompanyConsumptionUSD     float64 `json:"companyConsumptionUSD"`
	OutputCostFactor          float64 `json:"outputCostFactor"`
	CalibratedOn              string  `json:"calibratedOn"`
	Window                    string  `json:"window"`
	// DefaultView (U3) sets pricing.Config.View, the monitor/full start
	// state, alongside the subscription block above.
	DefaultView string `json:"defaultView"`
}

// applySettings writes p to burnmon.json (preserving any other keys
// already in that file, such as an unusual Prices override or the wsl_scan
// / extra_sources fields), then updates the running config and seat in
// memory. Cost is computed fresh from the config at Collect time, so
// nothing needs invalidating here; the caller triggers the actual rebuild
// afterward.
func (a *app) applySettings(p settingsPayload) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	sub := pricing.Subscription{
		MonthlySubscriptionEUR:    p.MonthlySubscriptionEUR,
		MonthlySubscriptionUSD:    p.MonthlySubscriptionUSD,
		SeatsPurchased:            p.SeatsPurchased,
		Seats:                     map[string]int{"Standard": p.StandardSeats, "Premium": p.PremiumSeats},
		SeatPriceUSD:              map[string]float64{"Standard": p.StandardSeatPriceUSD, "Premium": p.PremiumSeatPriceUSD},
		UsageCreditsBalanceEUR:    p.UsageCreditsBalanceEUR,
		UsageCreditsSpentEUR:      p.UsageCreditsSpentEUR,
		UsageCreditsMonthlyCapEUR: p.UsageCreditsMonthlyCapEUR,
		CompanyConsumptionUSD:     p.CompanyConsumptionUSD,
		OutputCostFactor:          p.OutputCostFactor,
		CalibratedOn:              p.CalibratedOn,
		Window:                    p.Window,
		YourSeat:                  p.YourSeat,
	}
	if err := writeConfigKey(a.cfgPath, "subscription", sub); err != nil {
		return err
	}
	view := p.DefaultView
	if view != "monitor" {
		view = "full"
	}
	if err := writeConfigKey(a.cfgPath, "view", view); err != nil {
		return err
	}

	a.cfg.Subscription = sub
	a.cfg.View = view
	if p.YourSeat == "Standard" || p.YourSeat == "Premium" {
		a.seat = p.YourSeat
	}
	return nil
}

// writeConfigKey merges value into the named key of the JSON file at path,
// leaving any other keys (an unusual Prices override, wsl_scan,
// extra_sources) exactly as they were. A missing or unreadable existing file
// is treated as empty, not an error: this is very likely the first time
// anyone has saved from the window. Written via a .tmp file plus
// os.Rename, so a crash mid-write never corrupts the real file. Shared by
// the settings dialog (ccSaveSettings, "subscription" and "view") and the
// monitor/full header switch (ccSaveView, "view" alone).
func writeConfigKey(path, key string, value interface{}) error {
	raw := map[string]json.RawMessage{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &raw)
	}
	valBytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw[key] = valBytes

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// escapeAttr is a minimal HTML attribute escape for the couple of text
// fields (CalibratedOn, Window) that land inside a value="..." attribute.
// Not a general-purpose escaper, just enough for values the user themselves
// typed into this same form a moment ago.
func escapeAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// ---------------------------------------------------------------------------
// HTML chrome: turn the CLI's rendered page into the app window's page.
// The embedded template.html itself is never touched.
// ---------------------------------------------------------------------------

const warmingPageTemplate = `<!doctype html><title>BurnMon</title><body style="font-family:sans-serif;background:#1f2733;
color:#eee;display:grid;place-items:center;height:100vh;margin:0;overflow:hidden">
<div style="text-align:center;min-width:320px">
 <div>Reading your session transcripts&hellip;</div>
 <div style="margin:14px auto 0;width:320px;height:8px;background:#334455;border-radius:4px;overflow:hidden">
  <div id="cc_warm_fill" style="width:0%;height:100%;background:#FFDD32;transition:width .2s"></div>
 </div>
 <div id="cc_warm_text" style="margin-top:8px;font-size:13px;color:#9fb4bd">Starting&hellip;</div>
 <div style="margin-top:22px;font-size:11px;color:#5b6b74">BurnMon vAPP_VERSION</div>
</div>
<script>
function ccProgress(done,total,eta){
  if(total<=0) return;
  var pct = Math.round(done/total*100);
  var fill = document.getElementById('cc_warm_fill');
  if(fill) fill.style.width = pct+'%';
  var t = document.getElementById('cc_warm_text');
  if(t) t.textContent = done+' / '+total+' files ('+pct+'%)'+(eta!=null && eta>0 ? ', about '+eta+'s left' : '');
}
function ccWarmFail(msg){
  var t = document.getElementById('cc_warm_text');
  if(t){ t.textContent = msg; t.style.color = '#ff8a65'; }
  var f = document.getElementById('cc_warm_fill');
  if(f) f.style.background = '#ff8a65';
}
</script>
</body>`

func warmingPageHTML() string {
	return strings.Replace(warmingPageTemplate, "APP_VERSION", version, 1)
}

const cliRebuildNotice = `<b>This page is rebuilt every time you run the burnmon CLI again.</b> It reads your session transcripts live at each run, so refreshing is simply running it again: a new report is written and opened for you.`

// appRebuildNotice produces the app window's "how this stays current" text.
// With no WSL distros found, it reproduces today's single-cadence wording
// byte for byte. With WSL distros found, it names them and states both
// cadences, since the "Snapshot taken" stamp would otherwise read as more
// current than the WSL half of the numbers actually is.
func appRebuildNotice(interval, wslInterval time.Duration, wslDistros []string) string {
	if len(wslDistros) == 0 {
		return fmt.Sprintf(`<b>This window keeps itself current.</b> It opens straight to your last snapshot and quietly catches up in the background within moments. It also re-reads your session transcripts every %d minutes while open, whenever you press Refresh now (top right), and right after you save changes in Settings (the gear icon).`,
			int(interval/time.Minute))
	}
	names := strings.Join(wslDistros, ", ")
	return fmt.Sprintf(`<b>This window keeps itself current.</b> It opens straight to your last snapshot and quietly catches up in the background within moments. Windows transcripts are re-read every %d minutes while open. WSL transcripts (%s) are re-read every %s, because reading Linux files from Windows is slow. Refresh now (top right) and saving Settings re-read everything, WSL included.`,
		int(interval/time.Minute), names, formatHours(wslInterval))
}

// formatHours renders a duration the way the header text wants it: whole
// hours as "N hour(s)", anything else as minutes.
func formatHours(d time.Duration) string {
	if d > 0 && d%time.Hour == 0 {
		h := int(d / time.Hour)
		if h == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", h)
	}
	return fmt.Sprintf("%d minutes", int(d/time.Minute))
}

// stampAnchor is the exact markup template.html renders for the "Snapshot
// taken" box in the header. Its own script fills it in by id, so it stays
// intact; we just wrap it together with our button so both sit in the same
// spot, top right, instead of adding a second, duplicate timestamp of our
// own at the bottom.
const stampAnchor = `<div class="stamp" id="stamp"></div>`

func (a *app) stampAreaHTML() string {
	wslStamp := ""
	if len(a.wslDistros) > 0 && !a.cache.SlowScannedAt.IsZero() {
		wslStamp = ` <span style="color:var(--muted);font-size:11px;white-space:nowrap" title="WSL transcripts are re-read on a slower, ` +
			formatHours(a.wslInterval) +
			` cycle because reading Linux files from Windows is slow; native Windows transcripts are current as of the main snapshot time.">WSL data as of ` +
			a.cache.SlowScannedAt.Format("15:04") + `</span>`
	}
	return `<div style="display:flex;align-items:center;gap:12px">
 <span style="color:var(--sun);font-size:11px;white-space:nowrap">v` + version + `</span>` + wslStamp + `
 <span id="cc_progress" style="display:none;color:var(--muted);font-size:12px;white-space:nowrap"></span>
 <button id="cc_settings_btn" class="act ghost" type="button" title="Subscription settings" style="padding:6px 10px;white-space:nowrap">&#9881;</button>
 <button id="cc_btn" class="act" type="button" style="padding:6px 12px;white-space:nowrap">Refresh now</button>
 ` + stampAnchor + `
</div>`
}

// appChromeStyle is a small CSS override, not a template edit: the Sessions
// tab's own scrollable table (".scroll.tall.with-filters", template.html's
// class, unique to that tab) is sized for a generic browser viewport. In our
// fixed 1280x860 window that leaves the page slightly taller than the
// window, so a second, outer scrollbar appears alongside the table's own
// one. Shrinking just that box's max-height keeps everything on one screen.
// Only the Sessions tab is affected; Months/Weeks/Days are untouched.
const appChromeStyle = `
<style>
.scroll.tall.with-filters{max-height:calc(100vh - 500px) !important}
</style>`

const appChromeScript = `
<script>
(function(){
  var btn = document.getElementById('cc_btn');
  if(!btn) return;
  var progressEl = document.getElementById('cc_progress');
  var hideTimer = null;

  window.ccProgress = function(done, total, eta){
    if(!progressEl || total<=0) return;
    var pct = Math.round(done/total*100);
    progressEl.style.display = 'inline';
    progressEl.textContent = pct+'% ('+done+'/'+total+')'+(eta!=null && eta>0 ? ', ~'+eta+'s left' : '');
    if(hideTimer) clearTimeout(hideTimer);
    if(done>=total){
      hideTimer = setTimeout(function(){ progressEl.style.display='none'; }, 800);
    }
  };

  // ccReload: reload the page for a fresh snapshot, landing back on the
  // tab the user was on. Tab state is in-memory only (see template.html's
  // tab navigation notes), so it is stashed in localStorage across the
  // reload. Used by both interval tickers, Refresh now, and Settings save.
  window.ccReload = function(){
    try{ localStorage.setItem('cc_tab', (typeof currentTab === 'function') ? currentTab() : 'overview'); }catch(e){}
    location.reload();
  };

  // Restore the stashed tab after a ccReload-driven reload.
  try{
    var savedTab = localStorage.getItem('cc_tab');
    if(savedTab){
      localStorage.removeItem('cc_tab');
      if(savedTab !== 'overview' && typeof showTab === 'function') showTab(savedTab);
    }
  }catch(e){}

  window.ccRefreshFailed = function(){
    btn.disabled = false;
    btn.textContent = 'Refresh failed – try again';
    btn.style.borderColor = 'var(--warn)';
    btn.style.color = 'var(--warn)';
  };

  btn.onclick = function(){
    btn.disabled = true;
    btn.style.borderColor = '';
    btn.style.color = '';
    btn.textContent = 'Rebuilding…';
    window.ccRefresh();
  };
})();
</script>`

func (a *app) applyAppChrome(html string) string {
	if n := strings.Count(html, cliRebuildNotice); n != 1 {
		log.Printf("warning: rebuild notice found %d times in the rendered template, expected 1", n)
	}
	html = strings.Replace(html, cliRebuildNotice, appRebuildNotice(a.interval, a.wslInterval, a.wslDistros), 1)

	if n := strings.Count(html, stampAnchor); n != 1 {
		log.Printf("warning: snapshot stamp found %d times in the rendered template, expected 1; Refresh now not placed", n)
	} else {
		html = strings.Replace(html, stampAnchor, a.stampAreaHTML(), 1)
	}

	if !strings.Contains(html, "</body>") {
		log.Println("warning: no </body> in the rendered template; chrome not injected")
		return html
	}
	return strings.Replace(html, "</body>", appChromeStyle+appChromeScript+a.settingsModalHTML()+"</body>", 1)
}

// settingsModalHTML renders the Settings overlay, pre-filled with the
// currently loaded subscription numbers and seat, so opening it always
// shows what the dashboard is actually using right now, not stale form
// defaults. Saving posts to ccSaveSettings (bound in main), which writes
// burnmon.json and rebuilds in the background.
func (a *app) settingsModalHTML() string {
	sub := a.cfg.Subscription
	std := sub.Seats["Standard"]
	prem := sub.Seats["Premium"]
	stdPrice := sub.SeatPriceUSD["Standard"]
	premPrice := sub.SeatPriceUSD["Premium"]
	selected := func(seat string) string {
		if a.seat == seat {
			return " selected"
		}
		return ""
	}
	selectedView := func(view string) string {
		if a.cfg.View == view || (a.cfg.View == "" && view == "full") {
			return " selected"
		}
		return ""
	}
	f := func(v float64) string { return fmt.Sprintf("%v", v) }
	n := func(v int) string { return fmt.Sprintf("%d", v) }

	return `
<div id="cc_settings_overlay" style="display:none;position:fixed;inset:0;background:rgba(0,0,0,.55);z-index:1000;align-items:center;justify-content:center;font-family:'Inter','Segoe UI',sans-serif">
 <div style="background:#1f2733;color:#eee;border-radius:10px;padding:22px 26px;width:480px;max-height:86vh;overflow:auto;box-shadow:0 12px 40px rgba(0,0,0,.5)">
  <h3 style="margin:0 0 4px;font-size:16px">Subscription settings</h3>
  <p style="margin:0 0 16px;color:#9fb4bd;font-size:12px">Saved to ` + escapeAttr(a.cfgPath) + `.</p>
  <div id="cc_settings_error" style="display:none;margin-bottom:12px;color:#ff8a65;font-size:12px"></div>
  <style>
   #cc_settings_overlay label{display:flex;flex-direction:column;gap:4px;font-size:12px;color:#c8d4d9}
   #cc_settings_overlay input,#cc_settings_overlay select{background:#111a24;border:1px solid var(--pine-40);border-radius:6px;color:#fff;padding:6px 8px;font-size:13px}
   #cc_settings_overlay .grid{display:grid;grid-template-columns:1fr 1fr;gap:10px 14px}
  </style>
  <div class="grid">
   <label>Your seat
    <select id="cc_s_yourSeat">
     <option value="Standard"` + selected("Standard") + `>Standard</option>
     <option value="Premium"` + selected("Premium") + `>Premium</option>
    </select></label>
   <label>Default view
    <select id="cc_s_defaultView">
     <option value="full"` + selectedView("full") + `>Full</option>
     <option value="monitor"` + selectedView("monitor") + `>Monitor</option>
    </select></label>
   <label>Monthly subscription (EUR)<input id="cc_s_subEUR" type="number" step="0.01" min="0" value="` + f(sub.MonthlySubscriptionEUR) + `"></label>
   <label>Monthly subscription (USD)<input id="cc_s_subUSD" type="number" step="0.01" min="0" value="` + f(sub.MonthlySubscriptionUSD) + `"></label>
   <label>Seats purchased<input id="cc_s_seatsTotal" type="number" step="1" min="0" value="` + n(sub.SeatsPurchased) + `"></label>
   <div></div>
   <label>Standard seats<input id="cc_s_seatsStd" type="number" step="1" min="0" value="` + n(std) + `"></label>
   <label>Premium seats<input id="cc_s_seatsPrem" type="number" step="1" min="0" value="` + n(prem) + `"></label>
   <label>Standard seat price (USD)<input id="cc_s_priceStd" type="number" step="0.01" min="0" value="` + f(stdPrice) + `"></label>
   <label>Premium seat price (USD)<input id="cc_s_pricePrem" type="number" step="0.01" min="0" value="` + f(premPrice) + `"></label>
   <label>Usage credits balance (EUR)<input id="cc_s_creditsBal" type="number" step="0.01" min="0" value="` + f(sub.UsageCreditsBalanceEUR) + `"></label>
   <label>Usage credits spent (EUR)<input id="cc_s_creditsSpent" type="number" step="0.01" min="0" value="` + f(sub.UsageCreditsSpentEUR) + `"></label>
   <label>Usage credits monthly cap (EUR)<input id="cc_s_creditsCap" type="number" step="0.01" min="0" value="` + f(sub.UsageCreditsMonthlyCapEUR) + `"></label>
   <label>Company consumption (USD)<input id="cc_s_companyUSD" type="number" step="0.01" min="0" value="` + f(sub.CompanyConsumptionUSD) + `"></label>
  </div>
  <details style="margin-top:14px">
   <summary style="cursor:pointer;color:#9fb4bd;font-size:12px">Advanced (recalibration)</summary>
   <div class="grid" style="margin-top:10px">
    <label>Output cost factor<input id="cc_s_factor" type="number" step="0.0001" min="0" value="` + f(sub.OutputCostFactor) + `"></label>
    <div></div>
    <label>Calibrated on<input id="cc_s_calibratedOn" type="text" value="` + escapeAttr(sub.CalibratedOn) + `"></label>
    <label>Window<input id="cc_s_window" type="text" value="` + escapeAttr(sub.Window) + `"></label>
   </div>
  </details>
  <div style="display:flex;justify-content:flex-end;gap:10px;margin-top:20px">
   <button id="cc_s_cancel" style="padding:6px 14px;border:1px solid var(--pine-40);border-radius:6px;background:transparent;color:#fff;font:13px 'Inter','Segoe UI',sans-serif;cursor:pointer">Cancel</button>
   <button id="cc_s_save" style="padding:6px 14px;border:1px solid #FFDD32;border-radius:6px;background:transparent;color:#FFDD32;font:13px 'Inter','Segoe UI',sans-serif;cursor:pointer">Save and reload</button>
  </div>
 </div>
</div>
<script>
(function(){
  var btn = document.getElementById('cc_settings_btn');
  var overlay = document.getElementById('cc_settings_overlay');
  if(!btn || !overlay) return;
  var errEl = document.getElementById('cc_settings_error');
  var saveBtn = document.getElementById('cc_s_save');

  btn.onclick = function(){ if(errEl) errEl.style.display='none'; overlay.style.display='flex'; };
  var cancelBtn = document.getElementById('cc_s_cancel');
  if(cancelBtn) cancelBtn.onclick = function(){ overlay.style.display='none'; };

  window.ccSettingsRebuildFailed = function(msg){
    document.body.style.opacity = '';
    if(saveBtn){ saveBtn.disabled = false; saveBtn.textContent = 'Save and reload'; }
    if(errEl){ errEl.textContent = msg || 'Rebuild failed after saving; your numbers were kept, try Refresh now.'; errEl.style.display = 'block'; }
  };

  if(saveBtn) saveBtn.onclick = function(){
    var num = function(id){ var v = parseFloat(document.getElementById(id).value); return isNaN(v) ? 0 : v; };
    var int = function(id){ var v = parseInt(document.getElementById(id).value, 10); return isNaN(v) ? 0 : v; };
    var payload = {
      yourSeat: document.getElementById('cc_s_yourSeat').value,
      defaultView: document.getElementById('cc_s_defaultView').value,
      monthlySubscriptionEUR: num('cc_s_subEUR'),
      monthlySubscriptionUSD: num('cc_s_subUSD'),
      seatsPurchased: int('cc_s_seatsTotal'),
      standardSeats: int('cc_s_seatsStd'),
      premiumSeats: int('cc_s_seatsPrem'),
      standardSeatPriceUSD: num('cc_s_priceStd'),
      premiumSeatPriceUSD: num('cc_s_pricePrem'),
      usageCreditsBalanceEUR: num('cc_s_creditsBal'),
      usageCreditsSpentEUR: num('cc_s_creditsSpent'),
      usageCreditsMonthlyCapEUR: num('cc_s_creditsCap'),
      companyConsumptionUSD: num('cc_s_companyUSD'),
      outputCostFactor: num('cc_s_factor'),
      calibratedOn: document.getElementById('cc_s_calibratedOn').value,
      window: document.getElementById('cc_s_window').value
    };
    saveBtn.disabled = true;
    saveBtn.textContent = 'Saving…';
    if(errEl) errEl.style.display = 'none';
    window.ccSaveSettings(payload).then(function(){
      saveBtn.textContent = 'Rebuilding…';
      document.body.style.opacity = '.6';
    }).catch(function(err){
      saveBtn.disabled = false;
      saveBtn.textContent = 'Save and reload';
      if(errEl){ errEl.textContent = String(err); errEl.style.display = 'block'; }
    });
  };
})();
</script>`
}

func toFileURL(path string) string {
	u := &url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(path)}
	return u.String()
}

// ---------------------------------------------------------------------------
// Data dir, logging
// ---------------------------------------------------------------------------

func appDataDir() string {
	if lad := os.Getenv("LOCALAPPDATA"); lad != "" {
		return filepath.Join(lad, "burnmon")
	}
	// Very unlikely on a managed Windows machine; fall back rather than
	// write next to a possibly read-only exe.
	return filepath.Join(os.TempDir(), "burnmon")
}

// setupLog opens the app's log file in append mode, rotating it out of the
// way first if it has grown past 512KB. Truncating on every start (the
// previous behaviour) destroyed the previous run's trail on every restart,
// including the "another instance already running" case, which exits before
// logging anything else: exactly the log a "my dashboard is stuck / shows no
// data" report needs to diagnose from app.log alone.
func setupLog(dataDir string) {
	path := filepath.Join(dataDir, "burnmon-app.log")
	if fi, err := os.Stat(path); err == nil && fi.Size() > 512*1024 {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(f)
}

// ---------------------------------------------------------------------------
// Single instance and the WebView2-missing fallback
// ---------------------------------------------------------------------------

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW         = user32.NewProc("FindWindowW")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

// alreadyRunning claims a named mutex for the life of the process. Windows
// reports ERROR_ALREADY_EXISTS when another instance already holds it, even
// though the returned handle is otherwise valid; that is the documented way
// to detect this without a helper class.
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

const (
	mbOK          = 0x00000000
	mbIconWarning = 0x00000030
)

// runWithoutWindow covers the very unlikely case of a managed machine
// missing the WebView2 runtime: collect once, open the result in the
// default browser, and say why there is no app window.
func runWithoutWindow(a *app) {
	if _, err := a.rebuild(true, nil); err != nil {
		log.Println("fallback collection failed:", err)
	}
	openInBrowser(a.htmlPath)
	showWebView2MissingMessage()
}

func openInBrowser(path string) {
	_ = exec.Command("cmd", "/c", "start", "", path).Start()
}

func showWebView2MissingMessage() {
	text, _ := windows.UTF16PtrFromString(
		"Claude Cost could not open its window because the WebView2 runtime is missing.\n" +
			"The dashboard was opened in your default browser instead. Installing the\n" +
			"WebView2 runtime (part of Microsoft Edge) will let the app window open normally.")
	caption, _ := windows.UTF16PtrFromString(windowTitle)
	_, _ = windows.MessageBox(0, text, caption, mbOK|mbIconWarning)
}
