# wtguard

Go CLI that creates git worktrees and installs a `pre-commit` hook to **block direct commits to protected branches** (e.g. `main`) while any worktree is active. Defends against LLM/automation accidentally committing to `main`.

## Why

When using LLM-driven development, agents sometimes commit straight to `main` instead of a feature branch. `wtguard` makes that physically impossible whenever a worktree is in play: the `pre-commit` hook on the host repo refuses commits to protected branches if any worktree exists.

## Install

```sh
go install github.com/cuongtranba/wtguard/cmd/wtguard@latest
```

Or grab a binary from [Releases](https://github.com/cuongtranba/wtguard/releases).

## Quick start

```sh
cd my-repo
wtguard create feat-x          # creates ../my-repo-feat-x, prompts to install hook
cd ../my-repo-feat-x           # work here
# ... commits to feat-x branch are fine
# ... commits to main from any worktree are now BLOCKED
```

## Commands

```
wtguard install                      # drop hook into .git/hooks
wtguard uninstall                    # remove hook (restore chained hook if any)
wtguard status                       # hook status, protected branches, worktree count
wtguard create <branch> [--path P]   # git worktree add + auto-install hook
wtguard remove <path|branch>         # git worktree remove
wtguard list                         # list worktrees, mark protected branches
wtguard config get|set <key> [value]
wtguard config protected add|rm <branch>
```

## How it works

A shim at `.git/hooks/pre-commit` execs `wtguard hook pre-commit`. On commit:

1. If current branch ∉ protected list → allow.
2. If `git worktree list` has > 1 entry → **block**.
3. Otherwise → allow.

If you had a pre-existing `pre-commit` hook, it is preserved as `pre-commit.local` and chained after wtguard's check passes.

Bypass for emergencies: `WTGUARD_BYPASS=1 git commit ...` (logged).

## Configuration

All settings live in `.git/config` under `wtguard.*`:

| Key | Default |
|---|---|
| `wtguard.protected` | `main,master` |
| `wtguard.worktreeDir` | `../` |
| `wtguard.bypassLog` | `true` |
| `wtguard.chainHook` | `true` |

## Status

Early. CLI surface scaffolded; subcommand bodies land per the [design doc](docs/plans/2026-05-09-wtguard-design.md).

## License

MIT
