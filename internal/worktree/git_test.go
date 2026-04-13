package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsMainWorktree(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main")
	separateMainPath := filepath.Join(root, "separate-main")
	linkedPath := filepath.Join(root, "linked")
	missingPath := filepath.Join(root, "missing")
	separateGitDir := filepath.Join(root, "repo.git")
	linkedGitDir := filepath.Join(separateGitDir, "worktrees", "linked")

	if err := os.MkdirAll(filepath.Join(mainPath, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir main .git: %v", err)
	}
	if err := os.MkdirAll(separateMainPath, 0o755); err != nil {
		t.Fatalf("mkdir separate main: %v", err)
	}
	if err := os.MkdirAll(linkedPath, 0o755); err != nil {
		t.Fatalf("mkdir linked: %v", err)
	}
	if err := os.MkdirAll(linkedGitDir, 0o755); err != nil {
		t.Fatalf("mkdir linked gitdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(separateGitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("write separate gitdir HEAD: %v", err)
	}
	if err := os.WriteFile(filepath.Join(separateGitDir, "config"), []byte("[core]\n"), 0o644); err != nil {
		t.Fatalf("write separate gitdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(separateMainPath, ".git"), []byte("gitdir: "+separateGitDir+"\n"), 0o644); err != nil {
		t.Fatalf("write separate main .git file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(linkedPath, ".git"), []byte("gitdir: "+linkedGitDir+"\n"), 0o644); err != nil {
		t.Fatalf("write linked .git file: %v", err)
	}

	if !isMainWorktree(mainPath) {
		t.Fatalf("expected %q to be detected as the main worktree", mainPath)
	}
	if !isMainWorktree(separateMainPath) {
		t.Fatalf("expected %q with separate git dir to be detected as the main worktree", separateMainPath)
	}
	if !isMainWorktree(separateGitDir) {
		t.Fatalf("expected %q gitdir path to be detected as the main worktree entry", separateGitDir)
	}
	if isMainWorktree(linkedPath) {
		t.Fatalf("expected %q to be detected as a linked worktree", linkedPath)
	}
	if isMainWorktree(missingPath) {
		t.Fatalf("expected %q without .git metadata to be non-main", missingPath)
	}
}

func TestListProtectsSeparateGitDirMainWorktree(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main")
	gitDir := filepath.Join(root, "repo.git")
	linkedPath := filepath.Join(root, "feature")

	runGit(t, root, "init", "-b", "main", "--separate-git-dir", gitDir, mainPath)
	runGit(t, mainPath, "config", "user.name", "Test User")
	runGit(t, mainPath, "config", "user.email", "test@example.com")

	if err := os.WriteFile(filepath.Join(mainPath, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	runGit(t, mainPath, "add", "README.md")
	runGit(t, mainPath, "commit", "-m", "initial commit")
	runGit(t, mainPath, "worktree", "add", "-b", "feature", linkedPath, "HEAD")

	t.Chdir(mainPath)

	entries, err := List()
	if err != nil {
		t.Fatalf("List(): %v", err)
	}

	var mainEntry *Entry
	var linkedEntry *Entry
	for i := range entries {
		e := &entries[i]
		switch {
		case e.IsMain:
			mainEntry = e
		case samePath(e.Path, linkedPath):
			linkedEntry = e
		}
	}

	if mainEntry == nil {
		t.Fatalf("expected a main worktree entry, got %+v", entries)
	}
	if !samePath(mainEntry.Path, gitDir) {
		t.Fatalf("expected main entry path %q from git porcelain, got %q", gitDir, mainEntry.Path)
	}
	if !mainEntry.IsCurrent {
		t.Fatalf("expected separate-git-dir main entry to be current: %+v", *mainEntry)
	}
	if !mainEntry.Protected() {
		t.Fatalf("expected separate-git-dir main entry to be protected: %+v", *mainEntry)
	}
	merged := map[string]bool{mainEntry.Branch: true}
	if shouldCleanEntry(*mainEntry, merged) {
		t.Fatalf("expected main entry to be skipped by clean: %+v", *mainEntry)
	}
	if shouldNukeEntry(*mainEntry) {
		t.Fatalf("expected main entry to be skipped by nuke: %+v", *mainEntry)
	}

	if linkedEntry == nil {
		t.Fatalf("expected linked worktree entry, got %+v", entries)
	}
	if linkedEntry.IsMain {
		t.Fatalf("expected linked entry to remain non-main: %+v", *linkedEntry)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (dir=%s): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}
