//go:build windows

package scan

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestDistroRoot(t *testing.T) {
	cases := []struct {
		name    string
		distro  string
		base    string
		version uint32
		want    string
	}{
		{"wsl1 with verbatim prefix", "Ubuntu-Legacy", `\\?\C:\WSL\ubuntu1`, 1, filepath.Join(`C:\WSL\ubuntu1`, "rootfs")},
		{"wsl1 without prefix", "Debian-Old", `D:\wsl1`, 1, filepath.Join(`D:\wsl1`, "rootfs")},
		{"wsl2", "Ubuntu", "", 2, `\\wsl.localhost\Ubuntu`},
		{"wsl1 empty basepath falls back to unc", "Ubuntu-Old", "", 1, `\\wsl.localhost\Ubuntu-Old`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := distroRoot(c.distro, c.base, c.version)
			if got != c.want {
				t.Errorf("distroRoot(%q, %q, %d) = %q, want %q", c.distro, c.base, c.version, got, c.want)
			}
		})
	}
}

func TestSkipDistro(t *testing.T) {
	skip := []string{"docker-desktop", "Docker-Desktop-Data", "rancher-desktop", "RANCHER-DESKTOP-DATA", "podman-machine-default", "PODMAN-MACHINE-foo"}
	for _, name := range skip {
		if !skipDistro(name) {
			t.Errorf("skipDistro(%q) = false, want true", name)
		}
	}
	keep := []string{"Ubuntu", "Debian", "Ubuntu-22.04"}
	for _, name := range keep {
		if skipDistro(name) {
			t.Errorf("skipDistro(%q) = true, want false", name)
		}
	}
}

func TestDistroSources(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "home", "alice", ".claude", "projects"))
	mustMkdirAll(t, filepath.Join(root, "home", "bob")) // no .claude here
	mustMkdirAll(t, filepath.Join(root, "root", ".claude", "projects"))

	got := distroSources(root)
	sort.Strings(got)
	want := []string{
		filepath.Join(root, "home", "alice", ".claude", "projects"),
		filepath.Join(root, "root", ".claude", "projects"),
	}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("distroSources(%q) = %v, want %v", root, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("distroSources(%q)[%d] = %q, want %q", root, i, got[i], want[i])
		}
	}
}

func mustMkdirAll(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", p, err)
	}
}

func TestParsePasswd(t *testing.T) {
	data := strings.Join([]string{
		"root:x:0:0:root:/root:/bin/bash",
		"daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin",
		"bin:x:2:2:bin:/bin:/usr/sbin/nologin",
		"# a comment line, ignored",
		"",
		"nobody:x:65534:65534:nobody:/nonexistent:/bin/false",
		"wilco:x:1000:1000:wilco,,,:/home/wilco:/bin/bash",
	}, "\n")

	got := parsePasswd(strings.NewReader(data))
	wantNames := map[string]bool{"root": true, "wilco": true}
	if len(got) != len(wantNames) {
		t.Fatalf("parsePasswd returned %d entries, want %d: %+v", len(got), len(wantNames), got)
	}
	for _, e := range got {
		if !wantNames[e.name] {
			t.Errorf("parsePasswd returned unexpected entry %+v", e)
		}
	}
}

func TestConfigDirFromShellFiles(t *testing.T) {
	root := t.TempDir()
	home := "/home/testuser"
	winHome := linuxToWindowsPath(root, home)
	mustMkdirAll(t, winHome)

	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(winHome, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%q): %v", name, err)
		}
	}

	t.Run("quoted value", func(t *testing.T) {
		write(".profile", `export CLAUDE_CONFIG_DIR="/home/testuser/.claude2"`+"\n")
		got := configDirFromShellFiles(root, home)
		want := "/home/testuser/.claude2"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		os.Remove(filepath.Join(winHome, ".profile"))
	})

	t.Run("$HOME value", func(t *testing.T) {
		write(".profile", `CLAUDE_CONFIG_DIR=$HOME/.config/claude`+"\n")
		got := configDirFromShellFiles(root, home)
		want := "/home/testuser/.config/claude"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		os.Remove(filepath.Join(winHome, ".profile"))
	})

	t.Run("~/ value", func(t *testing.T) {
		write(".profile", `CLAUDE_CONFIG_DIR=~/.claude3`+"\n")
		got := configDirFromShellFiles(root, home)
		want := "/home/testuser/.claude3"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		os.Remove(filepath.Join(winHome, ".profile"))
	})

	t.Run("commented line ignored", func(t *testing.T) {
		write(".profile", "# CLAUDE_CONFIG_DIR=/should/not/apply\n")
		got := configDirFromShellFiles(root, home)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
		os.Remove(filepath.Join(winHome, ".profile"))
	})

	t.Run("unexpandable value rejected", func(t *testing.T) {
		write(".profile", `CLAUDE_CONFIG_DIR=$OTHER/.claude`+"\n")
		got := configDirFromShellFiles(root, home)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
		os.Remove(filepath.Join(winHome, ".profile"))
	})

	t.Run("precedence between two files", func(t *testing.T) {
		write(".profile", "CLAUDE_CONFIG_DIR=/home/testuser/.from-profile\n")
		write(".bashrc", "CLAUDE_CONFIG_DIR=/home/testuser/.from-bashrc\n")
		got := configDirFromShellFiles(root, home)
		want := "/home/testuser/.from-bashrc" // .bashrc is read after .profile
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		os.Remove(filepath.Join(winHome, ".profile"))
		os.Remove(filepath.Join(winHome, ".bashrc"))
	})
}
