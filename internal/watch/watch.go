// Package watch keeps burnmon's store current between the 15-minute full
// rescan ticks: fsnotify on every native adapter root, a 5-second poll for
// WSL roots, since fsnotify cannot watch a \\wsl.localhost path on Windows
// at all (confirmed on this laptop, SESSION_LOG.md, v0.1 Step 3: adding a
// watch on a live \\wsl.localhost folder fails immediately with
// "ReadDirectoryChanges: Incorrect function", not merely late).
package watch

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// pollInterval is how often a WSL root's .jsonl files are re-stat'd, per the
// spec's "WSL roots polled every 5 s".
const pollInterval = 5 * time.Second

// OnChange is called with a .jsonl file's path whenever it is created or
// written to, native or WSL. Called from a background goroutine; the
// receiver is responsible for its own synchronisation (dataset.Cache's
// ingest methods already serialise on the store).
type OnChange func(path string)

// Watcher watches every adapter root for new or changed .jsonl files.
// Zero value is not usable; construct with New.
type Watcher struct {
	onChange OnChange

	fsw       *fsnotify.Watcher
	watchedMu sync.Mutex
	watched   map[string]bool // directories already added to fsw

	wslMu      sync.Mutex
	wslRoots   []string
	wslMtime   map[string]time.Time // path -> last seen mtime, for the poll loop
	wslStarted bool

	stop chan struct{}
	wg   sync.WaitGroup
}

// New creates a Watcher over nativeRoots (watched live via fsnotify,
// recursively) and wslRoots (polled every 5 seconds; nil or empty is fine,
// e.g. when wsl_scan is off or no distro was found). onChange fires once per
// detected write, debounced only by fsnotify's own event coalescing.
func New(nativeRoots, wslRoots []string, onChange OnChange) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		onChange: onChange,
		fsw:      fsw,
		watched:  map[string]bool{},
		wslRoots: append([]string(nil), wslRoots...),
		wslMtime: map[string]time.Time{},
		stop:     make(chan struct{}),
	}
	for _, root := range nativeRoots {
		// Startup only: the initial full backfill (cmd/burnmon's
		// a.rebuild, run right after this) ingests every existing file
		// under these roots itself, so addTree here only needs to set up
		// watches, not fire onChange for files it finds along the way.
		w.addTree(root, false)
	}
	return w, nil
}

// addTree adds root and every subdirectory under it to the fsnotify watcher.
// Errors (root missing, a permission-denied subfolder) are logged and
// otherwise ignored: a folder that cannot be watched simply falls back to
// being caught by the 15-minute full rescan, same as before this package
// existed.
//
// notifyExisting, when true, also calls onChange for every .jsonl file
// already present under root (root's own watch is skipped if already
// registered, but each subdirectory found is walked and watched exactly as
// during startup). This closes the race behind F7 (SESSION_LOG.md, v0.1.2,
// confirmed by TestWatcher_NewNestedDayFolderRace): a rollout writer that
// creates a whole new nested folder (Codex's YYYY/MM/DD) and its first file
// back to back can have the file's own Create event fire, and be silently
// dropped by Windows ReadDirectoryChanges, before fsw.Add on the brand-new
// leaf directory has run. Re-listing the directory right after the watch is
// added catches any file that raced past that window; onChange is safe to
// call again for a file the live watcher (or the full rescan) already saw,
// since dataset.Cache.IngestFile ingests by cursor and a repeat call is a
// no-op. handleFsnotifyEvent always passes true: every directory it sees is
// one fsnotify just told it about, i.e. new since startup, so nothing here
// duplicates the initial backfill.
func (w *Watcher) addTree(root string, notifyExisting bool) {
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if notifyExisting && strings.HasSuffix(strings.ToLower(p), ".jsonl") {
				w.onChange(p)
			}
			return nil
		}
		w.watchedMu.Lock()
		already := w.watched[p]
		w.watchedMu.Unlock()
		if already {
			return nil
		}
		if err := w.fsw.Add(p); err != nil {
			log.Printf("watch: could not watch %s: %v", p, err)
			return nil
		}
		w.watchedMu.Lock()
		w.watched[p] = true
		w.watchedMu.Unlock()
		return nil
	})
	if err != nil {
		log.Printf("watch: could not walk %s: %v", root, err)
	}
}

// Start runs the fsnotify event loop and the WSL poll loop in the
// background. Safe to call once; call Stop to end both.
func (w *Watcher) Start() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.runFsnotify()
	}()
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

// Stop closes the fsnotify watcher and stops the WSL poll loop, then waits
// for both goroutines to exit.
func (w *Watcher) Stop() {
	close(w.stop)
	w.fsw.Close()
	w.wg.Wait()
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

func (w *Watcher) runFsnotify() {
	for {
		select {
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handleFsnotifyEvent(ev)
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			log.Printf("watch: fsnotify error: %v", err)
		}
	}
}

func (w *Watcher) handleFsnotifyEvent(ev fsnotify.Event) {
	if ev.Op&(fsnotify.Create|fsnotify.Write) == 0 {
		return
	}
	fi, err := os.Stat(ev.Name)
	if err != nil {
		return // gone already, or a rename/remove we don't care about
	}
	if fi.IsDir() {
		if ev.Op&fsnotify.Create != 0 {
			// A new project or session folder: watch it (and anything under
			// it) too, so files written inside it are seen from here on. Pass
			// notifyExisting=true (F7 fix): this directory is new since
			// startup, so any .jsonl already inside it raced its own Create
			// event past the watch not existing yet and must be picked up
			// here instead.
			w.addTree(ev.Name, true)
		}
		return
	}
	if strings.HasSuffix(strings.ToLower(ev.Name), ".jsonl") {
		w.onChange(ev.Name)
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
