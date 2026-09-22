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

	wslRoots []string
	wslMtime map[string]time.Time // path -> last seen mtime, for the poll loop

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
		w.addTree(root)
	}
	return w, nil
}

// addTree adds root and every subdirectory under it to the fsnotify watcher.
// Errors (root missing, a permission-denied subfolder) are logged and
// otherwise ignored: a folder that cannot be watched simply falls back to
// being caught by the 15-minute full rescan, same as before this package
// existed.
func (w *Watcher) addTree(root string) {
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
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
	if len(w.wslRoots) > 0 {
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
			// it) too, so files written inside it are seen from here on.
			w.addTree(ev.Name)
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
	for _, root := range w.wslRoots {
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
