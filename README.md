# wtguard

Go CLI that **physically prevents commits to protected branches** (`main`, `master`, …) when an LLM or human is supposed to be working in a worktree. Three layers of enforcement so an automated agent **never misses**.

## Why

LLM-driven development agents occasionally `git commit` straight to `main` instead of a feature branch. A plain `pre-commit` hook is not enough: agents can pass `--no-verify`, delete the hook, or work in a fresh clone. `wtguard` closes those holes by stacking three independent defenses behind a single install command.

## Defense in depth

```
LLM runs `git commit ...`
   │
   ├─[1] Proxy        ~/.wtguard/bin/git is first in PATH.
   │                  Catches `--no-verify` and missing hooks.
   │
   ├─[2] Hook         .git/hooks/pre-commit checks the same rule.
   │                  Catches the case where proxy is bypassed
   │                  (e.g. /usr/bin/git called directly).
   │
   └─[3] Branch       GitHub branch protection rejects direct
        protection    pushes to `main`. Server-side, unbypassable.
        (opt-in)
```

Each layer alone is bypassable. All three together are not.

## Install

```sh
go install github.com/cuongtranba/wtguard/cmd/wtguard@latest
wtguard install                  # layers 1 + 2 + global init template
wtguard install --remote-protect # also apply layer 3 via `gh api`
```

Or grab a binary from [Releases](https://github.com/cuongtranba/wtguard/releases).

`wtguard install` is idempotent and does the following:

1. Symlinks `~/.wtguard/bin/git` → `wtguard`.
2. Patches your shell rc with `export PATH="$HOME/.wtguard/bin:$PATH"` (guarded by a marker block, safe to re-run).
3. Sets `git config --global init.templateDir ~/.wtguard/template` so every new `git init` / `git clone` ships with the hook.
4. Drops the hook into the current repo.

`wtguard uninstall` reverses 1–4 (branch protection on the remote stays — it affects shared resources).

## Quick start

```sh
cd my-repo
wtguard create feat-x          # creates ../my-repo-feat-x, installs hook
cd ../my-repo-feat-x           # work here

# ... commits on feat-x are fine.
# ... commits on main from anywhere are BLOCKED:
#
#   wtguard: blocked `git commit` on protected branch 'main'
#     reason: 1 active worktree(s) — commit there instead
#     worktrees:
#       ../my-repo-feat-x  [feat-x]
#     next:
#       cd ../my-repo-feat-x && git commit ...
```

## Commands

```
wtguard install [--remote-protect]   # set up proxy + hook + (opt) branch protection
wtguard uninstall                    # remove proxy + hook
wtguard status                       # what's installed, where, on what
wtguard create <branch> [--path P]   # git worktree add + ensure hook installed
wtguard remove <path|branch>         # git worktree remove
wtguard list                         # list worktrees, mark protected branches
wtguard config get|set <key> [value] # read / write wtguard.<key>
wtguard config protected add|rm <branch>
wtguard explain                      # full LLM-friendly reference (markdown)
```

## How it works

### Layer 1 — proxy

A symlink at `~/.wtguard/bin/git` points to the `wtguard` binary. Because that directory is first on `PATH`, every `git` call goes through `wtguard`. The wrapper inspects the subcommand:

- `git commit` and `git push` to a protected ref → block with a structured stderr message and a non-zero exit code.
- everything else → forwarded bit-for-bit to the real `git` (resolved via PATH-strip, with `WTGUARD_REAL_GIT` as override).

The proxy refuses regardless of `--no-verify` because it runs *before* git invokes any hooks.

### Layer 2 — hook

A `pre-commit` shim in `.git/hooks` (and in `~/.wtguard/template/hooks` for new clones) execs `wtguard hook pre-commit` and applies the same rule. Catches the case where someone invokes `/usr/bin/git` directly, bypassing PATH.

If you already had a `pre-commit` hook, it is preserved as `pre-commit.local` and chained after the guard check.

### Layer 3 — GitHub branch protection (opt-in)

`wtguard install --remote-protect` calls `gh api` to require pull requests for `main` and friends. Even if both client layers are bypassed, the server rejects the push. This is the only truly unbypassable layer.

### The block rule

```
block iff:  current branch ∈ wtguard.protected
       AND  git worktree list reports > 1 entry   (default policy)
```

Set `wtguard.policy = always` to drop the worktree clause and protect `main` even when no worktree is open.

### Bypass

Real emergency: `WTGUARD_BYPASS=1 git commit ...` skips the check at both proxy and hook layers. Bypasses are appended to `~/.wtguard/audit.jsonl` and `.git/wtguard.log` with timestamp, repo, and commit subject.

## Configuration

All settings live in `.git/config` under `wtguard.*`:

| Key | Default | Notes |
|---|---|---|
| `wtguard.protected` | `main,master` | comma-separated branch list |
| `wtguard.policy` | `worktree-active` | or `always` |
| `wtguard.worktreeDir` | `../` | base dir for new worktrees |
| `wtguard.bypassLog` | `true` | log bypassed commits |
| `wtguard.chainHook` | `true` | run `pre-commit.local` after guard check |

Environment variables:

| Var | Effect |
|---|---|
| `WTGUARD_BYPASS=1` | skip block for one commit (logged) |
| `WTGUARD_REAL_GIT` | absolute path to the real `git` binary (proxy override) |

## Status

Early. Repo scaffolded with release-please + GoReleaser; design docs in [`docs/plans/`](docs/plans/). Subcommand bodies land per the proxy / defense-in-depth design [2026-05-10](docs/plans/2026-05-10-wtguard-proxy-design.md).

## License

MIT
