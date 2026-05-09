# wtguard — Design

**Date:** 2026-05-09
**Module:** `github.com/cuongtranba/wtguard`
**Purpose:** Go CLI that creates git worktrees and installs a `pre-commit` hook on the host repo that **blocks direct commits to protected branches (e.g. `main`) while any worktree is active**. Defends against LLM/automation accidentally committing to `main`.

---

## 1. Architecture

Single Go binary, two modes:

1. **CLI mode** — user runs `wtguard <subcommand>`. Built with `urfave/cli/v2`.
2. **Hook mode** — git invokes a shim that execs `wtguard hook pre-commit`. Hidden subcommand.

### Components

- `cmd/wtguard/main.go` — entry, urfave/cli/v2 App.
- `internal/cli/` — subcommand handlers.
- `internal/git/` — `os/exec` wrapper around `git`.
- `internal/hook/` — install / chain / uninstall logic.
- `internal/config/` — typed wrappers over `git config wtguard.*`.
- `internal/guard/` — block decision (the single source of truth).

### Data flow on `git commit`

```
git commit (on main repo, branch=main)
  → .git/hooks/pre-commit (shim, 3 lines of sh)
  → exec wtguard hook pre-commit
  → guard.Check(): branch ∈ protected? AND `git worktree list` count > 1?
  → block (exit 1, message) OR pass-through to pre-commit.local
```

### Storage

No JSON / YAML config file. All settings live in `.git/config` under `wtguard.*` keys:

| Key | Default | Type |
|---|---|---|
| `wtguard.protected` | `main,master` | comma-list |
| `wtguard.worktreeDir` | `../` | path |
| `wtguard.bypassLog` | `true` | bool |
| `wtguard.chainHook` | `true` | bool |

---

## 2. CLI surface

```
wtguard install                     # drop hook + shim into .git/hooks
wtguard uninstall                   # remove shim, restore chained hook if any
wtguard status                      # hook installed?, protected list, worktree count
wtguard create <branch> [--path P]  # git worktree add + auto-install hook (prompt)
wtguard remove <path|branch>        # git worktree remove + cleanup
wtguard list                        # wrap `git worktree list`, mark protected
wtguard config get <key>
wtguard config set <key> <val>
wtguard config protected add <br>
wtguard config protected rm  <br>
wtguard hook pre-commit             # internal, hidden, called by shim
```

Global flags: `--repo <path>` (default cwd), `--yes`, `--verbose`.

---

## 3. Hook install + chaining

### Shim content (`.git/hooks/pre-commit`, mode 0755)

```sh
#!/bin/sh
# wtguard managed — do not edit
exec "${WTGUARD_BIN:-wtguard}" hook pre-commit "$@"
```

Marker comment lets `status`/`uninstall` recognize ours vs user's.

### Install algorithm

1. `git rev-parse --git-path hooks` → hooks dir.
2. Path = `<hooksDir>/pre-commit`.
3. If absent → write shim, chmod 0755.
4. If present → read first lines:
   - Contains `wtguard managed` → no-op (or `--force` rewrite).
   - Else → user hook present:
     - Move existing → `pre-commit.local` (preserve mode).
     - If `pre-commit.local` already exists → error, ask user resolve.
     - Write our shim.

### Hook execution path

1. Run guard check.
2. **Block path:** print message to stderr, exit 1.
3. **Allow path:** if `pre-commit.local` exists + executable → exec it (its exit code wins). Else exit 0.

### Bypass

`WTGUARD_BYPASS=1` skips check. Logged to `.git/wtguard.log` (timestamp + commit msg subject) when `wtguard.bypassLog=true`.

### Uninstall

1. Read shim, verify marker. Not ours → refuse.
2. Delete shim.
3. If `pre-commit.local` exists → rename back to `pre-commit`, preserve mode.

---

## 4. Block decision

Hook logic (`internal/guard/check.go`):

1. `git symbolic-ref --short HEAD` → current branch. Detached HEAD → allow.
2. Branch ∉ `wtguard.protected` → allow.
3. `git worktree list --porcelain` → count entries.
4. Count > 1 → **block**. Else allow.

Rule is uniform regardless of worktree count (1 or 10 extra worktrees both block).

### Block message

```
wtguard: refusing commit on 'main' — 2 active worktree(s):
  ../wtguard-feat-x  [feat-x]
  ../wtguard-bugfix  [bugfix]
Commit there, then merge via PR. Override: WTGUARD_BYPASS=1
```

---

## 5. Git interface

`internal/git`:

```go
type Repo struct{ root string }

func Open(path string) (*Repo, error)
func (r *Repo) HooksDir() (string, error)
func (r *Repo) CurrentBranch() (string, bool, error) // bool = false on detached HEAD
func (r *Repo) Worktrees() ([]Worktree, error)
func (r *Repo) AddWorktree(path, branch string, create bool) error
func (r *Repo) RemoveWorktree(path string, force bool) error
func (r *Repo) ConfigGet(key string) (string, bool, error)
func (r *Repo) ConfigSet(key, val string) error

type Worktree struct {
    Path     string
    Branch   string
    Head     string
    Bare     bool
    Detached bool
}
```

Shells out to `git`. No `go-git`.

---

## 6. Testing

- **Unit:** porcelain parser table tests, guard decision matrix (branch × worktree count × protected list), hook install/chain matrix.
- **Integration (`//go:build integration`):** real `git init` in `t.TempDir()`, real worktree add, exercise commit-block path.
- **End-to-end:** install shim pointing to test-built `wtguard`, perform `git commit`, assert exit code + stderr.

Strong typing throughout — no `map[string]any`, no `interface{}`.

---

## 7. Project layout

```
wtguard/
├── cmd/wtguard/main.go
├── internal/
│   ├── cli/      # one file per subcommand
│   ├── git/
│   ├── hook/
│   ├── config/
│   └── guard/
├── docs/plans/
├── .github/workflows/
├── .goreleaser.yaml
├── release-please-config.json
├── .release-please-manifest.json
├── go.mod
└── README.md
```

---

## 8. Ship

- `go install github.com/cuongtranba/wtguard/cmd/wtguard@latest` — primary.
- **Release-please** parses Conventional Commits on `main`, opens release PR, tags `vX.Y.Z` on merge.
- **GoReleaser** triggered by tag push, builds darwin/linux × amd64/arm64, attaches archives to GitHub Release.

---

## 9. Out of scope (YAGNI)

- Windows native shim (git-bash works for v0).
- `doctor` command.
- Daemon / file-watching modes.
- Auto-cherry-picking blocked commits onto worktree branch.
- Homebrew tap (later).
