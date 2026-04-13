package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsMainWorktree(t *testing.T) {
	t.Parallel()

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
	if isMainWorktree(linkedPath) {
		t.Fatalf("expected %q to be detected as a linked worktree", linkedPath)
	}
	if isMainWorktree(missingPath) {
		t.Fatalf("expected %q without .git metadata to be non-main", missingPath)
	}
}
