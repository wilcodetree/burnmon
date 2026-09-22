// burnmon-cli builds your own Claude usage and cost dashboard from the session
// transcripts Cowork and Claude Code already write on this machine.
//
// Standalone port of claude_usage_extract.py (schema 1) plus the dashboard
// template of the Cowork "Claude Cost (User)" artifact. Nothing leaves your
// computer: no API key, no admin rights, no network calls.
//
// This is deliberately personal. It reads only your own transcript folders
// and refuses no one, because there is nothing to refuse: other people's
// usage is simply not on your disk.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"burnmon/internal/dataset"
	"burnmon/internal/live"
	"burnmon/internal/pricing"
	"burnmon/internal/report"
	"burnmon/internal/store"
)

const version = "0.1.2"

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ", ") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "price-check" {
		os.Exit(runPriceCheck(os.Args[2:]))
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "live" {
		os.Exit(runLive(os.Args[2:]))
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "reown" {
		os.Exit(runReown(os.Args[2:]))
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "tools" {
		os.Exit(runTools(os.Args[2:]))
		return
	}
	os.Exit(run())
}

// runLive does one full Collect pass (so the store is as current as a
// one-shot process can make it, same as `report`), then prints the Now
// page's JSON snapshot once: "for tests and for a later TUI" (v0.1 spec,
// Step 3).
func runLive(args []string) int {
	fs := flag.NewFlagSet("live", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file (default: burnmon.json next to the exe, if present)")
	jsonOut := fs.Bool("json", false, "print the snapshot as JSON (the only supported form today)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg, err := loadConfig(*cfgPath, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		return 1
	}

	storePath, err := store.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "internal error:", err)
		return 1
	}
	st, err := store.Open(storePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not open the local store at", storePath, ":", err)
		return 1
	}
	defer st.Close()

	cache := dataset.Cache{Store: st}
	opts := dataset.CollectOpts{Seat: "Standard", MonthsN: 1, RefreshSlow: true}
	if _, err := cache.Collect(&cfg, opts, nil); err != nil {
		var seatErr *dataset.SeatError
		if !errors.As(err, &seatErr) && !errors.Is(err, dataset.ErrNoSessions) {
			fmt.Fprintln(os.Stderr, "could not collect:", err)
			return 1
		}
		// No sessions inside the reporting window is fine here: the store
		// may still hold events (an old session), or none at all, either
		// way the snapshot below is what actually answers "is anything
		// running now".
	}

	// S3: the same windowed query and SessionTotals path the app uses (F1),
	// not AllEvents, which re-groups the whole store on every call.
	now := time.Now()
	events, err := st.EventsSince(now.Add(-live.ChartWindow))
	if err != nil {
		fmt.Fprintln(os.Stderr, "internal error:", err)
		return 1
	}
	snap := live.BuildSnapshot(events, &cfg, now)
	if err := live.ApplySessionTotals(snap.Sessions, st, &cfg); err != nil {
		fmt.Fprintln(os.Stderr, "internal error:", err)
		return 1
	}

	if !*jsonOut {
		fmt.Fprintln(os.Stderr, "live currently only supports -json")
	}
	b, err := json.MarshalIndent(snap, "", " ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "internal error:", err)
		return 1
	}
	fmt.Println(string(b))
	return 0
}

// runReown re-applies burnmon.json's owner rules (P6) to every event already
// in the store, for after an owner rule change: without this, an existing
// session keeps whatever owner ingest gave it at the time, even once the
// rules that produced it are edited.
func runReown(args []string) int {
	fs := flag.NewFlagSet("reown", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file (default: burnmon.json next to the exe, if present)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg, err := loadConfig(*cfgPath, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		return 1
	}

	storePath, err := store.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "internal error:", err)
		return 1
	}
	st, err := store.Open(storePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not open the local store at", storePath, ":", err)
		return 1
	}
	defer st.Close()

	n, err := st.ReownEvents(cfg.OwnerFor)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not reown events:", err)
		return 1
	}
	fmt.Printf("reowned %d event(s)\n", n)
	return 0
}

// parseSinceDuration parses `tools`'s -since flag: a bare integer with a "d"
// suffix (30d) alongside anything time.ParseDuration already accepts, since
// Go's own duration parser has no day unit.
func parseSinceDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("bad day count in %q: %w", s, err)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// runTools prints S2's tool_calls totals since the given lookback window:
// tool name, calls, sessions, and input/result bytes, the same
// store.ToolCallTotals query a later Tools tab will read. Does a full
// Collect pass first (same as live and price-check) so the store is as
// current as a one-shot process can make it.
func runTools(args []string) int {
	fs := flag.NewFlagSet("tools", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file (default: burnmon.json next to the exe, if present)")
	since := fs.String("since", "30d", "lookback window, e.g. 30d, 7d, 24h")
	jsonOut := fs.Bool("json", false, "print as JSON instead of a table")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	dur, err := parseSinceDuration(*since)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid -since:", err)
		return 1
	}

	cfg, err := loadConfig(*cfgPath, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		return 1
	}

	storePath, err := store.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "internal error:", err)
		return 1
	}
	st, err := store.Open(storePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not open the local store at", storePath, ":", err)
		return 1
	}
	defer st.Close()

	cache := dataset.Cache{Store: st}
	opts := dataset.CollectOpts{Seat: "Standard", MonthsN: 1, RefreshSlow: true}
	if _, err := cache.Collect(&cfg, opts, nil); err != nil {
		var seatErr *dataset.SeatError
		if !errors.As(err, &seatErr) && !errors.Is(err, dataset.ErrNoSessions) {
			fmt.Fprintln(os.Stderr, "could not collect:", err)
			return 1
		}
	}

	rows, err := st.ToolCallTotals(time.Now().Add(-dur))
	if err != nil {
		fmt.Fprintln(os.Stderr, "internal error:", err)
		return 1
	}

	if *jsonOut {
		b, err := json.MarshalIndent(rows, "", " ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "internal error:", err)
			return 1
		}
		fmt.Println(string(b))
		return 0
	}

	fmt.Printf("%-28s %8s %8s %14s %14s\n", "tool", "calls", "sessions", "input bytes", "result bytes")
	for _, r := range rows {
		fmt.Printf("%-28s %8d %8d %14d %14d\n", r.Tool, r.Calls, r.Sessions, r.InputBytes, r.ResultBytes)
	}
	return 0
}

// runPriceCheck prints every compiled-in (or burnmon.json-overridden) price
// book with the date it was last checked, per the v0.1 Step 2 done-when:
// "price-check CLI command prints every book with its date."
func runPriceCheck(args []string) int {
	fs := flag.NewFlagSet("price-check", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file (default: burnmon.json next to the exe, if present)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg, err := loadConfig(*cfgPath, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		return 1
	}

	fmt.Println("Anthropic price book, dated", pricing.AnthropicPriceBookDate)
	for _, fam := range cfg.Families() {
		p := cfg.Prices[fam]
		fmt.Printf("  %-10s in %6.2f  out %6.2f  USD/MTok\n", p.Label, p.In, p.Out)
	}

	fmt.Println()
	fmt.Println("OpenAI price book, dated", pricing.OpenAIPriceBookDate)
	models := make([]string, 0, len(cfg.OpenAIPrices))
	for m := range cfg.OpenAIPrices {
		models = append(models, m)
	}
	sort.Strings(models)
	for _, m := range models {
		p := cfg.OpenAIPrices[m]
		fmt.Printf("  %-20s (%-14s) in %6.3f  cached-in %6.3f  out %6.3f  USD/MTok\n",
			m, p.Label, p.In, p.CachedIn, p.Out)
	}
	return 0
}

func run() int {
	monthsN := flag.Int("months", 2, "how many months to include, counting the current one")
	seat := flag.String("seat", "Standard", "your own seat tier (Standard or Premium)")
	var sources multiFlag
	flag.Var(&sources, "source", "extra folder to scan; repeatable, overrides auto-detect")
	outDir := flag.String("out", "", "report folder (default: reports next to the exe)")
	jsonOut := flag.String("json", "", "also write the raw dataset to this JSON path")
	cfgPath := flag.String("config", "", "config file (default: burnmon.json next to the exe, if present)")
	noOpen := flag.Bool("no-open", false, "do not open the report in the browser")
	quiet := flag.Bool("quiet", false, "suppress progress output")
	noCache := flag.Bool("no-cache", false, "re-read every transcript from the start, "+
		"ignoring the store's cursors")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("burnmon-cli " + version)
		return 0
	}

	cfg, err := loadConfig(*cfgPath, *quiet)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		return 1
	}

	// An explicit -seat wins; otherwise a your_seat set in burnmon.json
	// wins over the flag's own "Standard" default, so a shared config for a
	// team that is all on one tier (see the Groundwork Kit) never needs
	// -seat passed by hand. flag.Visit is the only way to tell "passed at
	// its default value" from "not passed at all".
	effectiveSeat := *seat
	seatFlagSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "seat" {
			seatFlagSet = true
		}
	})
	if !seatFlagSet && cfg.Subscription.YourSeat != "" {
		effectiveSeat = cfg.Subscription.YourSeat
	}

	// The CLI runs once and exits, so this Cache only lives for one Collect
	// call; the store on disk is what carries the benefit across runs, same
	// file the app binary also reads and writes.
	storePath, err := store.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "internal error:", err)
		return 1
	}
	st, err := store.Open(storePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not open the local store at", storePath, ":", err)
		return 1
	}
	defer st.Close()

	// Best-effort cleanup of the old gob-based parse cache, replaced by the
	// SQLite store above; ignore any error, missing is the expected case on
	// every run after the first.
	_ = os.Remove(filepath.Join(filepath.Dir(storePath), "parsecache.gob"))

	cache := dataset.Cache{Store: st}
	progress := func(done, total int) {
		if *quiet {
			return
		}
		if done == 0 {
			fmt.Printf("Scanning %d folder(s), found %d session files.\n", len(cache.Sources), total)
			return
		}
		if done%200 == 0 {
			fmt.Printf("  ...%d/%d\n", done, total)
		}
	}

	// A one-shot run always does a full pass: there is no ticker here to
	// spread the WSL cost over, so RefreshSlow is always true. See "Two
	// refresh cadences" in docs/2026-08-17_wsl-source-detection-design.md,
	// which only applies to the app's own interval tickers.
	opts := dataset.CollectOpts{
		Seat:        effectiveSeat,
		MonthsN:     *monthsN,
		Sources:     []string(sources),
		RefreshSlow: true,
		ForceFull:   *noCache,
	}
	payload, err := cache.Collect(&cfg, opts, progress)
	if err != nil {
		var seatErr *dataset.SeatError
		switch {
		case errors.As(err, &seatErr):
			fmt.Fprintf(os.Stderr, "unknown seat tier %q, valid: %s\n", seatErr.Seat, strings.Join(seatErr.Valid, ", "))
			return 1
		case errors.Is(err, dataset.ErrNoSources):
			fmt.Fprintln(os.Stderr, "No Claude transcript folders found on this machine.")
			fmt.Fprintln(os.Stderr, "If you only use Claude in the browser, there is nothing to read:")
			fmt.Fprintln(os.Stderr, "those chats keep no local transcript. See claude.ai/settings/usage instead.")
			return 2
		case errors.Is(err, dataset.ErrNoFiles):
			fmt.Fprintln(os.Stderr, "No session transcripts found.")
			return 2
		case errors.Is(err, dataset.ErrNoSessions):
			fmt.Fprintln(os.Stderr, "No sessions inside the reporting window; nothing to show.")
			return 2
		default:
			fmt.Fprintln(os.Stderr, "internal error:", err)
			return 1
		}
	}

	if *jsonOut != "" {
		b, err := json.MarshalIndent(payload, "", " ")
		if err == nil {
			err = os.WriteFile(*jsonOut, b, 0o644)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "could not write", *jsonOut, ":", err)
			return 1
		}
		if !*quiet {
			fmt.Println("Wrote", *jsonOut)
		}
	}

	blob, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintln(os.Stderr, "internal error:", err)
		return 1
	}

	dir := *outDir
	if dir == "" {
		dir = defaultReportDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "cannot create report folder", dir, ":", err)
		return 1
	}
	htmlPath := filepath.Join(dir, "burnmon-report-"+time.Now().Format("20060102-150405")+".html")
	if err := report.Write(htmlPath, blob); err != nil {
		fmt.Fprintln(os.Stderr, "could not write report:", err)
		return 1
	}

	printSummary(&cfg, payload, htmlPath, *quiet)

	if !*noOpen {
		openBrowser(htmlPath)
	}
	return 0
}

func loadConfig(explicit string, quiet bool) (pricing.Config, error) {
	path := explicit
	if path == "" {
		if exe, err := os.Executable(); err == nil {
			cand := filepath.Join(filepath.Dir(exe), "burnmon.json")
			if _, err := os.Stat(cand); err == nil {
				path = cand
			}
		}
	}
	if path == "" {
		if _, err := os.Stat("burnmon.json"); err == nil {
			path = "burnmon.json"
		}
	}
	cfg, err := pricing.Load(path)
	if err == nil && path != "" && !quiet {
		fmt.Println("Using config overrides from", path)
	}
	return cfg, err
}

func defaultReportDir() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "reports")
		if os.MkdirAll(dir, 0o755) == nil {
			probe := filepath.Join(dir, ".probe")
			if f, err := os.Create(probe); err == nil {
				f.Close()
				os.Remove(probe)
				return dir
			}
		}
	}
	if lad := os.Getenv("LOCALAPPDATA"); lad != "" {
		return filepath.Join(lad, "burnmon", "reports")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".burnmon", "reports")
}

func openBrowser(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	_ = cmd.Start()
}

// ---------------------------------------------------------------------------
// Terminal summary
// ---------------------------------------------------------------------------

func printSummary(cfg *pricing.Config, p dataset.Payload, htmlPath string, quiet bool) {
	if quiet {
		fmt.Println(htmlPath)
		return
	}

	eurPerUSD := cfg.FXUSDEUR
	if cfg.Subscription.MonthlySubscriptionUSD > 0 {
		eurPerUSD = cfg.Subscription.MonthlySubscriptionEUR / cfg.Subscription.MonthlySubscriptionUSD
	}

	t := p.Totals
	fmt.Println()
	fmt.Printf("  window    %s to %s\n", p.WindowFrom, p.WindowTo)
	fmt.Printf("  sessions  %d, API calls %d, tokens %.2fB\n", t.Sessions, t.Calls, float64(t.Tokens)/1e9)
	fmt.Printf("  subscription cost EUR %.2f (USD %.2f)   API list equivalent USD %.2f\n",
		t.CostSub*eurPerUSD, t.CostSub, t.Cost)
	fmt.Println()
	fmt.Println("  month      sub EUR    sub USD    list USD    calls   tokens  sessions")
	keys := make([]string, 0, len(p.Months))
	for k := range p.Months {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		m := p.Months[k]
		fmt.Printf("  %s  %9.2f  %9.2f  %10.2f  %7d  %6.2fB  %8d\n",
			k, m.CostSub*eurPerUSD, m.CostSub, m.Cost, m.Calls, float64(m.Tokens)/1e9, m.Sessions)
	}

	// The single most useful observation: cost share sitting in marathon sessions.
	var marathon float64
	for _, s := range p.Sessions {
		if s.Long {
			marathon += s.CostSub
		}
	}
	if t.CostSub > 0 {
		share := marathon / t.CostSub
		if share > 0.45 {
			fmt.Println()
			fmt.Printf("  Note: %.0f%% of your cost sits in sessions past %d calls. An agent session\n",
				share*100, p.LongSessionCalls)
			fmt.Println("  re-sends its whole context every turn, so per-call cost climbs steeply as a")
			fmt.Println("  session grows. Finishing a topic and starting a new session is the fix,")
			fmt.Println("  not switching to a cheaper model.")
		}
	}

	fmt.Println()
	fmt.Println("  Report:", htmlPath)
	fmt.Println("  These are allocated shares of a flat monthly invoice, not money owed.")
}
