package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// v3b: v0.3 V3-3b's real-window check for U3/U4 monitor mode. Self-managed
// (like w1): it needs to control burnmon.json before the very first launch
// (to prove "opens from a saved setting") and to restart burnmon.exe mid
// check (to prove the choice survives a restart), neither of which an
// already-running instance started by scripts\uicheck.ps1 would let it do.
func init() {
	checks["v3b"] = checkV3b
	selfManaged["v3b"] = true
}

// v3bCfgPath is exe-adjacent, the same "burnmon.json next to burnmon.exe"
// two-tier lookup cmd\burnmon\main.go tries first (portable/Groundwork Kit
// pattern), so this check's config actually wins over any %LOCALAPPDATA%
// one on this machine.
const v3bCfgPath = `..\..\burnmon.json`

func checkV3b(_ uintptr, _ []string) error {
	if h := findBurnmonWindowFast(); h != 0 && isRealBurnmonWindow(h) {
		return fmt.Errorf("burnmon.exe is already running (window %q); close it first so this check can control burnmon.json before launch", windowTitleText(h))
	}
	if _, err := os.Stat(v3bCfgPath); err == nil {
		return fmt.Errorf("%s already exists; this check writes and removes its own, move any real one aside first", v3bCfgPath)
	}
	if err := os.WriteFile(v3bCfgPath, []byte("{\n  \"view\": \"monitor\"\n}\n"), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", v3bCfgPath, err)
	}
	defer os.Remove(v3bCfgPath)

	hwnd, cmd, err := v3bLaunch()
	if err != nil {
		return err
	}
	defer v3bKill(cmd)

	// 1. Opens straight into monitor mode, dev (the compiled default), from
	// the saved "view":"monitor" setting: no header, no tab bar, the
	// real-text page shown instead of the normal Now page.
	v3bShot(hwnd, "v3b-monitor-dev-startup")
	startup, err := v3bReadState()
	if err != nil {
		return fmt.Errorf("read monitor-dev startup state: %w", err)
	}
	fmt.Printf("uicheck: monitor-dev startup state = %+v\n", startup)
	if startup.TopbarVis != "none" {
		return fmt.Errorf("#topbar display = %q at monitor-mode startup, want none (no header, and no tab bar: .tabsbar is #topbar's own child)", startup.TopbarVis)
	}
	if startup.TextViewVis != "block" {
		return fmt.Errorf("#monitor_text_view display = %q at monitor-mode dev startup, want block (real text, not the chart canvas)", startup.TextViewVis)
	}
	if startup.ExitBtnVis != "block" {
		return fmt.Errorf("#monitor_exit_btn display = %q in monitor mode, want block", startup.ExitBtnVis)
	}
	if !strings.Contains(startup.BodyClass, "monitor-dev") {
		return fmt.Errorf("body class = %q at dev-mode monitor startup, want monitor-dev present", startup.BodyClass)
	}

	// 2. Switch to full view with the small "Full view" switch (the header
	// and its dev/business toggle are hidden while in monitor mode, by
	// design: this switch is the only way out). Confirm the chrome is back.
	if err := v3bClick(hwnd, "#monitor_exit_btn"); err != nil {
		return fmt.Errorf("click #monitor_exit_btn: %w", err)
	}
	time.Sleep(500 * time.Millisecond)
	v3bShot(hwnd, "v3b-full-view")
	full, err := v3bReadState()
	if err != nil {
		return fmt.Errorf("read state after Full view: %w", err)
	}
	if strings.Contains(full.BodyClass, "monitor-mode") {
		return fmt.Errorf("body class = %q after clicking Full view, monitor-mode should be gone", full.BodyClass)
	}
	if full.TopbarVis == "none" {
		return fmt.Errorf("#topbar still hidden after clicking Full view")
	}

	// 3. From full view, flip to business mode, then back into monitor
	// mode: it should now show the normal visuals (the Now page's own
	// chart/cards/vendor strip) with euros, not the text page.
	if err := v3bClick(hwnd, "#btn_mode"); err != nil {
		return fmt.Errorf("click #btn_mode: %w", err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := v3bClick(hwnd, "#btn_monitor"); err != nil {
		return fmt.Errorf("click #btn_monitor: %w", err)
	}
	time.Sleep(1000 * time.Millisecond)
	v3bShot(hwnd, "v3b-monitor-business")
	business, err := v3bReadState()
	if err != nil {
		return fmt.Errorf("read business monitor state: %w", err)
	}
	if business.TextViewVis != "none" {
		return fmt.Errorf("#monitor_text_view display = %q in business monitor mode, want none (normal visuals, not text)", business.TextViewVis)
	}
	if business.NowVis == "none" {
		return fmt.Errorf("#now display = none in business monitor mode, want visible (it carries the normal chart/cards/vendor strip)")
	}
	if strings.Contains(business.BodyClass, "monitor-dev") {
		return fmt.Errorf("body class = %q in business monitor mode, monitor-dev should not apply", business.BodyClass)
	}
	if !strings.Contains(business.BodyClass, "monitor-mode") {
		return fmt.Errorf("body class = %q after clicking Monitor from business mode, want monitor-mode present", business.BodyClass)
	}

	// Back to full, then dev, then monitor again: leaves the window in the
	// dev-mode text page it started in, and exercises both switch buttons a
	// second time.
	if err := v3bClick(hwnd, "#monitor_exit_btn"); err != nil {
		return fmt.Errorf("click #monitor_exit_btn (from business monitor): %w", err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := v3bClick(hwnd, "#btn_mode"); err != nil {
		return fmt.Errorf("click #btn_mode (revert to dev): %w", err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := v3bClick(hwnd, "#btn_monitor"); err != nil {
		return fmt.Errorf("click #btn_monitor (back to dev monitor): %w", err)
	}
	time.Sleep(500 * time.Millisecond)
	backToMonitor, err := v3bReadState()
	if err != nil {
		return fmt.Errorf("read state after re-entering monitor mode: %w", err)
	}
	if !strings.Contains(backToMonitor.BodyClass, "monitor-mode") {
		return fmt.Errorf("body class = %q after clicking Monitor, want monitor-mode present", backToMonitor.BodyClass)
	}

	// 4. The live toggle (step 3's #btn_monitor click) must have saved back
	// to burnmon.json through the same path Settings uses; confirm on disk,
	// then restart and confirm the running window reads it back.
	cfgBytes, err := os.ReadFile(v3bCfgPath)
	if err != nil {
		return fmt.Errorf("read %s after the live Monitor toggle: %w", v3bCfgPath, err)
	}
	if !strings.Contains(string(cfgBytes), `"monitor"`) {
		return fmt.Errorf("%s does not hold view:monitor after the live toggle, got: %s", v3bCfgPath, cfgBytes)
	}

	v3bKill(cmd)
	hwnd2, cmd2, err := v3bLaunch()
	if err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	defer v3bKill(cmd2)
	restarted, err := v3bReadState()
	if err != nil {
		return fmt.Errorf("read state after restart: %w", err)
	}
	if !strings.Contains(restarted.BodyClass, "monitor-mode") {
		return fmt.Errorf("monitor mode not restored after restart, body class = %q", restarted.BodyClass)
	}
	v3bShot(hwnd2, "v3b-monitor-restart")
	return nil
}

// v3bShot re-asserts the window's topmost z-order (see v3bClick) before
// taking a screenshot, for the same reason: a BitBlt capture samples
// whatever DWM has actually composited at these screen coordinates, not
// this HWND specifically.
func v3bShot(hwnd uintptr, name string) {
	if err := ensureWindowSize(hwnd); err != nil {
		fmt.Printf("uicheck: could not re-assert window z-order before screenshot (non-fatal): %v\n", err)
	}
	if _, err := screenshot(hwnd, name); err != nil {
		fmt.Printf("uicheck: screenshot failed (non-fatal): %v\n", err)
	}
}

type v3bState struct {
	BodyClass   string `json:"bodyClass"`
	TopbarVis   string `json:"topbarVis"`
	ExitBtnVis  string `json:"exitBtnVis"`
	TextViewVis string `json:"textViewVis"`
	NowVis      string `json:"nowVis"`
}

func v3bReadState() (v3bState, error) {
	var s v3bState
	script := `(function(){
  function vis(sel){ var el=document.querySelector(sel); if(!el) return "missing"; return getComputedStyle(el).display; }
  return {
    bodyClass: document.body.className,
    topbarVis: vis('#topbar'),
    exitBtnVis: vis('#monitor_exit_btn'),
    textViewVis: vis('#monitor_text_view'),
    nowVis: vis('#now')
  };
})()`
	if err := evalInto(script, &s); err != nil {
		return s, err
	}
	return s, nil
}

// v3bLaunch starts burnmon.exe with its eval channel open, waits for the
// window to appear, then for the eval channel and the first Collect to
// finish (the app replaces its warming page with the real dashboard only
// once that first collection lands), mirroring scripts\uicheck.ps1's own
// readiness wait for a check that must manage its own process.
func v3bLaunch() (uintptr, *exec.Cmd, error) {
	logPath := os.Getenv("LOCALAPPDATA") + `\burnmon\burnmon-app.log`
	var before int
	if b, err := os.ReadFile(logPath); err == nil {
		before = strings.Count(string(b), "\n")
	}

	cmd := exec.Command(`..\..\burnmon.exe`)
	cmd.Env = envWithUICheck()
	if err := cmd.Start(); err != nil {
		return 0, nil, fmt.Errorf("start burnmon.exe: %w", err)
	}

	deadline := time.Now().Add(20 * time.Second)
	var hwnd uintptr
	for time.Now().Before(deadline) {
		if h := findBurnmonWindowFast(); h != 0 {
			hwnd = h
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if hwnd == 0 {
		_ = cmd.Process.Kill()
		return 0, nil, fmt.Errorf("no burnmon window appeared within 20s of launch")
	}
	if err := ensureWindowSize(hwnd); err != nil {
		_ = cmd.Process.Kill()
		return 0, nil, err
	}

	collectDeadline := time.Now().Add(90 * time.Second)
	collectDone := false
	for time.Now().Before(collectDeadline) {
		b, err := os.ReadFile(logPath)
		if err == nil {
			lines := strings.Split(string(b), "\n")
			if before <= len(lines) {
				lines = lines[before:]
			}
			if strings.Contains(strings.Join(lines, "\n"), "stage Collect (total)") {
				collectDone = true
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !collectDone {
		_ = cmd.Process.Kill()
		return 0, nil, fmt.Errorf("first Collect never finished within 90s; check %s", logPath)
	}

	// The reload/navigate to the real dashboard.html that follows Collect
	// is dispatched onto the UI thread, not synchronous with the log line
	// above; give the page's own script (which sets body's monitor-mode
	// class from D.view on load) a moment to actually run.
	readyDeadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(readyDeadline) {
		if _, err := evalString(`document.body.className`); err == nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)
	return hwnd, cmd, nil
}

// v3bClick re-asserts the window's topmost z-order immediately before
// clicking. main.go's own dispatch does this once, automatically, right
// before a non-self-managed check's fn runs (ensureWindowSize); a
// self-managed check gets no such help and has to call it itself. One call
// right after v3bLaunch is not enough here: this check's clicks happen long
// after that (past the 90s Collect wait, the config-driven monitor-mode
// startup, and earlier clicks/sleeps in this same run), by which point
// another window may have regained the top of the z-order, and a
// screen-coordinate click or BitBlt capture (screenshot, win32.go) would
// then hit that window instead, not burnmon's.
func v3bClick(hwnd uintptr, selector string) error {
	if err := ensureWindowSize(hwnd); err != nil {
		return fmt.Errorf("re-assert window z-order before clicking %s: %w", selector, err)
	}
	return clickSelector(hwnd, selector)
}

func v3bKill(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}
