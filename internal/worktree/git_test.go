package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDirSize_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	size := dirSize(dir)
	if size != 0 {
		t.Errorf("dirSize(empty) = %d, want 0", size)
	}
}

func TestDirSize_WithFiles(t *testing.T) {
	dir := t.TempDir()
	data := []byte("hello world") // 11 bytes
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	size := dirSize(dir)
	if size != 22 {
		t.Errorf("dirSize = %d, want 22", size)
	}
}

func TestDirSize_Nested(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	data := []byte("test")
	if err := os.WriteFile(filepath.Join(sub, "file"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	size := dirSize(dir)
	if size != int64(len(data)) {
		t.Errorf("dirSize = %d, want %d", size, len(data))
	}
}

func TestDirSize_NonexistentDir(t *testing.T) {
	size := dirSize("/nonexistent/path/that/does/not/exist")
	if size != 0 {
		t.Errorf("dirSize(nonexistent) = %d, want 0", size)
	}
}

func TestIsSameOrChild_Same(t *testing.T) {
	dir := t.TempDir()
	if !isSameOrChild(dir, dir) {
		t.Error("expected same directory to return true")
	}
}

func TestIsSameOrChild_Child(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "sub")
	if err := os.MkdirAll(child, 0o750); err != nil {
		t.Fatal(err)
	}

	if !isSameOrChild(child, parent) {
		t.Error("expected child directory to return true")
	}
}

func TestIsSameOrChild_Unrelated(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()

	if isSameOrChild(a, b) {
		t.Error("expected unrelated directories to return false")
	}
}

func TestIsSameOrChild_Nonexistent(_ *testing.T) {
	// When paths don't exist, falls back to string comparison.
	// Just ensure it doesn't panic.
	isSameOrChild("/a/b", "/a")
}

func TestClassifyEntry(t *testing.T) {
	merged := map[string]bool{"feature-done": true}

	tests := []struct {
		name  string
		entry Entry
		want  Status
	}{
		{"prunable", Entry{Prunable: true, Branch: "x"}, StatusPrunable},
		{"detached", Entry{Branch: ""}, StatusDetached},
		{"merged", Entry{Branch: "feature-done"}, StatusMerged},
		{"active", Entry{Branch: "feature-wip"}, StatusActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyEntry(&tt.entry, merged)
			if got != tt.want {
				t.Errorf("classifyEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLastActiveTime_NonexistentPath(t *testing.T) {
	result := lastActiveTime("/nonexistent/worktree/path")
	if !result.IsZero() {
		t.Errorf("expected zero time for nonexistent path, got %v", result)
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "-"},
		{512, "512 B"},
		{1024, "1 KB"},
		{1536, "2 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := humanSize(tt.bytes)
		if got != tt.want {
			t.Errorf("humanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"zero", time.Time{}, "-"},
		{"just now", now.Add(-10 * time.Second), "just now"},
		{"minutes", now.Add(-5 * time.Minute), "5m ago"},
		{"hours", now.Add(-3 * time.Hour), "3h ago"},
		{"days", now.Add(-7 * 24 * time.Hour), "7d ago"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relativeTime(tt.t)
			if got != tt.want {
				t.Errorf("relativeTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusString(t *testing.T) {
	tests := []struct {
		s    Status
		want string
	}{
		{StatusActive, "active"},
		{StatusMerged, "merged"},
		{StatusDetached, "detached"},
		{StatusPrunable, "prunable"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Status(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestEntryProtected(t *testing.T) {
	tests := []struct {
		name string
		e    Entry
		want bool
	}{
		{"main", Entry{IsMain: true}, true},
		{"locked", Entry{Locked: true}, true},
		{"current", Entry{IsCurrent: true}, true},
		{"normal", Entry{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.Protected(); got != tt.want {
				t.Errorf("Protected() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsMainWorktree(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main")
	separateMainPath := filepath.Join(root, "separate-main")
	linkedPath := filepath.Join(root, "linked")
	missingPath := filepath.Join(root, "missing")
	separateGitDir := filepath.Join(root, "repo.git")
	linkedGitDir := filepath.Join(separateGitDir, "worktrees", "linked")

	if err := os.MkdirAll(filepath.Join(mainPath, ".git"), 0o750); err != nil {
		t.Fatalf("mkdir main .git: %v", err)
	}
	if err := os.MkdirAll(separateMainPath, 0o750); err != nil {
		t.Fatalf("mkdir separate main: %v", err)
	}
	if err := os.MkdirAll(linkedPath, 0o750); err != nil {
		t.Fatalf("mkdir linked: %v", err)
	}
	if err := os.MkdirAll(linkedGitDir, 0o750); err != nil {
		t.Fatalf("mkdir linked gitdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(separateGitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o600); err != nil {
		t.Fatalf("write separate gitdir HEAD: %v", err)
	}
	if err := os.WriteFile(filepath.Join(separateGitDir, "config"), []byte("[core]\n"), 0o600); err != nil {
		t.Fatalf("write separate gitdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(separateMainPath, ".git"), []byte("gitdir: "+separateGitDir+"\n"), 0o600); err != nil {
		t.Fatalf("write separate main .git file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(linkedPath, ".git"), []byte("gitdir: "+linkedGitDir+"\n"), 0o600); err != nil {
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

	if err := os.WriteFile(filepath.Join(mainPath, "README.md"), []byte("hello\n"), 0o600); err != nil {
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
