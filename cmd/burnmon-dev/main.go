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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	webview2 "github.com/jchv/go-webview2"

	"burnmon/internal/adapter/codex"
	"burnmon/internal/devexport"
	"burnmon/internal/history"
	"burnmon/internal/live"
	"burnmon/internal/scan"
	"burnmon/internal/store"
	"burnmon/internal/sysmon"
	"burnmon/internal/todo"
	"burnmon/internal/vendorstrip"
)

//go:embed page.html
var pageHTML string

// Microsoft To Do panel payloads (section 9). Kept small and purpose-built
// per call rather than reusing internal/todo's own structs directly: the
// unexported deviceCode/interval/expiresIn fields on todo.DeviceCodeInfo
// never need to reach the page at all (main.go keeps the whole struct
// server-side to hand to FinishLogin).
type todoStatusPayload struct {
	Enabled  bool `json:"enabled"`
	SignedIn bool `json:"signed_in"`
}

type todoLoginStartPayload struct {
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	Message         string `json:"message"`
}

type todoLoginDonePayload struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type todoTasksPayload struct {
	Items []todo.Item `json:"items"`
	Error string      `json:"error,omitempty"`
}

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

	// Window, section 10: first launch is a normal 1280x860 window,
	// centered; every launch after that remembers size, position, maximized
	// state and monitor, restoring them only if that monitor still exists
	// (windowstate.go). WindowOptions has no X/Y field, so a restored
	// window is created at this same default size/center and then
	// immediately repositioned via applyWindowState below; a very brief
	// visible jump to the default rect is an accepted tradeoff of
	// go-webview2's WebView interface having no "create hidden, then show"
	// hook to avoid it.
	savedState, hasSavedState := loadWindowState(dataDir)
	initialWidth, initialHeight := uint(1280), uint(860)
	if hasSavedState {
		initialWidth, initialHeight = uint(savedState.Width), uint(savedState.Height)
	}
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		DataPath: wv2Dir,
		WindowOptions: webview2.WindowOptions{
			Title:  windowTitle,
			Width:  initialWidth,
			Height: initialHeight,
			Center: true,
			IconId: 1, // matches the "#1" icon group winres/burnmon-dev.json embeds
		},
	})
	if w == nil {
		log.Println("could not create the WebView2 window; BurnMon Dev needs the WebView2 runtime")
		return
	}
	hwnd := uintptr(w.Window())
	if hasSavedState {
		applyWindowState(hwnd, savedState)
	}

	// F11 toggles fullscreen, Esc also leaves it (section 10). fs is this
	// window's own fullscreen state (windowstate.go), declared before
	// startWindowStateSaver so that goroutine can skip saving while
	// fullscreen (its own doc comment explains why). Esc reaches
	// bdevExitFullscreen through the page's own keydown handler, same as any
	// other binding; F11 does not, since WebView2/Chromium reserves it as a
	// default browser accelerator key and never lets the DOM see it at all
	// (found empirically this session) - installF11Hotkey works around that
	// with a global hotkey plus a thread-local message hook instead (see its
	// own doc comment, windowstate.go), so bdevToggleFullscreen below is
	// only reachable from the uicheck dev eval channel, not from a real F11
	// keypress.
	fs := &fullscreenState{}
	startWindowStateSaver(hwnd, dataDir, fs)
	startupMark("window created")

	installF11Hotkey(hwnd, fs)
	if err := w.Bind("bdevToggleFullscreen", func() {
		toggleFullscreen(hwnd, fs)
	}); err != nil {
		log.Println("could not bind bdevToggleFullscreen:", err)
	}
	if err := w.Bind("bdevExitFullscreen", func() {
		exitFullscreen(hwnd, fs)
	}); err != nil {
		log.Println("could not bind bdevExitFullscreen:", err)
	}

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

	// Microsoft To Do panel (section 9), off unless burnmon-dev.json sets
	// microsoft_todo_enabled. Read-only, own token cache under
	// %LOCALAPPDATA%\burnmon\ (internal/todo's own doc comment), never
	// touched by the export bundle or any log line - task titles are
	// personal data the design doc's "never in exports, logs or
	// screenshots" line covers by this package simply never being imported
	// from that path, not by a redaction rule.
	if err := w.Bind("bdevTodoStatus", func() todoStatusPayload {
		return todoStatusPayload{Enabled: devCfg.MicrosoftTodoEnabled, SignedIn: todo.SignedIn()}
	}); err != nil {
		log.Println("could not bind bdevTodoStatus:", err)
	}

	// bdevTodoLogin: StartLogin is one fast HTTP call, so this returns
	// synchronously with the code/URL to show; the browser opens
	// immediately, and FinishLogin's own slow polling loop (the
	// interactive "type this code in your browser" wait) runs in a
	// goroutine, resolving through bdevAsyncResolveJS once the user
	// actually approves it (or it times out), the same pattern every other
	// slow bdev* binding in this file already uses.
	if err := w.Bind("bdevTodoLogin", func() (todoLoginStartPayload, error) {
		if !devCfg.MicrosoftTodoEnabled {
			return todoLoginStartPayload{}, fmt.Errorf("Microsoft To Do is not enabled")
		}
		info, err := todo.StartLogin()
		if err != nil {
			return todoLoginStartPayload{}, err
		}
		openBrowser(info.VerificationURI)
		go func() {
			loginErr := todo.FinishLogin(info)
			payload := todoLoginDonePayload{OK: loginErr == nil}
			if loginErr != nil {
				payload.Error = loginErr.Error()
			}
			js, jerr := bdevAsyncResolveJS("__bdevTodoLoginResolve", payload)
			if jerr != nil {
				log.Println("bdevTodoLogin: encode result:", jerr)
				return
			}
			w.Dispatch(func() { w.Eval(js) })
		}()
		return todoLoginStartPayload{UserCode: info.UserCode, VerificationURI: info.VerificationURI, Message: info.Message}, nil
	}); err != nil {
		log.Println("could not bind bdevTodoLogin:", err)
	}

	// bdevTodoTasks: TodayTasks makes one or more Graph HTTP calls, off the
	// UI thread for the same reason every other network/store-touching
	// binding here is.
	if err := w.Bind("bdevTodoTasks", func() {
		if !devCfg.MicrosoftTodoEnabled {
			return
		}
		go func() {
			items, err := todo.TodayTasks()
			payload := todoTasksPayload{Items: items}
			if err != nil {
				payload.Error = err.Error()
			}
			js, jerr := bdevAsyncResolveJS("__bdevTodoTasksResolve", payload)
			if jerr != nil {
				log.Println("bdevTodoTasks: encode result:", jerr)
				return
			}
			w.Dispatch(func() { w.Eval(js) })
		}()
	}); err != nil {
		log.Println("could not bind bdevTodoTasks:", err)
	}

	startUICheckServer(w)

	w.SetHtml(pageHTML)
	w.Run()
}

// openBrowser opens url in the default browser (cmd\burnmon-cli\main.go's
// own Windows branch, ported here since burnmon-dev is Windows-only and has
// no such helper of its own yet). Only ever called with Microsoft's own
// device-code verification_uri (internal/todo.StartLogin), never anything
// page- or user-supplied, but still checked for an https scheme before
// shelling out (found by review, 2026-09-25: cheap, and rules out this
// call site ever being repurposed later with a less trusted URL).
// HideWindow: burnmon-dev.exe is built -H windowsgui, so without it `cmd`
// can still flash a console window.
func openBrowser(rawURL string) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		log.Println("openBrowser: refusing a non-https URL:", rawURL)
		return
	}
	cmd := exec.Command("cmd", "/c", "start", "", rawURL)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		log.Println("openBrowser: could not start:", err)
	}
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
