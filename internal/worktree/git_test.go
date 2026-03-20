package worktree

import (
	"os"
	"path/filepath"
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
