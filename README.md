# devkit

Developer toolkit CLI for [rara](https://github.com/rararulab/rara).

## Install

```bash
go install github.com/rararulab/devkit@latest
```

## Commands

| Command | Description |
|---------|-------------|
| `devkit wt` | Interactive worktree manager (TUI) |
| `devkit wt list` | List all worktrees (non-interactive) |
| `devkit wt clean` | Remove merged worktrees |
| `devkit wt nuke` | Force-remove all worktrees except main |
| `devkit check-agent-md` | Verify every crate has an AGENT.md |
| `devkit check-deps` | Check crate dependency direction rules |

## Configuration

Place a `.devkit.toml` in your repository root. See [`devkit.example.toml`](devkit.example.toml) for all options.

## License

MIT
