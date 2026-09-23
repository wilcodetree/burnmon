//go:build !windows

// Command burnmon (darwin, linux; B1, 2026-09-23_v0.3_spec.md 2.4): a
// browser-mode build of the same dashboard.html the Windows WebView2 window
// shows. WebView2 itself is Windows-only (jchv/go-webview2 wraps the Win32
// runtime), so there is no live app window here, no bound JS functions
// (ccRefresh, bmLive, ...): a page opened this way already falls back to its
// own "only available in the BurnMon app window" text for anything live and
// otherwise reads exactly like a saved report, the same as
// runWithoutWindow's own fallback page does today on Windows when the
// WebView2 runtime itself is missing. Untested on real macOS/Linux
// hardware: built here (CGO_ENABLED=0, no cgo) and cross-compiled by
// build.ps1 and the release workflow from Windows.
package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"burnmon/internal/adapter/codex"
	"burnmon/internal/pricing"
	"burnmon/internal/scan"
	"burnmon/internal/store"
)

func main() {
	interval := flag.Duration("interval", 15*time.Minute, "how often to re-read transcripts and rewrite the dashboard")
	monthsN := flag.Int("months", 2, "how many months to include, counting the current one")
	seat := flag.String("seat", "Standard", "your own seat tier (Standard or Premium)")
	cfgPath := flag.String("config", "", "config file overriding the compiled-in prices and subscription (default: burnmon.json in the app's data folder, if present)")
	var sources multiFlag
	flag.Var(&sources, "source", "extra folder to scan; repeatable, overrides auto-detect")
	flag.Parse()

	if *interval < minInterval {
		*interval = minInterval
	}

	dataDir := appDataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return
	}
	setupLog(dataDir)
	log.Printf("burnmon %s starting, browser mode (%s/%s, untested)", version, runtime.GOOS, runtime.GOARCH)

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

	cfgSavePath := cfgFile
	if cfgSavePath == "" {
		cfgSavePath = filepath.Join(dataDir, "burnmon.json")
	}

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

	a := &app{
		cfg:      cfg,
		seat:     *seat,
		monthsN:  *monthsN,
		sources:  []string(sources),
		htmlPath: filepath.Join(dataDir, "dashboard.html"),
		interval: *interval,
		cfgPath:  cfgSavePath,
	}
	a.cache.Store = st

	nativeClaudeRoots := scan.DefaultSourcesWithOptions(false)
	nativeCodexRoots := codex.NativeSources()
	a.cache.SeedNativeRoots(nativeClaudeRoots, nativeCodexRoots)
	a.startLiveWatch(nativeClaudeRoots, nativeCodexRoots)
	startHermesPoll(a, st)
	startCopilotCLIPoll(a, st)
	startCopilotVSCPoll(a, st)

	if _, err := a.rebuild(true, nil); err != nil {
		log.Println("initial collection failed:", err)
		return
	}
	a.extendLiveWatchWSL(nativeClaudeRoots, nativeCodexRoots)
	openInBrowser(a.htmlPath)

	// Browser mode has no bound JS to push updates through, so the only way
	// the open tab ever shows fresher numbers is a manual reload; this loop
	// keeps dashboard.html itself current on disk so a reload (or the next
	// launch's own open) picks up new turns. No WSL ticker: scan's own
	// wsl_other.go stub already makes WSL detection a no-op off Windows.
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for range ticker.C {
		if _, err := a.rebuild(false, nil); err != nil {
			log.Println("rebuild failed:", err)
		}
	}
}

// openInBrowser opens path (a file:// URL, via toFileURL) in the platform's
// default browser: "open" on darwin, "xdg-open" on linux (both bundled with
// the OS, no extra dependency). Any other GOOS just logs the path, since
// CGO_ENABLED=0 cross-compilation is not restricted to these two.
func openInBrowser(path string) {
	url := toFileURL(path)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		log.Println("do not know how to open a browser on", runtime.GOOS, "; open manually:", path)
		return
	}
	if err := cmd.Start(); err != nil {
		log.Println("could not open the default browser:", err)
	}
}
