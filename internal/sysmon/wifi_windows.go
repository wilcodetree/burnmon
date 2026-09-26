//go:build windows

package sysmon

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// readWifi shells out to netsh (hidden, short timeout), ported from
// perfadvisor's own internal/tui/sysmetrics_windows.go (source commit
// perfadvisor main 2ed8046, design doc section 7). Label matching is
// deliberately loose so it survives localized Windows output. Not called
// every tick (see wifiRefreshEvery in sample_windows.go): a process
// shell-out on a 2s sampling cadence is not worth paying for every tick.
func readWifi() WifiSample {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "netsh", "wlan", "show", "interfaces")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return WifiSample{}
	}
	w := WifiSample{OK: true}
	for _, line := range strings.Split(string(out), "\n") {
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(line[:i]))
		value := strings.TrimSpace(line[i+1:])
		switch {
		case strings.Contains(label, "ssid") && !strings.Contains(label, "bssid") && w.SSID == "":
			w.SSID = value
		case strings.Contains(label, "sig") && strings.HasSuffix(value, "%"):
			if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(value, "%"))); err == nil {
				w.SignalPct = n
			}
		}
	}
	w.Connected = w.SSID != "" && w.SignalPct > 0
	return w
}
