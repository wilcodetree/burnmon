//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestUIThemeRoundTrip: what saveUITheme writes, loadUITheme reads back,
// from burnmon-dev-view.json and nowhere else.
func TestUIThemeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if got := loadUITheme(dir); got != "dark" {
		t.Fatalf("no file: %q, want dark", got)
	}
	for _, theme := range []string{"light", "dark", "light"} {
		if err := saveUITheme(dir, theme); err != nil {
			t.Fatalf("save %s: %v", theme, err)
		}
		if got := loadUITheme(dir); got != theme {
			t.Fatalf("after saving %s, loaded %q", theme, got)
		}
	}
	if filepath.Base(viewStatePath(dir)) != "burnmon-dev-view.json" {
		t.Fatalf("view file is %s", viewStatePath(dir))
	}
	if _, err := os.Stat(filepath.Join(dir, "burnmon-dev.json")); err == nil {
		t.Fatal("saving the view state touched burnmon-dev.json")
	}
}

// TestUIThemeClamping: only "light" and "dark" are saved, anything else is
// rejected and leaves the file alone; a bad or unparsable file means dark.
func TestUIThemeClamping(t *testing.T) {
	dir := t.TempDir()
	if err := saveUITheme(dir, "light"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "Light", "blue", `x"};alert(1)`} {
		if err := saveUITheme(dir, bad); err == nil {
			t.Errorf("save %q: want an error", bad)
		}
		if got := loadUITheme(dir); got != "light" {
			t.Errorf("save %q changed the stored theme to %q", bad, got)
		}
	}
	for name, body := range map[string]string{"unparsable": `{"ui_theme": `, "unknown": `{"ui_theme": "blue"}`, "absent key": `{}`} {
		if err := os.WriteFile(viewStatePath(dir), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := loadUITheme(dir); got != "dark" {
			t.Errorf("%s: loaded %q, want dark", name, got)
		}
	}
}
