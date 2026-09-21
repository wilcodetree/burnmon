// claudecost builds your own Claude usage and cost dashboard from the session
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
	"strings"
	"time"

	"claudecost/internal/dataset"
	"claudecost/internal/pricing"
	"claudecost/internal/report"
)

const version = "0.8.1"

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ", ") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func main() { os.Exit(run()) }

func run() int {
	monthsN := flag.Int("months", 2, "how many months to include, counting the current one")
	seat := flag.String("seat", "Standard", "your own seat tier (Standard or Premium)")
	var sources multiFlag
	flag.Var(&sources, "source", "extra folder to scan; repeatable, overrides auto-detect")
	outDir := flag.String("out", "", "report folder (default: reports next to the exe)")
	jsonOut := flag.String("json", "", "also write the raw dataset to this JSON path")
	cfgPath := flag.String("config", "", "config file (default: claudecost.json next to the exe, if present)")
	noOpen := flag.Bool("no-open", false, "do not open the report in the browser")
	quiet := flag.Bool("quiet", false, "suppress progress output")
	noCache := flag.Bool("no-cache", false, "skip the parse cache entirely: always re-parse "+
		"every transcript, and do not write the cache file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("claudecost " + version)
		return 0
	}

	cfg, err := loadConfig(*cfgPath, *quiet)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		return 1
	}

	// An explicit -seat wins; otherwise a your_seat set in claudecost.json
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
	// call; the disk-persisted parse cache below is what carries the benefit
	// across runs, same file the app binary also reads and writes.
	var cache dataset.Cache
	cachePath := filepath.Join(cacheDir(), "parsecache.gob")
	fingerprint := dataset.Fingerprint(version, &cfg)
	if !*noCache {
		cache.Load(cachePath, fingerprint)
	}
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
	}
	payload, err := cache.Collect(&cfg, opts, progress)
	if err == nil && !*noCache {
		if mkErr := os.MkdirAll(filepath.Dir(cachePath), 0o755); mkErr == nil {
			if saveErr := cache.Save(cachePath, fingerprint); saveErr != nil && !*quiet {
				fmt.Fprintln(os.Stderr, "warning: could not save parse cache:", saveErr)
			}
		}
	}
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
	htmlPath := filepath.Join(dir, "claudecost-report-"+time.Now().Format("20060102-150405")+".html")
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
			cand := filepath.Join(filepath.Dir(exe), "claudecost.json")
			if _, err := os.Stat(cand); err == nil {
				path = cand
			}
		}
	}
	if path == "" {
		if _, err := os.Stat("claudecost.json"); err == nil {
			path = "claudecost.json"
		}
	}
	cfg, err := pricing.Load(path)
	if err == nil && path != "" && !quiet {
		fmt.Println("Using config overrides from", path)
	}
	return cfg, err
}

// cacheDir is the same %LOCALAPPDATA%\claudecost folder the app binary uses
// for its own data, cache and log, so the CLI and the app share one parse
// cache instead of each keeping a separate copy.
func cacheDir() string {
	if lad := os.Getenv("LOCALAPPDATA"); lad != "" {
		return filepath.Join(lad, "claudecost")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claudecost")
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
		return filepath.Join(lad, "claudecost", "reports")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claudecost", "reports")
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
