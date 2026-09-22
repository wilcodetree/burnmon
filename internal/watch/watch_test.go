package watch

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// changeCollector is a thread-safe OnChange sink for tests.
type changeCollector struct {
	mu    sync.Mutex
	paths []string
}

func (c *changeCollector) onChange(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paths = append(c.paths, path)
}

func (c *changeCollector) waitFor(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		for _, p := range c.paths {
			if p == path {
				c.mu.Unlock()
				return
			}
		}
		c.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for a change notification on %s", path)
}

func TestWatcher_NativeRootSeesNewFile(t *testing.T) {
	dir := t.TempDir()
	c := &changeCollector{}
	w, err := New([]string{dir}, nil, c.onChange)
	if err != nil {
		t.Fatal(err)
	}
	w.Start()
	defer w.Stop()

	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"a":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.waitFor(t, path, 3*time.Second)
}

func TestWatcher_NativeRootSeesNewSubdirectory(t *testing.T) {
	dir := t.TempDir()
	c := &changeCollector{}
	w, err := New([]string{dir}, nil, c.onChange)
	if err != nil {
		t.Fatal(err)
	}
	w.Start()
	defer w.Stop()

	sub := filepath.Join(dir, "project-a")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// Give the watcher a moment to notice the new directory and start
	// watching it before a file lands inside it, mirroring how a fresh
	// Claude Code project folder is created just before its first session
	// file.
	time.Sleep(200 * time.Millisecond)

	path := filepath.Join(sub, "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"a":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.waitFor(t, path, 3*time.Second)
}

func TestWatcher_PollsWSLRootOnAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout.jsonl")
	if err := os.WriteFile(path, []byte(`{"a":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := &changeCollector{}
	w, err := New(nil, []string{dir}, c.onChange)
	if err != nil {
		t.Fatal(err)
	}
	// pollWSLOnce, called directly rather than through the 5-second ticker,
	// so the test does not need to sleep for a real poll cycle.
	w.pollWSLOnce()
	c.waitFor(t, path, time.Second)

	// A second poll with no further change must not re-fire.
	w.pollWSLOnce()
	c.mu.Lock()
	n := len(c.paths)
	c.mu.Unlock()
	if n != 1 {
		t.Fatalf("want exactly 1 notification for an unchanged file, got %d", n)
	}

	// Append: mtime moves forward, so it must fire again.
	time.Sleep(10 * time.Millisecond) // ensure a distinct mtime on filesystems with coarse resolution
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"a":2}` + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	w.pollWSLOnce()
	c.mu.Lock()
	n = len(c.paths)
	c.mu.Unlock()
	if n != 2 {
		t.Fatalf("want 2 notifications after an append, got %d", n)
	}
}
