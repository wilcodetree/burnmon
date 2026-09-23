// Package pricing: gitRemoteURL reads a project's origin remote URL
// straight from .git\config (no git binary, K1's decision), so ClientFor
// can match a session's project against a client's remote pattern.

package pricing

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// gitRemoteURL walks up from projectPath looking for a .git entry (a
// directory in a normal checkout, or a file whose content is
// "gitdir: <path>" in a worktree or submodule), stopping at the filesystem
// root. Returns ("", false) when no .git is found or no
// `[remote "origin"]` section carries a url line.
func gitRemoteURL(projectPath string) (string, bool) {
	if projectPath == "" {
		return "", false
	}
	dir := projectPath
	for {
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil {
			realGitDir := gitPath
			if !info.IsDir() {
				// A worktree's or submodule's .git is a file containing
				// "gitdir: <path>", relative to the .git file's own
				// directory (dir) when not absolute.
				b, err := os.ReadFile(gitPath)
				if err != nil {
					return "", false
				}
				line := strings.TrimSpace(string(b))
				rest, ok := strings.CutPrefix(line, "gitdir:")
				if !ok {
					return "", false
				}
				gitdir := strings.TrimSpace(rest)
				if !filepath.IsAbs(gitdir) {
					gitdir = filepath.Join(dir, gitdir)
				}
				realGitDir = gitdir
				// A linked worktree's gitdir (.git/worktrees/<name>) has no
				// config of its own: "commondir" there points (often with
				// "..") at the real repository .git directory that does.
				if cb, err := os.ReadFile(filepath.Join(gitdir, "commondir")); err == nil {
					common := strings.TrimSpace(string(cb))
					if !filepath.IsAbs(common) {
						common = filepath.Join(gitdir, common)
					}
					realGitDir = common
				}
			}
			if url, ok := readOriginURL(filepath.Join(realGitDir, "config")); ok {
				return url, true
			}
			return "", false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// readOriginURL scans a git config file's INI-like text for the url line
// inside a [remote "origin"] section, without any git-specific parsing
// beyond section headers and "key = value" lines.
func readOriginURL(configPath string) (string, bool) {
	f, err := os.Open(configPath)
	if err != nil {
		return "", false
	}
	defer f.Close()

	inOrigin := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inOrigin = strings.EqualFold(line, `[remote "origin"]`)
			continue
		}
		if !inOrigin {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == "url" {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

// normalizeRemote puts an SSH-shorthand remote ("git@github.com:org/repo.git")
// into the same "host/path" shape as an https remote
// ("https://github.com/org/repo.git"), so a burnmon.json Remote pattern
// written once (e.g. "github.com/org/repo") matches a project cloned via
// either protocol. Known gap (documented, not fixed): an "ssh://" URL with
// an explicit port, or a non-"git" SSH user, is left as-is rather than
// reshaped; write a Remote pattern against its literal form in that case.
func normalizeRemote(url string) string {
	url = strings.TrimPrefix(url, "git@")
	if strings.Contains(url, "://") {
		return url
	}
	return strings.Replace(url, ":", "/", 1)
}

// canonicalRemote lowercases and strips a trailing ".git" and "/" so two
// spellings of the same remote (with or without the ".git" suffix, with or
// without a trailing slash) compare equal.
func canonicalRemote(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")
	return s
}

// remoteMatches reports whether pattern names remote as a whole path
// segment, not a bare substring: "github.com/org/dsi" matches
// ".../org/dsi" but not ".../org/dsi-engine", because the match must be
// bounded by "/" (or the string's own start/end) on both sides. remote is
// normalized first (normalizeRemote), so an SSH remote
// ("git@github.com:org/dsi.git") matches an https-shaped pattern
// ("github.com/org/dsi") the same way an https remote would.
func remoteMatches(remote, pattern string) bool {
	r := canonicalRemote(normalizeRemote(remote))
	p := canonicalRemote(pattern)
	if p == "" {
		return false
	}
	if r == p {
		return true
	}
	return strings.Contains("/"+r+"/", "/"+p+"/")
}
