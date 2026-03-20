# Ralph Agent Instructions — devkit

You are an autonomous coding agent working on **devkit**, a Go CLI tool for the rara project.

## Project Context

- **Module**: `github.com/rararulab/devkit`
- **Go**: 1.26.1
- **CLI framework**: urfave/cli v3 (NOT cobra)
- **TUI framework**: bubbletea v2 (NOT v1), bubbles v2, lipgloss v2
- **Config**: `.devkit.toml` parsed with go-toml/v2
- **Git operations**: shell out to git CLI via `os/exec` — no git libraries

## Critical Invariants

- Must use bubbletea v2 (`charm.land/bubbletea/v2`), NOT v1
- Must use urfave/cli v3, NOT cobra
- Layer rules and crate directories come from `.devkit.toml`, not hardcoded
- Do NOT add external git libraries — shell out to git CLI

## Your Task

1. Read the PRD at `scripts/ralph/prd.json`
2. Read the progress log at `scripts/ralph/progress.txt` (check Codebase Patterns section first)
3. Check you're on the correct branch from PRD `branchName`. If not, check it out or create from main.
4. Pick the **highest priority** user story where `passes: false`
5. Implement that single user story
6. Run quality checks (see below)
7. Update AGENT.md if you discover reusable patterns
8. If checks pass, commit ALL changes with message: `feat: [Story ID] - [Story Title]`
9. Update the PRD to set `passes: true` for the completed story
10. Append your progress to `scripts/ralph/progress.txt`

## Quality Checks (MUST ALL PASS)

Run these before committing:

```bash
just fmt-check        # gofmt + goimports formatting
just lint             # golangci-lint with 21 linters
just test             # go test with race detector
go vet ./...          # static analysis
go build ./...        # compilation check
```

If any check fails, fix the issue before committing.

Key linter settings:
- goimports local prefix: `github.com/rararulab/devkit`
- funlen: max 60 statements
- gocyclo: max complexity 20
- line length: 140 chars
- Formatters: gofmt, goimports

## Progress Report Format

APPEND to `scripts/ralph/progress.txt` (never replace):
```
## [Date/Time] - [Story ID]
- What was implemented
- Files changed
- **Learnings for future iterations:**
  - Patterns discovered
  - Gotchas encountered
  - Useful context
---
```

## Consolidate Patterns

If you discover a **reusable pattern**, add it to the `## Codebase Patterns` section at the TOP of progress.txt:

```
## Codebase Patterns
- Use urfave/cli v3 action signature: func(ctx context.Context, cmd *cli.Command) error
- TUI colors defined in internal/worktree/tui.go palette
- Config loading walks up directory tree to find .devkit.toml
```

## Update AGENT.md

Before committing, check if edited directories have learnings worth preserving in `AGENT.md`:
- API patterns or conventions specific to that module
- Gotchas or non-obvious requirements
- Dependencies between files

Do NOT add story-specific details or temporary notes.

## Stop Condition

After completing a story, check if ALL stories have `passes: true`.

If ALL complete: reply with `<promise>COMPLETE</promise>`

If stories remain with `passes: false`: end your response normally.

## Important

- Work on ONE story per iteration
- Commit frequently
- Keep CI green
- Read Codebase Patterns in progress.txt before starting
- ALL commits must pass quality checks
- Do NOT commit broken code
