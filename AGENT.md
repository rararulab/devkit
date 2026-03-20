# devkit — Agent Guidelines

## Purpose
Developer toolkit CLI for rara — provides worktree management TUI, AGENT.md presence checks, and crate dependency direction linting.

## Architecture
- `main.go` — entry point, wires CLI commands via urfave/cli v3
- `internal/config/` — reads `.devkit.toml` from repo root (walks up from cwd)
- `internal/worktree/` — git worktree operations + bubbletea v2 TUI
- `internal/agentmd/` — checks that every crate directory has an AGENT.md
- `internal/deps/` — enforces dependency layer rules from config

## Critical Invariants
- Must use bubbletea v2 (`charm.land/bubbletea/v2`), NOT v1.
- Must use urfave/cli v3, NOT cobra.
- Layer rules and crate directories come from `.devkit.toml`, not hardcoded.
- The `worktree` package calls git CLI directly via `os/exec` — no git library.

## What NOT To Do
- Do NOT hardcode layer maps or crate paths — read from `.devkit.toml`.
- Do NOT use bubbletea v1 or cobra — the project uses v2 and urfave/cli v3.
- Do NOT add external git libraries — shell out to git CLI for simplicity.

## Dependencies
- Upstream: none (standalone CLI tool)
- Downstream: consumed by rara repo via `go install` + justfile integration
