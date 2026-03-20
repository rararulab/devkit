// Entry point for devkit — a developer toolkit CLI for rara.
package main

import (
	"context"
	"log"
	"os"

	"github.com/rararulab/devkit/internal/agentmd"
	"github.com/rararulab/devkit/internal/deps"
	"github.com/rararulab/devkit/internal/worktree"
	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:    "devkit",
		Usage:   "Developer toolkit for rara",
		Version: Version,
		Commands: []*cli.Command{
			agentmd.Cmd(),
			worktree.Cmd(),
			deps.Cmd(),
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
