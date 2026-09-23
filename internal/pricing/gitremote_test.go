package pricing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitRemoteURLFindsOriginInGitConfig(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[core]\n\trepositoryformatversion = 0\n[remote \"origin\"]\n\turl = https://github.com/multica-ai/burnmon.git\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n[branch \"main\"]\n\tremote = origin\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "internal", "pricing")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got, ok := gitRemoteURL(sub)
	if !ok {
		t.Fatalf("gitRemoteURL(%q) found no remote, want a match walking up to %q", sub, gitDir)
	}
	want := "https://github.com/multica-ai/burnmon.git"
	if got != want {
		t.Fatalf("gitRemoteURL = %q, want %q", got, want)
	}
}

func TestGitRemoteURLNoGitDirReturnsFalse(t *testing.T) {
	root := t.TempDir()
	if _, ok := gitRemoteURL(root); ok {
		t.Fatalf("gitRemoteURL with no .git anywhere above %q, want ok=false", root)
	}
}

// TestGitRemoteURLFindsOriginThroughWorktreeGitFile guards Important 1: a
// linked git worktree's .git is a file pointing at
// <main repo>/.git/worktrees/<name>, which has no config of its own, only
// a "commondir" file (typically "../..") pointing back at the real .git
// directory that does. gitRemoteURL must follow that chain, not just the
// gitdir line.
func TestGitRemoteURLFindsOriginThroughWorktreeGitFile(t *testing.T) {
	root := t.TempDir()
	mainGitDir := filepath.Join(root, "main", ".git")
	worktreeGitDir := filepath.Join(mainGitDir, "worktrees", "feature")
	worktreeDir := filepath.Join(root, "worktrees", "feature")
	if err := os.MkdirAll(worktreeGitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfgText := "[remote \"origin\"]\n\turl = https://github.com/multica-ai/burnmon.git\n"
	if err := os.WriteFile(filepath.Join(mainGitDir, "config"), []byte(cfgText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktreeGitDir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitFileContent := "gitdir: " + worktreeGitDir + "\n"
	if err := os.WriteFile(filepath.Join(worktreeDir, ".git"), []byte(gitFileContent), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := gitRemoteURL(worktreeDir)
	if !ok {
		t.Fatalf("gitRemoteURL(%q) found no remote through the worktree's commondir chain", worktreeDir)
	}
	want := "https://github.com/multica-ai/burnmon.git"
	if got != want {
		t.Fatalf("gitRemoteURL = %q, want %q", got, want)
	}
}
