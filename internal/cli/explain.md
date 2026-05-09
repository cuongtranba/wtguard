# wtguard — LLM Quick Reference

## Purpose

Go CLI that creates git worktrees and installs a `pre-commit` hook that **blocks direct commits to protected branches** (default `main`,`master`) while any worktree is active. Defends repos against agents accidentally committing to main.

## Block rule (single source of truth)

A commit is **blocked** iff:

1. Current branch ∈ `wtguard.protected` list, **AND**
2. `git worktree list` reports more than one worktree.

Else allowed. Detached HEAD always allowed.

## Install

```sh
go install github.com/cuongtranba/wtguard/cmd/wtguard@latest
```

## Commands

| Command | Purpose |
|---|---|
| `wtguard install` | Drop pre-commit hook into `.git/hooks` (chains existing hook to `pre-commit.local`). |
| `wtguard uninstall` | Remove hook, restore chained hook if present. |
| `wtguard status` | Show hook state, protected branches, worktree count. |
| `wtguard create <branch> [--path P]` | `git worktree add` + auto-install hook (prompts unless `--yes`). |
| `wtguard remove <path\|branch> [--force]` | `git worktree remove`. |
| `wtguard list` | List worktrees, mark protected branches. |
| `wtguard config get <key>` | Read `wtguard.<key>` from `.git/config`. |
| `wtguard config set <key> <value>` | Write `wtguard.<key>` to `.git/config`. |
| `wtguard config protected add <branch>` | Append branch to protected list. |
| `wtguard config protected rm <branch>` | Remove branch from protected list. |
| `wtguard explain` | Print this LLM reference. |

## Global flags

| Flag | Default | Effect |
|---|---|---|
| `--repo <path>` | `.` | Target repo path. |
| `--yes`, `-y` | `false` | Skip interactive prompts. |
| `--verbose` | `false` | Print underlying git commands. |

## Config keys (stored in `.git/config`)

| Key | Default | Type |
|---|---|---|
| `wtguard.protected` | `main,master` | comma-separated branches |
| `wtguard.worktreeDir` | `../` | base dir for new worktrees |
| `wtguard.bypassLog` | `true` | log bypassed commits to `.git/wtguard.log` |
| `wtguard.chainHook` | `true` | run `pre-commit.local` after wtguard's check |

## Environment variables

| Var | Effect |
|---|---|
| `WTGUARD_BYPASS=1` | Skip block check for this commit (logged if `bypassLog=true`). |
| `WTGUARD_BIN` | Override binary path used by hook shim. |

## Hook shim (`.git/hooks/pre-commit`)

```sh
#!/bin/sh
# wtguard managed — do not edit
exec "${WTGUARD_BIN:-wtguard}" hook pre-commit "$@"
```

The marker comment lets `wtguard uninstall` recognize ours vs user's hook.

## Examples

```sh
# Start: install hook + create a worktree for branch feat-x
cd my-repo
wtguard create feat-x
cd ../my-repo-feat-x   # work here freely

# Try to commit on main from main worktree → blocked:
cd ../my-repo
git commit -am "wip"
# wtguard: refusing commit on 'main' — 1 active worktree(s):
#   ../my-repo-feat-x  [feat-x]
# Commit there, then merge via PR. Override: WTGUARD_BYPASS=1

# Add 'develop' to protected list
wtguard config protected add develop

# Tear down
wtguard remove feat-x
wtguard uninstall
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success / commit allowed. |
| `1` | Commit blocked by guard, or CLI usage error. |
| `2` | Internal / git invocation error. |
