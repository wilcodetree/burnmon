// Package scan finds and parses the JSONL session transcripts Cowork and
// Claude Code write on this machine.
package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// wslDeadline bounds how long DefaultSourcesWithOptions will wait on WSL
// distro detection and per-distro probing before giving up on whatever has
// not finished yet. Probing a WSL 2 distro crosses the 9P file server and,
// for a stopped distro, can cold-start a Linux VM; 5 seconds is enough for a
// warm distro and short enough that a cold-start stall does not hold up the
// window's first paint.
const wslDeadline = 5 * time.Second

// DefaultSources lists every place Cowork or Claude Code is known to write
// JSONL transcripts, keeping only those that exist on this machine,
// including any WSL distribution on Windows. Equivalent to
// DefaultSourcesWithOptions(true).
func DefaultSources() []string {
	return DefaultSourcesWithOptions(true)
}

// DefaultSourcesWithOptions is DefaultSources with WSL distro detection
// optional. A caller that already knows WSL scanning is disabled by config
// (wsl_scan: off), or that only wants the fast native sources for a
// cadence-limited pass, passes scanWSL false and skips the registry read and
// any UNC probing entirely. The option is threaded through as a plain bool
// rather than read from config here, so this package never needs to import
// the pricing config types.
func DefaultSourcesWithOptions(scanWSL bool) []string {
	var out []string
	add := func(p string) {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			out = append(out, p)
		}
	}
	home, _ := os.UserHomeDir()

	// Claude Code CLI
	if home != "" {
		add(filepath.Join(home, ".claude", "projects"))
	}
	// A moved CLAUDE_CONFIG_DIR relocates the whole .claude tree, transcripts
	// included. Honouring it here is a native-side bug fix independent of
	// WSL: a user who moved it currently sees the same empty report a WSL
	// tester with a moved config dir sees. Costs one env lookup when unset.
	if ccd := os.Getenv("CLAUDE_CONFIG_DIR"); ccd != "" {
		add(filepath.Join(ccd, "projects"))
	}

	// The two session folder names Claude Desktop has used. Cowork and Chat
	// sessions live in local-agent-mode-sessions; a 2026 update moved Desktop
	// Code sessions to claude-code-sessions (today that folder holds only
	// sidebar state, the Code transcripts land in ~\.claude\projects via the
	// embedded CLI runtime, but scan it anyway in case transcripts follow).
	names := []string{"local-agent-mode-sessions", "claude-code-sessions"}

	for _, name := range names {
		// Packaged (Microsoft Store / MSIX) install. LocalCache\Roaming is
		// the package's view of %APPDATA%, LocalCache\Local of %LOCALAPPDATA%.
		if lad := os.Getenv("LOCALAPPDATA"); lad != "" {
			for _, mapped := range []string{"Roaming", "Local"} {
				hits, _ := filepath.Glob(filepath.Join(lad, "Packages", "Claude_*",
					"LocalCache", mapped, "Claude", name))
				for _, h := range hits {
					add(h)
				}
			}
			// Regular install after the documented Roaming-to-Local move.
			add(filepath.Join(lad, "Claude", name))
		}

		// Regular install, Roaming (the location current builds use).
		if ad := os.Getenv("APPDATA"); ad != "" {
			add(filepath.Join(ad, "Claude", name))
		}

		if home != "" {
			// macOS
			add(filepath.Join(home, "Library", "Application Support", "Claude", name))
			// Linux
			add(filepath.Join(home, ".config", "Claude", name))
		}
	}

	if scanWSL {
		out = append(out, wslSources(wslDeadline)...)
	}
	return out
}

// FindJSONL walks the sources and returns one path per transcript file name.
// The same session can be mirrored in several places; keep the most recently
// modified copy, like the Python extractor does.
func FindJSONL(sources []string) []string {
	type pick struct {
		path  string
		mtime time.Time
	}
	best := map[string]pick{}
	for _, src := range sources {
		fi, err := os.Stat(src)
		if err != nil {
			continue
		}
		if !fi.IsDir() {
			if strings.HasSuffix(src, ".jsonl") && filepath.Base(src) != "audit.jsonl" {
				best[filepath.Base(src)] = pick{src, fi.ModTime()}
			}
			continue
		}
		_ = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				// Cowork session workspaces hold scratch copies of other
				// sessions' transcripts (mirror_copy, local_sessions, ...)
				// under outputs\. Picking those up mis-tags Claude Code
				// sessions as Cowork, because the copy's path wins the
				// newest-mtime dedup and then the surface classification.
				if d.Name() == "outputs" || d.Name() == "uploads" {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".jsonl") {
				return nil
			}
			// audit.jsonl is the Cowork session workspace's own operations
			// log, not a transcript: its type/operation lines replay enough
			// assistant/usage records to parse as a phantom session with
			// inflated call counts, and every one of them shares the same
			// basename anyway, so keeping it just means one random
			// workspace's audit log wins the dedup.
			if d.Name() == "audit.jsonl" {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			prev, ok := best[d.Name()]
			if !ok || info.ModTime().After(prev.mtime) {
				best[d.Name()] = pick{p, info.ModTime()}
			}
			return nil
		})
	}
	paths := make([]string, 0, len(best))
	for _, v := range best {
		paths = append(paths, v.path)
	}
	sort.Strings(paths)
	return paths
}
