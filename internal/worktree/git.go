// Package worktree provides low-level git worktree operations.
package worktree

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Status describes the state of a worktree entry.
type Status int

const (
	StatusActive   Status = iota // branch exists, not merged
	StatusMerged                 // branch fully merged into main
	StatusDetached               // detached HEAD (no branch)
	StatusPrunable               // stale reference, can be pruned
)

// String returns a human-readable label for the status.
func (s Status) String() string {
	switch s {
	case StatusMerged:
		return "merged"
	case StatusDetached:
		return "detached"
	case StatusPrunable:
		return "prunable"
	default:
		return "active"
	}
}

// Entry holds parsed porcelain output for a single git worktree.
type Entry struct {
	Path       string
	Branch     string // empty for detached HEAD
	IsMain     bool
	Prunable   bool
	Locked     bool // worktree has a lock file
	IsCurrent  bool // worktree is the current working directory
	Status     Status
	LastActive time.Time // last modification time of the worktree directory
	DiskSize   int64     // total disk usage in bytes
}

// Protected returns true if the worktree cannot be deleted.
func (e *Entry) Protected() bool {
	return e.IsMain || e.Locked || e.IsCurrent
}

// List parses `git worktree list --porcelain` and returns all entries,
// enriched with merge status information.
func List() ([]Entry, error) {
	out, err := exec.CommandContext(context.Background(), "git", "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	merged, err := MergedBranches()
	if err != nil {
		return nil, err
	}

	// Detect current working directory to mark the active worktree
	cwd, _ := os.Getwd()
	currentGitDir := currentWorktreeGitDir()

	var entries []Entry
	var cur Entry
	prunable := false
	locked := false

	// finalizeEntry fills computed fields and returns the entry ready for collection.
	finalizeEntry := func(e Entry) Entry {
		entryGitDir := entryGitDir(e.Path)
		e.IsMain = entryGitDir != "" && !isLinkedGitDir(entryGitDir)
		e.Prunable = prunable
		e.Locked = locked
		e.IsCurrent = isSameOrChild(cwd, e.Path) || samePath(currentGitDir, entryGitDir)
		e.Status = classifyEntry(&e, merged)
		// Populate LastActive for non-prunable entries with existing paths
		if !e.Prunable {
			if _, err := os.Stat(e.Path); err == nil {
				e.LastActive = lastActiveTime(e.Path)
			}
		}
		return e
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = Entry{Path: strings.TrimPrefix(line, "worktree ")}
			prunable = false
			locked = false
		case strings.HasPrefix(line, "branch refs/heads/"):
			cur.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "prunable":
			prunable = true
		case line == "locked", strings.HasPrefix(line, "locked "):
			locked = true
		case line == "":
			if cur.Path != "" {
				entries = append(entries, finalizeEntry(cur))
			}
			cur = Entry{}
		}
	}
	if cur.Path != "" {
		entries = append(entries, finalizeEntry(cur))
	}
	return entries, nil
}

// dirSize computes total disk usage of a directory tree in bytes.
// Returns 0 on any error.
func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// lastActiveTime returns the most recent modification time among key git files
// in the worktree, providing a meaningful "last active" signal.
// In linked worktrees, .git is a file containing "gitdir: <path>" — this function
// resolves the actual git directory to find HEAD and index files.
// Falls back to the directory mtime if no git files are found.
func lastActiveTime(path string) time.Time {
	var latest time.Time

	// Resolve the actual git directory (handles both main checkout and linked worktrees)
	gitDir := entryGitDir(path)
	var candidates []string
	if gitDir != "" {
		candidates = append(candidates,
			filepath.Join(gitDir, "HEAD"),
			filepath.Join(gitDir, "index"),
		)
	}
	candidates = append(candidates, filepath.Join(path, ".git")) // mtime of .git itself (file or dir)
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil {
			if info.ModTime().After(latest) {
				latest = info.ModTime()
			}
		}
	}
	// Fall back to directory mtime
	if latest.IsZero() {
		if info, err := os.Stat(path); err == nil {
			latest = info.ModTime()
		}
	}
	return latest
}

func currentWorktreeGitDir() string {
	out, err := exec.CommandContext(context.Background(), "git", "rev-parse", "--absolute-git-dir").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func entryGitDir(path string) string {
	if isGitDir(path) {
		return filepath.Clean(path)
	}
	dotGit := filepath.Join(path, ".git")
	if _, err := os.Stat(dotGit); err != nil {
		return ""
	}
	return resolveGitDir(path)
}

func isGitDir(path string) bool {
	headInfo, headErr := os.Stat(filepath.Join(path, "HEAD"))
	configInfo, configErr := os.Stat(filepath.Join(path, "config"))
	return headErr == nil && !headInfo.IsDir() && configErr == nil && !configInfo.IsDir()
}

// resolveGitDir returns the path to the actual git directory for a worktree.
// The worktree's .git metadata may be either a directory or a file containing
// "gitdir: <path>" that points to the actual git metadata directory.
func resolveGitDir(worktreePath string) string {
	dotGit := filepath.Join(worktreePath, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		return dotGit
	}
	// .git can be a directory or a file pointing at the real git metadata.
	if info.IsDir() {
		return dotGit
	}
	data, err := os.ReadFile(dotGit)
	if err != nil {
		return dotGit
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir: ") {
		return dotGit
	}
	gitdir := strings.TrimPrefix(line, "gitdir: ")
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(worktreePath, gitdir)
	}
	return gitdir
}

func isMainWorktree(worktreePath string) bool {
	gitDir := entryGitDir(worktreePath)
	if gitDir == "" {
		return false
	}
	return !isLinkedGitDir(gitDir)
}

func isLinkedGitDir(gitDir string) bool {
	gitDir = filepath.Clean(gitDir)
	return filepath.Base(filepath.Dir(gitDir)) == "worktrees"
}

func classifyEntry(e *Entry, merged map[string]bool) Status {
	if e.Prunable {
		return StatusPrunable
	}
	if e.Branch == "" {
		return StatusDetached
	}
	if merged[e.Branch] {
		return StatusMerged
	}
	return StatusActive
}

// isSameOrChild returns true if child is the same as or under parent directory.
func isSameOrChild(child, parent string) bool {
	c, err1 := filepath.EvalSymlinks(child)
	p, err2 := filepath.EvalSymlinks(parent)
	if err1 != nil || err2 != nil {
		return child == parent
	}
	return c == p || strings.HasPrefix(c, p+string(os.PathSeparator))
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	x, err1 := filepath.EvalSymlinks(a)
	y, err2 := filepath.EvalSymlinks(b)
	if err1 != nil || err2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return x == y
}

// MergedBranches returns branch names that are fully merged into main.
func MergedBranches() (map[string]bool, error) {
	out, err := exec.CommandContext(context.Background(), "git", "branch", "--merged", "main", "--format=%(refname:short)").Output()
	if err != nil {
		return nil, fmt.Errorf("git branch --merged: %w", err)
	}
	m := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		b := strings.TrimSpace(scanner.Text())
		if b != "" && b != "main" {
			m[b] = true
		}
	}
	return m, nil
}

// Prune runs `git worktree prune` to clean stale references.
func Prune() error {
	if out, err := exec.CommandContext(context.Background(), "git", "worktree", "prune").CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree prune: %w\n%s", err, out)
	}
	return nil
}

// Remove removes a worktree at the given path.
func Remove(path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	if out, err := exec.CommandContext(context.Background(), "git", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove %s: %w\n%s", path, err, out)
	}
	return nil
}

// DeleteBranch deletes a local branch. If force is true, uses -D instead of -d.
func DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	if out, err := exec.CommandContext(context.Background(), "git", "branch", flag, name).CombinedOutput(); err != nil {
		return fmt.Errorf("git branch %s %s: %w\n%s", flag, name, err, out)
	}
	return nil
}
