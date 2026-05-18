# wtguard — Proxy + Defense-in-Depth Design

**Date:** 2026-05-10
**Module:** `github.com/cuongtranba/wtguard`
**Status:** design
**Supersedes:** none — extends `2026-05-09-wtguard-design.md`

---

## Problem

Local `pre-commit` hooks alone do not protect against an LLM that:

- runs `git commit --no-verify`,
- removes the hook,
- works in a fresh clone where the hook isn't installed yet,
- pushes a feature branch directly to `main` (`git push origin HEAD:main`).

Goal: make committing or pushing to a protected branch effectively impossible without explicit human override, so an automated agent **never misses**.

---

## Approach

Three enforcement layers, all installed by a single `wtguard install` command:

1. **Proxy** — wrapper named `git`, first in PATH. Intercepts `commit` and `push`. Catches `--no-verify` and refusal to use the hook.
2. **Hook** — existing `.git/hooks/pre-commit` shim. Backstops the case where proxy is bypassed (LLM calls `/usr/bin/git` directly).
3. **GitHub branch protection** — opt-in via `--remote-protect`. Server-side rejection of direct pushes to `main`. Truly unbypassable.

Defense in depth: any one layer alone is bypassable; all three together are not.

---

## Architecture

Single Go binary, dispatched by `argv[0]`:

```
wtguard <subcommand>          # CLI mode (existing)
git     <args>                # proxy mode (symlink ~/.wtguard/bin/git → wtguard)
wtguard hook pre-commit       # hook mode (existing)
```

### Data flow on `git commit`

```
LLM runs:           git commit -m "..."
PATH first hit:     ~/.wtguard/bin/git    [Layer 1: PROXY]
                    ├─ guard.Decide()
                    ├─ block? → exit 1, structured stderr
                    └─ exec real git (PATH-strip, fallback $WTGUARD_REAL_GIT)

Real git fires:     .git/hooks/pre-commit  [Layer 2: HOOK]
                    ├─ guard.Decide()  (same code path)
                    └─ block? → exit 1
```

### Data flow on `git push origin HEAD:main`

```
LLM runs:           git push origin HEAD:main
Proxy:              parse refspecs → target=main is protected → block local
GitHub server:      branch protection rejects direct push   [Layer 3]
```

---

## Components

```
internal/
├── cli/        existing CLI subcommands
├── git/        git wrapper (existing)
├── hook/       hook install / chain (existing)
├── config/     git config keys (existing)
├── guard/      rules engine — SHARED by proxy + hook
├── proxy/      NEW — argv[0]=git dispatch, intercept commit/push, passthrough
├── shellrc/    NEW — detect + patch .zshrc / .bashrc with marker block
├── template/   NEW — set up ~/.wtguard/template + global init.templateDir
└── remote/     NEW — gh api branch protection (opt-in)
```

`cmd/wtguard/main.go` adds `argv[0]` dispatch:

```go
func main() {
    if filepath.Base(os.Args[0]) == "git" {
        os.Exit(proxy.Run(os.Args[1:]))
    }
    // existing CLI app
}
```

---

## Rules engine (`internal/guard`)

```go
type Policy string
const (
    PolicyWorktreeActive Policy = "worktree-active"
    PolicyAlways         Policy = "always" // default (changed 2026-05-13; was worktree-active)
)

type Rule struct {
    Branch        string
    Worktrees     int
    Policy        Policy
    ProtectedList []string
    BypassEnv     bool
}

type Decision struct {
    Block  bool
    Reason string
}

func Decide(r Rule) Decision
```

`Decide` is the single source of truth, called by proxy commit, proxy push, and hook.

### Policy

- `always` (default, since 2026-05-13) — block iff branch ∈ protected (regardless of worktree count).
- `worktree-active` — block iff branch ∈ protected AND worktree count > 1.

Set per repo via `git config --local wtguard.policy <value>`.

---

## Proxy internals (`internal/proxy`)

### Subcommand parsing

1. Walk args, classify global flags (`-C`, `--git-dir`, `-c key=value`) until the first non-flag token.
2. That token is the subcommand. Switch:
   - `commit` → `interceptCommit`
   - `push` → `interceptPush`
   - default → `passthrough` (bit-exact forwarding).

### `interceptCommit`

1. Resolve repo root (respect `-C`, `--git-dir`, `GIT_DIR`).
2. Read `wtguard.policy`, `wtguard.protected`.
3. Run `guard.Decide`.
4. If `WTGUARD_BYPASS=1` → log to audit, allow.
5. If block → exit 1, write structured message to stderr.
6. Else → `passthrough`. `--no-verify` is preserved (user's other hooks like linters behave normally; our rule already enforced at this layer).

### `interceptPush`

1. Parse refspecs: `<src>:<dst>`, `+<src>:<dst>`, `:<dst>` (delete), bare branch names.
2. For each, resolve `<dst>` against `wtguard.protected`.
3. Any push to a protected ref → block local. Branch protection on the server is the truth; this is fast feedback.

### Resolving real git (`internal/proxy/realgit.go`)

```go
func Resolve() (string, error) {
    if p := os.Getenv("WTGUARD_REAL_GIT"); p != "" {
        if _, err := os.Stat(p); err == nil { return p, nil }
    }
    own := filepath.Dir(must(os.Executable()))
    path := stripDir(os.Getenv("PATH"), own)
    return exec.LookPath("git", path)
}
```

`syscall.Exec` (not `exec.Cmd`) — replace process. Signals, exit code, and stdio pass through unchanged.

### Block message (stderr)

```
wtguard: blocked `git commit` on protected branch 'main'
  reason: 1 active worktree(s) — commit there instead
  worktrees:
    ../wtguard-feat-x  [feat-x]
  next:
    cd ../wtguard-feat-x && git commit ...
  override (logged): WTGUARD_BYPASS=1 git commit ...
```

---

## Install / uninstall

### `wtguard install` (no flags)

Idempotent, four steps:

1. **Symlink:** `mkdir -p ~/.wtguard/bin`; `ln -sf <wtguardBinary> ~/.wtguard/bin/git`. If existing path is a non-symlink file → refuse, message user.
2. **Shell rc:** detect `$SHELL`. Append guarded block to rc file (skip if marker present):
   ```sh
   # >>> wtguard >>>
   export PATH="$HOME/.wtguard/bin:$PATH"
   # <<< wtguard <<<
   ```
   Print `source ~/.zshrc` reminder.
3. **Init template:** `mkdir -p ~/.wtguard/template/hooks`; write hook shim there; `git config --global init.templateDir ~/.wtguard/template`. Every future `git init` / `git clone` ships with hook.
4. **Current repo (if cwd is a git repo):** drop `.git/hooks/pre-commit` shim immediately.

### `wtguard install --remote-protect`

Adds step 5: detect `gh` CLI + auth + `origin`, run

```sh
gh api -X PUT repos/{owner}/{repo}/branches/{br}/protection \
  -F required_pull_request_reviews.required_approving_review_count=1 \
  -F required_linear_history=true \
  -F enforce_admins=false
```

for each branch in `wtguard.protected`. Failures printed but do not abort the local install.

### `wtguard uninstall`

Reverse order:
- Remove repo hook (existing flow).
- `git config --global --unset init.templateDir` if it points to ours.
- Remove rc marker block (idempotent).
- Delete `~/.wtguard/bin` symlink + dir.
- `--remote-protect` is **not** auto-reversed. Print the `gh api -X DELETE ...` command for the user to run.

### `wtguard status` (extended)

```
proxy:         installed at ~/.wtguard/bin/git (PATH ok: yes)
real git:      /usr/bin/git
shell rc:      ~/.zshrc patched: yes
init template: ~/.wtguard/template (active: yes)
hook (cwd):   installed (chains pre-commit.local: no)
remote:        cuongtranba/wtguard — branch protection on main: ON
protected:     main, master
worktrees:     1 active (no extras)
```

---

## Audit log

JSON Lines at `~/.wtguard/audit.jsonl` (global) and `.git/wtguard.log` (per-repo).

```json
{"ts":"2026-05-10T03:11:22Z","repo":"/path/to/repo","branch":"main","action":"commit","decision":"block","reason":"protected+worktree","worktrees":2,"args":["commit","-am","wip"],"pid":12345,"layer":"proxy"}
```

Bypass entries also record `bypass:true` and the commit subject. Toggle via `wtguard.bypassLog`.

No rotation in v1 — user's responsibility.

---

## Testing

- **Unit:** `guard.Decide` table tests (policy × branch × worktree count × protected × bypass). `proxy/passthrough` test asserts exec args bit-equal. `shellrc` patch + unpatch idempotency. `proxy.parsePush` table tests for refspec edge cases.
- **Integration (`//go:build integration`):** real `git init` in `t.TempDir()`, real worktree add. Spawn child shell with rc patched, verify `git --version` resolves to wrapper. Real `git commit` exercises block path through proxy.
- **End-to-end:** build wtguard, symlink as `git`, prepend to PATH, run real `git commit` on protected branch, assert exit=1 + stderr matches.

Strong typing throughout — no `map[string]any`, no `interface{}`.

---

## Out of scope (YAGNI)

- Windows `git.exe` — defer.
- `pre-receive` server-side hook (GitHub branch protection covers it).
- Auto-removal of `--remote-protect` settings on uninstall.
- Audit log rotation.
- Aliasing other tools (`hub`, `lab`, `gh git`).
- Replacing `git` with the wrapper system-wide via Homebrew formula.
