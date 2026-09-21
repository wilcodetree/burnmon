//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestAppRebuildNoticeNoWSL(t *testing.T) {
	got := appRebuildNotice(15*time.Minute, 4*time.Hour, nil)
	want := `<b>This window keeps itself current.</b> It opens straight to your last snapshot and quietly catches up in the background within moments. It also re-reads your session transcripts every 15 minutes while open, whenever you press Refresh now (top right), and right after you save changes in Settings (the gear icon).`
	if got != want {
		t.Errorf("appRebuildNotice with no WSL distros changed the byte-for-byte text.\ngot:  %s\nwant: %s", got, want)
	}
}

func TestAppRebuildNoticeWithWSL(t *testing.T) {
	got := appRebuildNotice(15*time.Minute, 4*time.Hour, []string{"Ubuntu"})
	for _, want := range []string{"15 minutes", "Ubuntu", "4 hours", "WSL included"} {
		if !strings.Contains(got, want) {
			t.Errorf("appRebuildNotice with WSL distros missing %q in: %s", want, got)
		}
	}

	multi := appRebuildNotice(15*time.Minute, 30*time.Minute, []string{"Ubuntu", "Debian"})
	if !strings.Contains(multi, "Ubuntu, Debian") {
		t.Errorf("appRebuildNotice did not join multiple distro names: %s", multi)
	}
	if !strings.Contains(multi, "30 minutes") {
		t.Errorf("appRebuildNotice did not format a sub-hour wsl interval in minutes: %s", multi)
	}
}
