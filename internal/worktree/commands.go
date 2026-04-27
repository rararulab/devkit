// commands.go defines CLI subcommands for worktree management.
package worktree

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/urfave/cli/v3"
)

// Cmd returns the top-level "worktree" command group.
func Cmd() *cli.Command {
	return &cli.Command{
		Name:    "worktree",
		Aliases: []string{"wt"},
		Usage:   "Manage git worktree lifecycle",
		// Default action: launch interactive TUI
		Action: func(_ context.Context, _ *cli.Command) error {
			return RunTUI()
		},
		Commands: []*cli.Command{
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "List all worktrees (non-interactive)",
				Action: func(_ context.Context, _ *cli.Command) error {
					return runList()
				},
			},
			{
				Name:  "clean",
				Usage: "Remove worktrees whose branches are merged into main (non-interactive)",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "dry-run",
						Aliases: []string{"n"},
						Usage:   "Show what would be removed without actually removing",
					},
				},
				Action: func(_ context.Context, cmd *cli.Command) error {
					return runClean(cmd.Bool("dry-run"))
				},
			},
			{
				Name:  "nuke",
				Usage: "Force-remove ALL worktrees except the main checkout (non-interactive)",
				Action: func(_ context.Context, _ *cli.Command) error {
					return runNuke()
				},
			},
		},
	}
}

func runList() error {
	entries, err := List()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No worktrees found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PATH\tBRANCH\tSTATUS\tDIRTY\tSYNC\tLAST ACTIVE\tSIZE")

	for i := range entries {
		e := &entries[i]

		branch := e.Branch
		if branch == "" {
			branch = "(detached)"
		}

		status := e.Status.String()
		var tags []string
		if e.IsMain {
			tags = append(tags, "main")
		}
		if e.IsCurrent {
			tags = append(tags, "cwd")
		}
		if e.Locked {
			tags = append(tags, "locked")
		}
		if len(tags) > 0 {
			status += " [" + strings.Join(tags, ",") + "]"
		}

		dirty := "-"
		if e.Dirty > 0 {
			dirty = fmt.Sprintf("%d changed", e.Dirty)
		}

		sync := "ok"
		if e.Ahead > 0 && e.Behind > 0 {
			sync = fmt.Sprintf("+%d/-%d", e.Ahead, e.Behind)
		} else if e.Ahead > 0 {
			sync = fmt.Sprintf("+%d ahead", e.Ahead)
		} else if e.Behind > 0 {
			sync = fmt.Sprintf("-%d behind", e.Behind)
		}

		size := humanSize(dirSize(e.Path))
		lastActive := relativeTime(e.LastActive)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			shortenPath(e.Path), branch, status, dirty, sync, lastActive, size)
	}

	return w.Flush()
}

func runClean(dryRun bool) error {
	if !dryRun {
		if err := Prune(); err != nil {
			return err
		}
	}

	merged, err := MergedBranches()
	if err != nil {
		return err
	}
	if len(merged) == 0 {
		fmt.Println("No merged branches to clean up.")
		return nil
	}

	entries, err := List()
	if err != nil {
		return err
	}

	branchHandled := make(map[string]bool)
	removed := 0
	prefix := ""
	if dryRun {
		prefix = "(dry-run) "
	}

	for _, e := range entries {
		if !shouldCleanEntry(e, merged) {
			continue
		}
		if e.Dirty > 0 {
			fmt.Fprintf(os.Stderr, "  skipping %s: %d uncommitted change(s)\n", e.Branch, e.Dirty)
			continue
		}
		fmt.Printf("%sRemoving worktree: %s (branch: %s, size: %s)\n",
			prefix, shortenPath(e.Path), e.Branch, humanSize(dirSize(e.Path)))
		if dryRun {
			branchHandled[e.Branch] = true
			removed++
			continue
		}
		if err := Remove(e.Path, false); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %s\n", err)
			continue
		}
		if err := DeleteBranch(e.Branch, false); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %s\n", err)
		}
		branchHandled[e.Branch] = true
		removed++
	}

	for branch := range merged {
		if branchHandled[branch] {
			continue
		}
		fmt.Printf("%sDeleting merged branch: %s (no worktree)\n", prefix, branch)
		if dryRun {
			removed++
			continue
		}
		if err := DeleteBranch(branch, false); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %s\n", err)
			continue
		}
		removed++
	}

	fmt.Printf("Cleaned up %d merged worktree(s)/branch(es).\n", removed)
	return nil
}

func runNuke() error {
	entries, err := List()
	if err != nil {
		return err
	}

	removed := 0
	for _, e := range entries {
		if !shouldNukeEntry(e) {
			continue
		}
		fmt.Printf("Removing: %s\n", e.Path)
		if err := Remove(e.Path, true); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %s\n", err)
			continue
		}
		if e.Branch != "" {
			if branchErr := DeleteBranch(e.Branch, true); branchErr != nil {
				fmt.Fprintf(os.Stderr, "  warning: branch delete: %s\n", branchErr)
			}
		}
		removed++
	}

	if pruneErr := Prune(); pruneErr != nil {
		fmt.Fprintf(os.Stderr, "  warning: prune: %s\n", pruneErr)
	}
	fmt.Printf("Removed %d worktree(s).\n", removed)
	return nil
}

func shouldCleanEntry(e Entry, merged map[string]bool) bool {
	return !e.Protected() && e.Branch != "" && merged[e.Branch]
}

func shouldNukeEntry(e Entry) bool {
	return !e.Protected()
}
