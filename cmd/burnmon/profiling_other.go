//go:build !windows

package main

// processHandleCount: no portable equivalent of Windows's handle count on
// darwin/linux (B1's browser-mode build); WS3's shared-ingest investigation
// is Windows-only (the app window's live watcher and both exes running
// side by side), so this is a stub, not a gap.
func processHandleCount() int { return -1 }
