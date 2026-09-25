//go:build windows

// Command burnmon-dev is BurnMon Dev: a second WebView2 window, alongside
// burnmon.exe, that shows what running AI agents does to the token/cost
// side (burn) and the machine (system) on one screen, one time axis.
// 04_assets\2026-09-24_burnmon_dev_design.md is the design this follows.
// Phase 1 wired the system-side sampler and both viewports' empty panels;
// phase 2 starts the same live watch and adapter polls cmd\burnmon\app.go
// runs, into the same shared burnmon.db, and wires the burn zone (chart,
// session cards, vendor strip, turn ticker) against it. Phase 2b (this
// pass) adds the token-monitor items: headline running total, tok/min,
// the activity heatmap and cache-hit breakdown, design doc section 9.
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	webview2 "github.com/jchv/go-webview2"

	"burnmon/internal/adapter/codex"
	"burnmon/internal/devexport"
	"burnmon/internal/history"
	"burnmon/internal/live"
	"burnmon/internal/scan"
	"burnmon/internal/store"
	"burnmon/internal/sysmon"
	"burnmon/internal/vendorstrip"
)

//go:embed page.html
var pageHTML string

// progStart marks process entry, so every startupMark call below logs an
// elapsed time from the same zero point (section 11 step 1: "measure
// first" before changing any startup ordering). Read only from main's own
// goroutine before the window is up; the sampling goroutine's one
// first-sample mark (startSampling) reads it too, but only ever after
// main() has already set it, so no lock is needed.
var progStart = time.Now()

func startupMark(step string) {
	log.Printf("startup: %s at %v", step, time.Since(progStart).Round(time.Millisecond))
}

// heatmapPayload is bdevActivityHeatmap's return shape: renderHeatmap
// (page.html) only ever reads payload.rows, so this carries just that
// field rather than the rest of history.Payload (Totals, Vendors, Owners),
// which the merged closed+today rows below don't populate meaningfully.
type heatmapPayload struct {
	Rows []history.Row `json:"rows"`
}

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
	if len(os.Args) > 1 && os.Args[1] == "export" {
		os.Exit(runExport(os.Args[2:]))
		return
	}

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
	startupMark("store open and migrations done")

	cfg, cfgFile := loadConfig(dataDir)
	if cfgFile != "" {
		log.Println("using config overrides from", cfgFile)
	}

	devCfg := loadDevConfig(dataDir)

	a := &app{
		sys: sys, sampler: sysmon.NewSampler(), procSampler: sysmon.NewProcessSampler(),
		st: st, cfg: cfg, initialCollectDone: make(chan struct{}),
	}
	a.cache.Store = st
	a.startSampling()
	a.startRetentionPrune(devCfg.RetentionDays)

	nativeClaudeRoots := scan.DefaultSourcesWithOptions(false)
	nativeCodexRoots := codex.NativeSources()
	a.cache.SeedNativeRoots(nativeClaudeRoots, nativeCodexRoots)
	a.startLiveWatch(nativeClaudeRoots, nativeCodexRoots)
	startHermesPoll(a, st)
	startCopilotCLIPoll(a, st)
	startCopilotVSCPoll(a, st)
	startupMark("watcher and pollers started")
	a.startInitialCollect()
	go func() {
		<-a.initialCollectDone
		startupMark("initial collect and backfill done")
	}()

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
	startupMark("window created")

	// bdevMarkFirstRender: page.html calls this once, after its very first
	// poll cycle (sysmon, burn, vendor strip) has all resolved and painted,
	// so "first full render" is measured from the page's own perspective
	// rather than guessed at from the Go side.
	if err := w.Bind("bdevMarkFirstRender", func() {
		startupMark("first full render")
	}); err != nil {
		log.Println("could not bind bdevMarkFirstRender:", err)
	}

	// sysmonNowPayload adds the header pressure chip's score (internal/sysmon
	// .PressureScore, phase 3) and the CPU box's static base clock (UI review
	// patch section 6) alongside the raw sample: Sample is embedded
	// anonymously so every one of its fields still flattens into the same
	// JSON object the page already reads, plus these two extra keys.
	type sysmonNowPayload struct {
		sysmon.Sample
		PressureScore int     `json:"pressure_score"`
		BaseClockGHz  float64 `json:"base_clock_ghz"`
	}
	if err := w.Bind("bdevSysmonNow", func() (sysmonNowPayload, error) {
		a.mu.Lock()
		defer a.mu.Unlock()
		return sysmonNowPayload{
			Sample: a.latest, PressureScore: sysmon.PressureScore(a.latest),
			BaseClockGHz: sysmon.BaseClockGHz(),
		}, nil
	}); err != nil {
		log.Println("could not bind bdevSysmonNow:", err)
	}

	// bdevProcessGroupsNow: the process-groups table's current row per
	// harness, the latest 2s tick's sums (design doc section 3).
	if err := w.Bind("bdevProcessGroupsNow", func() ([]sysmon.ProcessGroupSample, error) {
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.latestGroups, nil
	}); err != nil {
		log.Println("could not bind bdevProcessGroupsNow:", err)
	}

	// bdevProcessGroupsHistory: the same table's per-harness sparklines and
	// the harness x minute heatmap both read from this one call (the page
	// buckets the raw 10s rows into 1-minute columns itself; the dataset is
	// small enough - a handful of harnesses times 6 rows/minute - that a
	// second Go-side aggregation buys nothing a client-side one doesn't
	// already do just as cheaply).
	if err := w.Bind("bdevProcessGroupsHistory", func(sinceMinutes int) ([]sysmon.ProcessGroupSample, error) {
		if sinceMinutes <= 0 {
			sinceMinutes = 60
		}
		since := time.Now().Add(-time.Duration(sinceMinutes) * time.Minute)
		rows, err := sys.RecentProcessGroups(since)
		if err != nil {
			log.Println("bdevProcessGroupsHistory:", err)
			return nil, err
		}
		return rows, nil
	}); err != nil {
		log.Println("could not bind bdevProcessGroupsHistory:", err)
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

	// bdevTurnDetail: the turn ticker's click-through popup (UI review patch
	// section 5, "the equivalent of burnmon.exe's turn drawer"), same
	// live.BuildTurnDetail cmd\burnmon\main.go's own bmTurn binding calls,
	// off the UI thread for the same reason bmTurn's own N1 fix gives (a
	// full-session EventsForSession re-read must not block the window from
	// responding to the drawer's own Close/Esc while it runs). turnDetailPayload
	// wraps the result with an Error string (found by review, 2026-09-25: the
	// original version silently sent back a zero-value TurnDetail on error,
	// which the popup then rendered as "Turn 0" / an invalid date instead of
	// surfacing the failure) rather than TurnDetail bare.
	type turnDetailPayload struct {
		live.TurnDetail
		Error string `json:"error,omitempty"`
	}
	if err := w.Bind("bdevTurnDetail", func(sessionID string, turn int) {
		go func() {
			a.mu.Lock()
			cfg := a.cfg
			a.mu.Unlock()
			detail, err := live.BuildTurnDetail(st, &cfg, sessionID, turn)
			payload := turnDetailPayload{TurnDetail: detail}
			if err != nil {
				log.Println("bdevTurnDetail:", sessionID, turn, ":", err)
				payload.Error = err.Error()
			}
			key := fmt.Sprintf("%s:%d", sessionID, turn)
			js, err := bdevAsyncResolveJSWithKey("__bdevTurnDetailResolve", key, payload)
			if err != nil {
				log.Println("bdevTurnDetail: encode result:", err)
				return
			}
			w.Dispatch(func() { w.Eval(js) })
		}()
	}); err != nil {
		log.Println("could not bind bdevTurnDetail:", err)
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

	// bdevActivityHeatmap: phase 2b's "N active days" grid, design doc
	// section 9. Wilco's own finding (2026-09-24): the original version
	// ran a fresh 182-day EventsSince plus history.Build on every 1-minute
	// poll, 3.34s for 52,728 rows against the real store, about 5.5 percent
	// of one core on average, alone over this workstream's 2 percent CPU
	// target (moving it off the UI thread, done in the same commit that
	// first added it, fixed the freeze but not that CPU cost). Every day
	// except today is closed, its events never change once the day is
	// over, so app.closedDayRows caches the 182-day pass and only
	// recomputes it once per UTC day (and only once the startup backfill
	// has finished, see closedDayRows' own comment); this binding now runs
	// that (cheap on every call except the rare day-rollover/pre-backfill
	// one) plus a fresh query for today alone (a single day's rows, not
	// 182), and merges the two. dayStart comes back from closedDayRows
	// itself rather than a second time.Now() call, so the two queries
	// can never straddle different UTC days (fixed after review). Runs
	// off the UI thread and resolves through bdevAsyncResolveJS for the
	// same reason as before: even the cheap path still touches the store.
	if err := w.Bind("bdevActivityHeatmap", func() {
		go func() {
			a.mu.Lock()
			cfg := a.cfg
			a.mu.Unlock()
			start := time.Now()

			closed, today, dayStart, cacheHit, err := a.closedDayRows(st, &cfg)
			if err != nil {
				log.Println("bdevActivityHeatmap: closedDayRows:", err)
				return
			}

			todayEvents, err := st.EventsSince(dayStart)
			if err != nil {
				log.Println("bdevActivityHeatmap: EventsSince(today):", err)
				return
			}
			todayPayload := history.Build(todayEvents, &cfg, history.Filter{Period: "day"})

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

			if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
				state := "closed cache miss, rebuilt"
				if cacheHit {
					state = "closed cache hit"
				}
				log.Printf("bdevActivityHeatmap: %v (%s, %d today rows scanned)", elapsed, state, len(todayEvents))
			}
			js, err := bdevAsyncResolveJS("__bdevActivityHeatmapResolve", heatmapPayload{Rows: rows})
			if err != nil {
				log.Println("bdevActivityHeatmap: encode result:", err)
				return
			}
			w.Dispatch(func() { w.Eval(js) })
		}()
	}); err != nil {
		log.Println("could not bind bdevActivityHeatmap:", err)
	}

	// bdevCacheBreakdown: phase 2b's click-to-expand cache-hit breakdown,
	// scoped to vendor-strip (harness) rows, design doc section 9. Today's
	// events for that vendor through the same history.Build, returning its
	// Totals (Fresh/CacheW/CacheR/Out) directly rather than a new struct.
	if err := w.Bind("bdevCacheBreakdown", func(vendor string) (history.Totals, error) {
		a.mu.Lock()
		cfg := a.cfg
		a.mu.Unlock()
		now := time.Now().UTC()
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		events, err := st.EventsSince(dayStart)
		if err != nil {
			return history.Totals{}, err
		}
		today := now.Format("2006-01-02")
		payload := history.Build(events, &cfg, history.Filter{Period: "day", From: today, To: today, Vendor: vendor})
		return payload.Totals, nil
	}); err != nil {
		log.Println("could not bind bdevCacheBreakdown:", err)
	}

	// bdevAdvisorNow (phase 4's "TODAY'S READ" panel) is gone: UI review
	// patch section 8 deletes the panel outright. The advisor rules
	// themselves (internal/advisor) stay, since buildExportBundle
	// (export_run.go) calls advisor.Analyze directly for summary.md,
	// independent of this binding.

	// bdevExport is the header EXPORT button (design doc "Export (AI-ready
	// bundle)"): the last 24 hours, not redacted (the CLI's --redact flag
	// covers the redacted case), written to dataDir\exports\. Shares
	// buildExportBundle (export_run.go) with the `export` CLI subcommand
	// so the two paths can never disagree on what an export contains.
	type exportResultPayload struct {
		OK            bool   `json:"ok"`
		Error         string `json:"error,omitempty"`
		Dir           string `json:"dir,omitempty"`
		SummaryBytes  int    `json:"summary_bytes,omitempty"`
		DataJSONBytes int    `json:"data_json_bytes,omitempty"`
		DailyCSVBytes int    `json:"daily_csv_bytes,omitempty"`
	}
	if err := w.Bind("bdevExport", func() {
		go func() {
			a.mu.Lock()
			cfg := a.cfg
			a.mu.Unlock()
			until := time.Now()
			since := until.Add(-24 * time.Hour)

			var payload exportResultPayload
			b, err := buildExportBundle(st, sys, &cfg, since, until)
			if err != nil {
				payload = exportResultPayload{OK: false, Error: err.Error()}
			} else {
				outDir := filepath.Join(dataDir, "exports")
				res, werr := devexport.Write(outDir, b)
				if werr != nil {
					payload = exportResultPayload{OK: false, Error: werr.Error()}
				} else {
					payload = exportResultPayload{
						OK: true, Dir: res.Dir,
						SummaryBytes: res.SummaryBytes, DataJSONBytes: res.DataJSONBytes, DailyCSVBytes: res.DailyCSVBytes,
					}
				}
			}
			js, jerr := bdevAsyncResolveJS("__bdevExportResolve", payload)
			if jerr != nil {
				log.Println("bdevExport: encode result:", jerr)
				return
			}
			w.Dispatch(func() { w.Eval(js) })
		}()
	}); err != nil {
		log.Println("could not bind bdevExport:", err)
	}

	startUICheckServer(w)

	w.SetHtml(pageHTML)
	w.Run()
}

// bdevAsyncResolveJS mirrors cmd\burnmon\main.go's own asyncResolveJS: a
// binding that returns immediately and finishes its real work in a
// goroutine hands the result to a page-side resolver this way instead of
// through go-webview2's own per-call Promise, which only resolves with
// whatever the bound function returns synchronously. encoding/json's
// default HTML escaping rules out a literal "</script>" in the marshalled
// JSON. No reqID here (unlike bmHistory): bdevActivityHeatmap is a fixed
// periodic poll with no filter to correlate against, so the latest
// resolved call simply overwrites whatever the page is showing.
func bdevAsyncResolveJS(resolverName string, data any) (string, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("window.%s && window.%s(%s)", resolverName, resolverName, dataJSON), nil
}

// bdevAsyncResolveJSWithKey mirrors cmd\burnmon\main.go's own asyncResolveJS:
// a keyed variant of bdevAsyncResolveJS for a binding more than one call to
// which can be in flight at once (bdevTurnDetail: a fast second ticker
// click before the first resolves), so the resolver can tell which call a
// result belongs to.
func bdevAsyncResolveJSWithKey(resolverName, key string, data any) (string, error) {
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
