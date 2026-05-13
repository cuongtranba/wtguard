// Package proxy implements wtguard's git-wrapper mode.
//
// When the binary is invoked as `git` (via the ~/.wtguard/bin/git symlink),
// it parses argv to find the subcommand, intercepts `commit` and `push` to
// the protected branches, and otherwise execs the real git transparently.
package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/cuongtranba/wtguard/internal/audit"
	"github.com/cuongtranba/wtguard/internal/config"
	"github.com/cuongtranba/wtguard/internal/git"
	"github.com/cuongtranba/wtguard/internal/guard"
)

// Run is the entry point. Returns the process exit code (0 on allow,
// 1 on block, or whatever the real git returns).
//
// Args is the slice after argv[0] — i.e. everything the user typed after
// `git`.
func Run(args []string) int {
	gargs := parseGlobalFlags(args)
	// `git --help <sub>` and `git --version` are terminal: git prints docs
	// and never reaches the subcommand. Skip intercept so wtguard doesn't
	// shadow the user's request for help.
	if gargs.hasTerminalGlobalFlag() {
		return passthrough(args)
	}
	switch gargs.Subcommand {
	case "commit":
		return interceptCommit(gargs)
	case "push":
		return interceptPush(gargs)
	default:
		return passthrough(args)
	}
}

func (g GlobalArgs) hasTerminalGlobalFlag() bool {
	for _, f := range g.GlobalFlags {
		switch f {
		case "--help", "-h", "--version":
			return true
		}
	}
	return false
}

// GlobalArgs is the result of parseGlobalFlags. It separates git's own
// pre-subcommand flags (-C, --git-dir, -c key=val) from the subcommand and
// its arguments.
type GlobalArgs struct {
	GlobalFlags []string // flags before the subcommand, in order
	WorkDir     string   // value of `git -C <dir>`, last wins
	GitDir      string   // value of `git --git-dir <dir>`, last wins
	Subcommand  string   // empty if argv is just global flags
	SubArgs     []string // everything after the subcommand
	Raw         []string // the original args (for passthrough)
}

func parseGlobalFlags(args []string) GlobalArgs {
	g := GlobalArgs{Raw: args}
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--":
			// rest is positional to the subcommand
			// but we already need a subcommand to be here.
			i++
			if i < len(args) {
				g.Subcommand = args[i]
				g.SubArgs = args[i+1:]
			}
			return g
		case a == "-C":
			if i+1 < len(args) {
				g.WorkDir = args[i+1]
				g.GlobalFlags = append(g.GlobalFlags, a, args[i+1])
				i += 2
				continue
			}
			i++
		case strings.HasPrefix(a, "--git-dir="):
			g.GitDir = strings.TrimPrefix(a, "--git-dir=")
			g.GlobalFlags = append(g.GlobalFlags, a)
			i++
		case a == "--git-dir":
			if i+1 < len(args) {
				g.GitDir = args[i+1]
				g.GlobalFlags = append(g.GlobalFlags, a, args[i+1])
				i += 2
				continue
			}
			i++
		case a == "-c":
			if i+1 < len(args) {
				g.GlobalFlags = append(g.GlobalFlags, a, args[i+1])
				i += 2
				continue
			}
			i++
		case strings.HasPrefix(a, "--"):
			// long flag (--help, --version, --no-pager, --paginate, ...)
			g.GlobalFlags = append(g.GlobalFlags, a)
			i++
		case strings.HasPrefix(a, "-") && len(a) > 1:
			// short flag block — treat as global (we don't need to know
			// exhaustively; non-git-global flags will simply not be ours).
			g.GlobalFlags = append(g.GlobalFlags, a)
			i++
		default:
			g.Subcommand = a
			g.SubArgs = args[i+1:]
			return g
		}
	}
	return g
}

// passthrough execs the real git with the original argv. On success the
// current process is replaced.
func passthrough(args []string) int {
	real, err := resolveRealGit()
	if err != nil {
		fmt.Fprintln(os.Stderr, "wtguard:", err)
		return 127
	}
	if err := execReal(real, args); err != nil {
		fmt.Fprintln(os.Stderr, "wtguard: exec git:", err)
		return 127
	}
	// unreachable on success (syscall.Exec replaces process)
	return 0
}

// interceptCommit applies the block rule before forwarding to real git.
func interceptCommit(g GlobalArgs) int {
	bypass := os.Getenv("WTGUARD_BYPASS") == "1"
	repoPath := chooseRepoPath(g)
	repo, err := git.Open(repoPath)
	if err != nil {
		// Not a git repo? Let real git handle the error.
		return passthrough(g.Raw)
	}
	br, hasBranch, err := repo.CurrentBranch()
	if err != nil {
		return passthrough(g.Raw)
	}
	settings, err := config.Load(repo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wtguard: config:", err)
		return 1
	}
	wts, err := repo.Worktrees()
	if err != nil {
		fmt.Fprintln(os.Stderr, "wtguard: worktree list:", err)
		return 1
	}
	rule := guard.Rule{
		Branch:        br,
		Detached:      !hasBranch,
		Worktrees:     len(wts),
		Policy:        settings.Policy,
		ProtectedList: settings.Protected,
		BypassEnv:     bypass,
	}
	decision := guard.Decide(rule)
	logEntry := audit.Entry{
		Repo:      repo.Root(),
		Branch:    br,
		Action:    audit.ActionCommit,
		Worktrees: len(wts),
		Args:      g.Raw,
		Layer:     audit.LayerProxy,
		Reason:    decision.Reason,
	}
	if !decision.Block {
		if bypass && settings.BypassLog {
			logEntry.Decision = audit.DecisionBypass
			logEntry.Bypass = true
			_ = audit.Append(logEntry, audit.GlobalPath(), audit.RepoPath(repo.GitDir()))
		}
		return passthrough(g.Raw)
	}
	logEntry.Decision = audit.DecisionBlock
	_ = audit.Append(logEntry, audit.GlobalPath(), audit.RepoPath(repo.GitDir()))

	// Build a useful next-hint pointing at the first non-current worktree.
	var briefs []guard.WorktreeBrief
	var hint string
	for _, w := range wts {
		if w.Path == repo.Root() {
			continue
		}
		briefs = append(briefs, guard.WorktreeBrief{Path: w.Path, Branch: w.Branch})
		if hint == "" {
			hint = fmt.Sprintf("cd %s && git commit ...", w.Path)
		}
	}
	fmt.Fprint(os.Stderr, guard.FormatBlock("proxy", br, decision, briefs, hint))
	return 1
}

// interceptPush checks the destination of each refspec. Any push targeting
// a protected ref is blocked locally; the server-side branch protection
// (when enabled) is the ultimate authority.
func interceptPush(g GlobalArgs) int {
	repoPath := chooseRepoPath(g)
	repo, err := git.Open(repoPath)
	if err != nil {
		return passthrough(g.Raw)
	}
	settings, err := config.Load(repo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wtguard: config:", err)
		return 1
	}
	bypass := os.Getenv("WTGUARD_BYPASS") == "1"
	br, hasBranch, _ := repo.CurrentBranch()
	targets := ParsePush(g.SubArgs, br, hasBranch)
	for _, t := range targets {
		for _, p := range settings.Protected {
			if t == p {
				logEntry := audit.Entry{
					Repo:     repo.Root(),
					Branch:   br,
					Action:   audit.ActionPush,
					Args:     g.Raw,
					Layer:    audit.LayerProxy,
					Reason:   fmt.Sprintf("push to protected ref %q", t),
					Decision: audit.DecisionBlock,
				}
				if bypass && settings.BypassLog {
					logEntry.Decision = audit.DecisionBypass
					logEntry.Bypass = true
					_ = audit.Append(logEntry, audit.GlobalPath(), audit.RepoPath(repo.GitDir()))
					return passthrough(g.Raw)
				}
				_ = audit.Append(logEntry, audit.GlobalPath(), audit.RepoPath(repo.GitDir()))
				fmt.Fprintf(os.Stderr,
					"wtguard: blocked `git push` to protected ref %q\n"+
						"  open a PR instead, or set WTGUARD_BYPASS=1 to override (logged).\n",
					t)
				return 1
			}
		}
	}
	return passthrough(g.Raw)
}

// chooseRepoPath picks the directory we should resolve the repo from.
func chooseRepoPath(g GlobalArgs) string {
	if g.WorkDir != "" {
		return g.WorkDir
	}
	if g.GitDir != "" {
		return g.GitDir
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func execReal(real string, args []string) error {
	argv := append([]string{real}, args...)
	return syscall.Exec(real, argv, os.Environ())
}

// resolveRealGit finds the real git binary, with three strategies:
//  1. $WTGUARD_REAL_GIT if set and exists.
//  2. PATH with our own directory stripped.
//  3. Fallback to exec.LookPath (cannot find ourselves because args[0] is
//     our basename — but if a sibling is the same path we already lost).
func resolveRealGit() (string, error) {
	if p := os.Getenv("WTGUARD_REAL_GIT"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	own, err := os.Executable()
	if err == nil {
		ownDir := filepath.Dir(own)
		stripped := stripPath(os.Getenv("PATH"), ownDir)
		if p, err := lookPathIn("git", stripped); err == nil && p != own {
			return p, nil
		}
	}
	p, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("real `git` not found in PATH (set WTGUARD_REAL_GIT)")
	}
	return p, nil
}

// stripPath removes any path entry equal to dir from a PATH-style colon list.
func stripPath(path, dir string) string {
	if dir == "" {
		return path
	}
	parts := strings.Split(path, string(os.PathListSeparator))
	out := parts[:0]
	for _, p := range parts {
		if p == "" {
			continue
		}
		if p == dir || filepath.Clean(p) == filepath.Clean(dir) {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, string(os.PathListSeparator))
}

// lookPathIn is exec.LookPath but with a caller-provided PATH.
func lookPathIn(name, path string) (string, error) {
	for dir := range strings.SplitSeq(path, string(os.PathListSeparator)) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		if info.Mode()&0o111 == 0 {
			continue
		}
		return candidate, nil
	}
	return "", fmt.Errorf("not found in PATH: %s", name)
}
