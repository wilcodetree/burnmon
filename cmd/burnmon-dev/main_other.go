//go:build !windows

// Command burnmon-dev is Windows-only: it exists for Wilco's own two
// screens (04_assets 2026-09-24_burnmon_dev_design.md), built with WebView2
// (jchv/go-webview2, Windows-only), unlike burnmon/burnmon-cli there is no
// portable macOS/Linux build to keep working. This stub only keeps
// `go build ./...`/`go vet ./...` runnable from a non-Windows machine.
package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	fmt.Fprintf(os.Stderr, "burnmon-dev is Windows-only; this is a %s/%s build with nothing to run.\n", runtime.GOOS, runtime.GOARCH)
	os.Exit(1)
}
