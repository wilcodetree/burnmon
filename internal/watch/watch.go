// Package watch keeps burnmon's store current between the 15-minute full
// rescan ticks: a recursive watch on every native adapter root (one open
// directory handle per root on Windows, watch_windows.go; fsnotify,
// one watch per directory, everywhere else, watch_other.go), plus a
// 5-second poll for WSL roots, since fsnotify cannot watch a
// \\wsl.localhost path on Windows at all (confirmed on this laptop,
// SESSION_LOG.md, v0.1 Step 3: adding a watch on a live \\wsl.localhost
// folder fails immediately with "ReadDirectoryChanges: Incorrect
// function", not merely late).
package watch

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// pollInterval is how often a WSL root's .jsonl files are re-stat'd, per the
// spec's "WSL roots polled every 5 s".
const pollInterval = 5 * time.Second

// OnChange is called with a .jsonl file's path whenever it is created or
// written to, native or WSL. Called from a background goroutine; the
// receiver is responsible for its own synchronisation (dataset.Cache's
// ingest methods already serialise on the store).
type OnChange func(path string)

// nativeBackend is the OS-specific mechanism that watches every native
// (non-WSL) adapter root for new or changed .jsonl files, recursively.
// Windows (watch_windows.go): one recursive ReadDirectoryChangesW handle
// per root, added once and never touched again, since the recursion itself
// covers every subdirectory created under it from then on, at any depth.
// This replaced a one-fsnotify-watch-per-subdirectory design (WS3,
// 02_roadmap\2026-09-26_ws3_shared_ingest_performance.md, hypothesis 1):
// step 0 profiling found 13,125 of a real process's 13,541 open handles
// were exactly these per-directory watches, against roughly 12,700 Cowork
// session folders on Wilco's laptop. Everywhere else (watch_other.go, B1's
// browser-mode darwin/linux build, untested on real hardware and with no
// live app window to make the handle count visible): fsnotify, one watch
// per directory, unchanged from before WS3.
type nativeBackend interface {
	// AddRoot registers root (and, on the fsnotify backend, every
	// subdirectory under it) for live notifications. Called only at
	// construction, once per configured native root; a directory created
	// later under an already-added root is picked up without a further
	// call (the recursive backend, structurally; the fsnotify backend,
	// by watching for the Create event itself and adding the new
	// subdirectory in response, self-contained inside watch_other.go).
	AddRoot(root string) error
	// WatchCount is the number of OS-level watch handles currently open,
	// logged by cmd/burnmon's step 0 profiling switch.
	WatchCount() int
	// Close releases every handle this backend opened and blocks until its
	// own background goroutine(s) have exited.
	Close()
}

// Watcher watches every adapter root for new or changed .jsonl files.
// Zero value is not usable; construct with New.
type Watcher struct {
	native   nativeBackend
	onChange OnChange // used directly by the WSL poll loop; the native backend gets its own copy at construction

	wslMu      sync.Mutex
	wslRoots   []string
	wslMtime   map[string]time.Time // path -> last seen mtime, for the poll loop
	wslStarted bool

	stop chan struct{}
	wg   sync.WaitGroup
}

// New creates a Watcher over nativeRoots (watched live, recursively, via
// this OS's nativeBackend) and wslRoots (polled every 5 seconds; nil or
// empty is fine, e.g. when wsl_scan is off or no distro was found).
// onChange fires once per detected write, debounced only by the backend's
// own event coalescing.
func New(nativeRoots, wslRoots []string, onChange OnChange) (*Watcher, error) {
	native, err := newNativeBackend(onChange)
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		native:   native,
		onChange: onChange,
		wslRoots: append([]string(nil), wslRoots...),
		wslMtime: map[string]time.Time{},
		stop:     make(chan struct{}),
	}
	for _, root := range nativeRoots {
		// Startup only: the initial full backfill (cmd/burnmon's
		// a.rebuild, run right after this) ingests every existing file
		// under these roots itself, so this only needs to set up the
		// watch, not fire onChange for files it finds along the way.
		if err := native.AddRoot(root); err != nil {
			// Logged by the backend itself (a root missing, or a
			// permission-denied folder); a folder that cannot be watched
			// simply falls back to being caught by the 15-minute full
			// rescan, same as before this package existed.
			_ = err
		}
	}
	return w, nil
}

// Start runs the WSL poll loop in the background, if any WSL roots were
// given. The native backend's own event loop is already running by the
// time New returns. Safe to call once; call Stop to end both.
func (w *Watcher) Start() {
	w.wslMu.Lock()
	hasWSLRoots := len(w.wslRoots) > 0
	if hasWSLRoots {
		w.wslStarted = true
	}
	w.wslMu.Unlock()
	if hasWSLRoots {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			w.runWSLPoll()
		}()
	}
}

// Stop closes the native backend and stops the WSL poll loop, then waits
// for both to fully exit.
func (w *Watcher) Stop() {
	close(w.stop)
	w.native.Close()
	w.wg.Wait()
}

// WatchCount returns the number of OS-level watch handles the native
// backend currently holds open (one per root on Windows, one per directory
// elsewhere). Step 0 profiling (BURNMON_PPROF=1, cmd\burnmon\profiling.go)
// logs this alongside the process's total handle count.
func (w *Watcher) WatchCount() int {
	return w.native.WatchCount()
}

// AddWSLRoots extends an already-Started watcher with more WSL roots,
// starting the 5-second poll goroutine now if it was not already running
// (New was given no WSL roots, e.g. because they were not known yet). For
// cmd\burnmon: v0.1.1 F2 starts the live watcher with native roots only,
// ahead of the initial full backfill, so a Codex or Claude turn is never
// stuck waiting behind that backfill; once the backfill resolves the WSL
// roots (which does require the full pass, since probing a WSL distro is
// the slow operation the two-cadence design exists to gate), this adds
// them without tearing down or restarting the native side. A no-op for an
// empty roots.
func (w *Watcher) AddWSLRoots(roots []string) {
	if len(roots) == 0 {
		return
	}
	w.wslMu.Lock()
	w.wslRoots = append(w.wslRoots, roots...)
	startNeeded := !w.wslStarted
	if startNeeded {
		w.wslStarted = true
	}
	w.wslMu.Unlock()
	if startNeeded {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			w.runWSLPoll()
		}()
	}
}

// runWSLPoll re-walks every WSL root every pollInterval, comparing each
// .jsonl file's mtime against what was last seen. inotify does not cross the
// WSL boundary and fsnotify cannot even add a watch on a \\wsl.localhost
// path on this laptop (see the package doc), so this is the only mechanism
// for WSL transcripts.
func (w *Watcher) runWSLPoll() {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			w.pollWSLOnce()
		}
	}
}

func (w *Watcher) pollWSLOnce() {
	w.wslMu.Lock()
	roots := append([]string(nil), w.wslRoots...)
	w.wslMu.Unlock()
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(p), ".jsonl") {
				return nil
			}
			fi, statErr := d.Info()
			if statErr != nil {
				return nil
			}
			if prev, ok := w.wslMtime[p]; ok && !fi.ModTime().After(prev) {
				return nil
			}
			w.wslMtime[p] = fi.ModTime()
			w.onChange(p)
			return nil
		})
	}
}
