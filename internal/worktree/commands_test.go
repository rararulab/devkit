package worktree

import "testing"

func TestShouldCleanEntry(t *testing.T) {
	t.Parallel()

	merged := map[string]bool{
		"merged": true,
	}

	cases := []struct {
		name  string
		entry Entry
		want  bool
	}{
		{name: "merged regular worktree", entry: Entry{Branch: "merged"}, want: true},
		{name: "main worktree", entry: Entry{Branch: "merged", IsMain: true}, want: false},
		{name: "current worktree", entry: Entry{Branch: "merged", IsCurrent: true}, want: false},
		{name: "locked worktree", entry: Entry{Branch: "merged", Locked: true}, want: false},
		{name: "detached head", entry: Entry{}, want: false},
		{name: "unmerged branch", entry: Entry{Branch: "feature"}, want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldCleanEntry(tc.entry, merged); got != tc.want {
				t.Fatalf("shouldCleanEntry(%+v) = %v, want %v", tc.entry, got, tc.want)
			}
		})
	}
}

func TestShouldNukeEntry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		entry Entry
		want  bool
	}{
		{name: "regular worktree", entry: Entry{}, want: true},
		{name: "main worktree", entry: Entry{IsMain: true}, want: false},
		{name: "current worktree", entry: Entry{IsCurrent: true}, want: false},
		{name: "locked worktree", entry: Entry{Locked: true}, want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldNukeEntry(tc.entry); got != tc.want {
				t.Fatalf("shouldNukeEntry(%+v) = %v, want %v", tc.entry, got, tc.want)
			}
		})
	}
}
