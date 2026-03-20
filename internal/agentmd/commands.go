// Package agentmd defines the check-agent-md subcommand that verifies
// every crate directory has an AGENT.md file.
package agentmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/rararulab/devkit/internal/config"
	"github.com/urfave/cli/v3"
)

// Cmd returns the "check-agent-md" command.
func Cmd() *cli.Command {
	return &cli.Command{
		Name:  "check-agent-md",
		Usage: "Verify every crate has an AGENT.md file",
		Action: func(_ context.Context, _ *cli.Command) error {
			return runCheck()
		},
	}
}

// runCheck iterates the configured crates directory and reports any
// subdirectory missing AGENT.md.
func runCheck() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	cratesDir := cfg.AgentMD.CratesDir
	if cratesDir == "" {
		cratesDir = "crates"
	}

	entries, err := os.ReadDir(cratesDir)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", cratesDir, err)
	}

	var missing []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		agentPath := filepath.Join(cratesDir, e.Name(), "AGENT.md")
		if _, err := os.Stat(agentPath); os.IsNotExist(err) {
			missing = append(missing, e.Name())
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		fmt.Fprintln(os.Stderr, "ERROR: The following crates are missing AGENT.md:")
		fmt.Fprintln(os.Stderr)
		for _, name := range missing {
			fmt.Fprintf(os.Stderr, "  - %s/%s — see CLAUDE.md for template\n", cratesDir, name)
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Every crate must have an AGENT.md. See the 'AGENT.md Requirements' section in CLAUDE.md.")
		return cli.Exit("", 1)
	}

	fmt.Println("All crates have AGENT.md.")
	return nil
}
