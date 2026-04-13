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
	linkedPath := filepath.Join(root, "linked")
	missingPath := filepath.Join(root, "missing")

	if err := os.MkdirAll(filepath.Join(mainPath, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir main .git: %v", err)
	}
	if err := os.MkdirAll(linkedPath, 0o755); err != nil {
		t.Fatalf("mkdir linked: %v", err)
	}
	if err := os.WriteFile(filepath.Join(linkedPath, ".git"), []byte("gitdir: /tmp/gitdir\n"), 0o644); err != nil {
		t.Fatalf("write linked .git file: %v", err)
	}

	if !isMainWorktree(mainPath) {
		t.Fatalf("expected %q to be detected as the main worktree", mainPath)
	}
	if isMainWorktree(linkedPath) {
		t.Fatalf("expected %q to be detected as a linked worktree", linkedPath)
	}
	if isMainWorktree(missingPath) {
		t.Fatalf("expected %q without .git metadata to be non-main", missingPath)
	}
}
