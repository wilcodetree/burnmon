package watch

import (
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The tail poll (WS2 follow-up, 2026-09-29, item 2) is the real fix for a
// transcript its writer keeps open: Codex holds a rollout open for the
// whole chat, and NTFS updates a held-open file's last-write time lazily,
// so ReadDirectoryChangesW never reports those appends (confirmed live: a
// rollout with 24 new token_count lines still showed its creation time as
// LastWriteTime two hours later). Every tailInterval, each recently active
// native .jsonl file is opened and its size read from the handle; a size
// that differs from the last one seen fires onChange, the same callback a
// real notification fires.
const (
	tailInterval = 2 * time.Second
	// tailWindow is how long a file stays tailed after its last sign of
	// life (a notification, a Track call or a size change seen here).
	tailWindow = 60 * time.Minute
	// tailCap bounds how many files one tick opens; the most recently
	// active ones win.
	tailCap = 50
	// tailCapLogEvery rate-limits the "cap hit" log line, which would
	// otherwise repeat every tailInterval.
	tailCapLogEvery = 10 * time.Minute
)

// tailEntry is one tailed file: when it last showed life, and the size the
// tail last saw (-1 until the first tick has looked at it).
type tailEntry struct {
	active time.Time
	size   int64
}

// Track marks path as recently active, so the tail poll watches it for
// tailWindow from now. Paths outside every native root (a WSL file, whose
// every open is a 9P round trip) are ignored. Safe to call from any
// goroutine, before or after Start. cmd wires it to dataset.Cache's
// OnAdvance (a cursor that moved); SetTailSeed covers Codex's recent
// rollouts; native notifications call it on their own.
func (w *Watcher) Track(path string) {
	if !strings.HasSuffix(strings.ToLower(path), ".jsonl") || !w.underNativeRoot(path) {
		return
	}
	now := time.Now()
	w.tailMu.Lock()
	defer w.tailMu.Unlock()
	if e, ok := w.tail[path]; ok {
		e.active = now
		return
	}
	w.tail[path] = &tailEntry{active: now, size: -1}
}

func (w *Watcher) underNativeRoot(path string) bool {
	p := strings.ToLower(filepath.Clean(path))
	for _, r := range w.nativeRoots {
		r = strings.ToLower(filepath.Clean(r))
		if strings.HasPrefix(p, r+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// tailSeedEvery is how often a SetTailSeed function is re-run.
const tailSeedEvery = time.Minute

// SetTailSeed registers fn, whose paths are Tracked at Start and again
// every tailSeedEvery. cmd passes Codex's today-and-yesterday rollouts: a
// chat left idle past tailWindow would otherwise drop out of the tail, and
// its next append raises no notice to bring it back (found by review,
// 2026-09-29). Call before Start.
func (w *Watcher) SetTailSeed(fn func() []string) {
	w.tailSeed = fn
}

func (w *Watcher) seedTail() {
	if w.tailSeed == nil {
		return
	}
	for _, p := range w.tailSeed() {
		w.Track(p)
	}
}

func (w *Watcher) runTail() {
	w.seedTail()
	ticker := time.NewTicker(tailInterval)
	defer ticker.Stop()
	lastSeed := time.Now()
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			if time.Since(lastSeed) >= tailSeedEvery {
				w.seedTail()
				lastSeed = time.Now()
			}
			w.pollTailOnce()
		}
	}
}

// pollTailOnce is one tail tick: expire files idle past tailWindow, pick
// the tailCap most recently active, read each one's size from an open
// handle (sizeByHandle, not a directory listing) and fire onChange for
// every size that moved. onChange runs outside tailMu.
func (w *Watcher) pollTailOnce() {
	now := time.Now()
	type cand struct {
		path   string
		active time.Time
		size   int64
	}
	w.tailMu.Lock()
	var cands []cand
	for p, e := range w.tail {
		if now.Sub(e.active) > tailWindow {
			delete(w.tail, p)
			continue
		}
		cands = append(cands, cand{p, e.active, e.size})
	}
	w.tailMu.Unlock()

	if len(cands) > tailCap {
		sort.Slice(cands, func(i, j int) bool { return cands[i].active.After(cands[j].active) })
		if now.Sub(w.tailCapLogged) >= tailCapLogEvery {
			log.Printf("watch: tail poll cap hit, %d active files, tailing the %d most recent", len(cands), tailCap)
			w.tailCapLogged = now
		}
		cands = cands[:tailCap]
	}

	var changed []string
	for _, c := range cands {
		size, err := sizeByHandle(c.path)
		if err != nil {
			continue // gone or locked this instant; the next tick retries
		}
		if size == c.size {
			continue
		}
		w.tailMu.Lock()
		if e, ok := w.tail[c.path]; ok {
			e.size = size
			if c.size >= 0 {
				e.active = now // a real growth, not the first look
			}
		}
		w.tailMu.Unlock()
		changed = append(changed, c.path)
	}
	for _, p := range changed {
		w.onChange(p)
	}
}
