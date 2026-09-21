//go:build windows

// WSL distro detection. Claude Code running inside a WSL distribution is a
// Linux process with a Linux $HOME; its transcripts never land under any of
// the Windows paths DefaultSourcesWithOptions already knows about. This file
// finds them from the Windows side: which distros are installed (the
// registry, which is instant and touches no filesystem), where each one's
// projects folder actually is (real home directories from /etc/passwd, a
// glob fallback when that cannot be read, and any CLAUDE_CONFIG_DIR override
// found in a home's shell startup files), all bounded by a deadline because
// probing a stopped WSL 2 distro over \\wsl.localhost can cold-start it.
//
// See docs\2026-08-17_wsl-source-detection-design.md for the research this
// is built on.
package scan

import (
	"bufio"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows/registry"
)

// distro is what readDistros reads out of one Lxss subkey.
type distro struct {
	name     string
	basePath string
	version  uint32
}

// skipDistroNames are container/VM helper distros registered under Lxss that
// never hold a user's .claude folder. Probing one risks cold-starting a
// container VM for nothing, so they are filtered before any filesystem
// access, not after. Matched case-insensitively; podman-machine additionally
// carries a per-machine suffix, so it is matched by prefix.
var skipDistroNames = []string{
	"docker-desktop", "docker-desktop-data",
	"rancher-desktop", "rancher-desktop-data",
}

func skipDistro(name string) bool {
	l := strings.ToLower(name)
	if strings.HasPrefix(l, "podman-machine") {
		return true
	}
	for _, s := range skipDistroNames {
		if l == s {
			return true
		}
	}
	return false
}

// readDistros enumerates the GUID subkeys of
// HKCU\Software\Microsoft\Windows\CurrentVersion\Lxss. Any error opening the
// key, including the common case of the key not existing because WSL is not
// installed, is returned so the caller can treat "no key" as "no WSL"
// without a single filesystem probe.
func readDistros() ([]distro, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Lxss`, registry.READ)
	if err != nil {
		return nil, err
	}
	defer k.Close()

	names, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil, err
	}

	var out []distro
	for _, sub := range names {
		sk, err := registry.OpenKey(k, sub, registry.READ)
		if err != nil {
			continue
		}
		name, _, err := sk.GetStringValue("DistributionName")
		if err != nil || name == "" {
			sk.Close()
			continue
		}
		state, _, _ := sk.GetIntegerValue("State")
		if state != 1 {
			sk.Close()
			continue
		}
		if skipDistro(name) {
			sk.Close()
			continue
		}
		basePath, _, _ := sk.GetStringValue("BasePath")
		version, _, _ := sk.GetIntegerValue("Version")
		sk.Close()
		out = append(out, distro{name: name, basePath: basePath, version: uint32(version)})
	}
	return out, nil
}

// distroRoot returns the filesystem root to probe for a distro. A WSL 1
// distro's root filesystem sits on NTFS under BasePath\rootfs: a plain local
// path, no 9P file server, no VM, no cold start, so it is worth the extra
// branch. Everything else, including a WSL 1 entry with no BasePath, uses
// the \\wsl.localhost UNC form. \\wsl.localhost is used rather than the
// older \\wsl$ form because Microsoft's own guidance prefers it for
// performance and reliability; a machine where it fails has a broken
// network provider order, which is a support issue and not something to
// paper over with a fallback to \\wsl$.
func distroRoot(name, basePath string, version uint32) string {
	if version == 1 && basePath != "" {
		p := strings.TrimPrefix(basePath, `\\?\`)
		return filepath.Join(p, "rootfs")
	}
	return `\\wsl.localhost\` + name
}

// distroSources is the fallback source list used when /etc/passwd cannot be
// read: the default .claude/projects folder under every home\* directory
// plus root's own, with no CLAUDE_CONFIG_DIR resolution attempted. Stat and
// permission errors (root\home in particular, on a distro with no such
// directory) are not worth surfacing; they just mean nothing was found
// there.
func distroSources(root string) []string {
	var out []string
	candidates, _ := filepath.Glob(filepath.Join(root, "home", "*", ".claude", "projects"))
	candidates = append(candidates, filepath.Join(root, "root", ".claude", "projects"))
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// passwdEntry is one kept row of /etc/passwd: an account worth checking for
// a .claude folder.
type passwdEntry struct {
	name string
	uid  int
	home string
}

var nologinShells = map[string]bool{
	"/usr/sbin/nologin": true,
	"/sbin/nologin":     true,
	"/bin/false":        true,
}

// parsePasswd extracts the accounts worth checking from an /etc/passwd file:
// uid 0 (root) or uid 1000 and above (Linux distros reserve 1-999 for system
// accounts), excluding a shell that marks the account as not for
// interactive login. Malformed lines (comments, blank lines, anything with
// fewer than the 7 colon-separated fields) are skipped rather than treated
// as an error, since a best-effort read is exactly what this layer is: see
// distroSources for the fallback when the file cannot be read at all.
func parsePasswd(r io.Reader) []passwdEntry {
	var out []passwdEntry
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) < 7 {
			continue
		}
		name, uidStr, home, shell := fields[0], fields[2], fields[5], fields[6]
		uid, err := strconv.Atoi(uidStr)
		if err != nil {
			continue
		}
		if uid != 0 && uid < 1000 {
			continue
		}
		if nologinShells[shell] {
			continue
		}
		if home == "" {
			continue
		}
		out = append(out, passwdEntry{name: name, uid: uid, home: home})
	}
	return out
}

// readPasswdHomes reads and parses /etc/passwd under a distro root. Its
// error is only ever "could not read the file", which readDistroDirs below
// treats as "fall back to the glob", so nothing here needs to distinguish
// missing-file from any other read error.
func readPasswdHomes(root string) ([]passwdEntry, error) {
	f, err := os.Open(filepath.Join(root, "etc", "passwd"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parsePasswd(f), nil
}

// linuxToWindowsPath rewrites a Linux absolute path (as read out of
// /etc/passwd or a shell file) into the Windows-side path under root that
// reaches the same file.
func linuxToWindowsPath(root, linuxPath string) string {
	rel := filepath.FromSlash(strings.TrimPrefix(linuxPath, "/"))
	return filepath.Join(root, rel)
}

// claudeConfigDirRe matches a shell assignment to CLAUDE_CONFIG_DIR, with or
// without a leading "export", capturing the value up to an optional trailing
// comment. Matching is case-sensitive, same as a shell reading its own
// startup files.
var claudeConfigDirRe = regexp.MustCompile(`^\s*(?:export\s+)?CLAUDE_CONFIG_DIR\s*=\s*(.+?)\s*(?:#.*)?$`)

// maxShellFileBytes bounds how much of any one shell startup file
// configDirFromShellFiles will read. A CLAUDE_CONFIG_DIR assignment, if
// present, lives in an ordinary small file; anything past this size is
// skipped outright rather than partially read, since a giant startup file is
// itself a sign something unusual is going on.
const maxShellFileBytes = 64 * 1024

// parseConfigDirValue cleans up one matched CLAUDE_CONFIG_DIR value: strips
// a matching pair of quotes, expands a leading ~/ and $HOME or ${HOME} to
// home, and rejects the value outright if it still contains an unexpanded
// $, since guessing at further shell expansion is how this kind of code
// starts lying about where it looked.
func parseConfigDirValue(raw, home string) (string, bool) {
	v := strings.TrimSpace(raw)
	if len(v) >= 2 {
		if (v[0] == '\'' && v[len(v)-1] == '\'') || (v[0] == '"' && v[len(v)-1] == '"') {
			v = v[1 : len(v)-1]
		}
	}
	if strings.HasPrefix(v, "~/") {
		v = home + "/" + strings.TrimPrefix(v, "~/")
	}
	v = strings.ReplaceAll(v, "${HOME}", home)
	v = strings.ReplaceAll(v, "$HOME", home)
	if v == "" || strings.Contains(v, "$") {
		return "", false
	}
	return v, true
}

// configDirFromShellFiles looks for a CLAUDE_CONFIG_DIR assignment in the
// fixed, ordered set of files a login or interactive shell would actually
// read, so a moved config directory is found without ever executing
// anything inside the distro. home is the Linux-style absolute home
// directory (e.g. "/home/wilco" or "/root"); root is the Windows-side distro
// root from distroRoot. The last matching assignment across the ordered list
// wins, the same as a shell evaluating them in order would behave. Returns
// "" if nothing was found.
func configDirFromShellFiles(root, home string) string {
	files := []string{
		linuxToWindowsPath(root, "/etc/environment"),
		linuxToWindowsPath(root, "/etc/profile"),
		filepath.Join(linuxToWindowsPath(root, home), ".profile"),
		filepath.Join(linuxToWindowsPath(root, home), ".bash_profile"),
		filepath.Join(linuxToWindowsPath(root, home), ".bashrc"),
		filepath.Join(linuxToWindowsPath(root, home), ".zshenv"),
		filepath.Join(linuxToWindowsPath(root, home), ".zshrc"),
	}

	var resolved string
	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil || fi.IsDir() || fi.Size() > maxShellFileBytes {
			continue
		}
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		for sc.Scan() {
			m := claudeConfigDirRe.FindStringSubmatch(sc.Text())
			if m == nil {
				continue
			}
			if v, ok := parseConfigDirValue(m[1], home); ok {
				resolved = v
			}
		}
		fh.Close()
	}
	return resolved
}

// distroProjectDirs resolves every .claude/projects folder for one distro
// root: real home directories from /etc/passwd, the default
// <home>/.claude/projects for each, and any CLAUDE_CONFIG_DIR override found
// in that home's shell startup files (kept alongside the default, not
// instead of it, so history predating a moved config dir is not dropped).
// Falls back to the plain glob in distroSources when /etc/passwd cannot be
// read, which is also the normal case for a container-style distro with no
// real passwd database.
func distroProjectDirs(root string) []string {
	entries, err := readPasswdHomes(root)
	if err != nil {
		return distroSources(root)
	}

	var out []string
	add := func(p, layer string) {
		fi, statErr := os.Stat(p)
		if statErr != nil || !fi.IsDir() {
			return
		}
		out = append(out, p)
		log.Printf("wsl: source %s (%s)", p, layer)
	}

	for _, e := range entries {
		winHome := linuxToWindowsPath(root, e.home)
		add(filepath.Join(winHome, ".claude", "projects"), "passwd default")
		if custom := configDirFromShellFiles(root, e.home); custom != "" {
			add(filepath.Join(linuxToWindowsPath(root, custom), "projects"), "shell-file CLAUDE_CONFIG_DIR")
		}
	}
	return out
}

// wslNamesMu and wslNamesFound record the distro names that contributed at
// least one source on the most recent wslSources call, for WSLDistroNames
// below. Package-level and mutex-guarded rather than threaded through
// DefaultSourcesWithOptions's plain []string return, because the app's
// header text ("WSL transcripts (Ubuntu) are re-read every...") needs to
// name a distro and a WSL 1 root path (BasePath\rootfs\...) carries no
// \\wsl.localhost\<name>\ segment to parse the name back out of.
var (
	wslNamesMu    sync.Mutex
	wslNamesFound []string
)

// WSLDistroNames returns the display names of the WSL distributions that
// contributed at least one source to the most recent wslSources call. Empty
// before the first call, if WSL is not installed, or if no distro's projects
// folder was found.
func WSLDistroNames() []string {
	wslNamesMu.Lock()
	defer wslNamesMu.Unlock()
	return append([]string(nil), wslNamesFound...)
}

// wslSources returns every .claude/projects folder found inside an installed
// WSL distribution, or nil if WSL is not installed on this machine or the
// deadline is exceeded before any distro finishes probing.
//
// Reading the Lxss registry key is microseconds and touches no filesystem;
// that is the gate. Everything past it, the UNC probing in
// distroProjectDirs and its CLAUDE_CONFIG_DIR resolution, can be slow or,
// for a stopped WSL 2 distro, cold-start a Linux VM, so all of it runs
// inside the deadline. On timeout, whatever distros had already finished are
// returned and the ones that had not are logged by name.
func wslSources(deadline time.Duration) []string {
	distros, err := readDistros()
	if err != nil || len(distros) == 0 {
		wslNamesMu.Lock()
		wslNamesFound = nil
		wslNamesMu.Unlock()
		return nil
	}

	var (
		mu        sync.Mutex
		out       []string
		names     []string
		completed = make(map[string]bool, len(distros))
	)

	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		for _, d := range distros {
			d := d
			wg.Add(1)
			go func() {
				defer wg.Done()
				root := distroRoot(d.name, d.basePath, d.version)
				found := distroProjectDirs(root)
				mu.Lock()
				if len(found) > 0 {
					out = append(out, found...)
					names = append(names, d.name)
				}
				completed[d.name] = true
				mu.Unlock()
			}()
		}
		wg.Wait()
	}()

	select {
	case <-done:
	case <-time.After(deadline):
		mu.Lock()
		var skipped []string
		for _, d := range distros {
			if !completed[d.name] {
				skipped = append(skipped, d.name)
			}
		}
		mu.Unlock()
		if len(skipped) > 0 {
			log.Printf("wsl: probing timed out after %s, skipped distro(s): %s", deadline, strings.Join(skipped, ", "))
		}
	}

	mu.Lock()
	defer mu.Unlock()
	wslNamesMu.Lock()
	wslNamesFound = append([]string(nil), names...)
	wslNamesMu.Unlock()
	return append([]string(nil), out...)
}
