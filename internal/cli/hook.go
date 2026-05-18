package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cuongtranba/wtguard/internal/audit"
	"github.com/cuongtranba/wtguard/internal/config"
	"github.com/cuongtranba/wtguard/internal/git"
	"github.com/cuongtranba/wtguard/internal/guard"
	"github.com/urfave/cli/v2"
)

func cmdHook() *cli.Command {
	return &cli.Command{
		Name:   "hook",
		Usage:  "internal: invoked by .git/hooks/pre-commit",
		Hidden: true,
		Description: "Not for direct human use. The shim at " +
			".git/hooks/pre-commit execs `wtguard hook pre-commit`.",
		Subcommands: []*cli.Command{
			{
				Name:   "pre-commit",
				Usage:  "internal: pre-commit guard entry point",
				Action: actionHookPreCommit,
			},
		},
	}
}

func actionHookPreCommit(c *cli.Context) error {
	bypass := os.Getenv("WTGUARD_BYPASS") == "1"
	repo, err := openRepo(c)
	if err != nil {
		return err
	}
	br, hasBranch, err := repo.CurrentBranch()
	if err != nil {
		return err
	}
	settings, err := config.Load(repo)
	if err != nil {
		return err
	}
	wts, err := repo.Worktrees()
	if err != nil {
		return err
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
	entry := audit.Entry{
		Repo:      repo.Root(),
		Branch:    br,
		Action:    audit.ActionCommit,
		Worktrees: len(wts),
		Layer:     audit.LayerHook,
		Reason:    decision.Reason,
	}
	if decision.Block {
		entry.Decision = audit.DecisionBlock
		_ = audit.Append(entry, audit.GlobalPath(), audit.RepoPath(repo.GitDir()))
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
		fmt.Fprint(os.Stderr, guard.FormatBlock("hook", br, decision, briefs, hint))
		return cli.Exit("", 1)
	}
	if bypass && settings.BypassLog {
		entry.Decision = audit.DecisionBypass
		entry.Bypass = true
		_ = audit.Append(entry, audit.GlobalPath(), audit.RepoPath(repo.GitDir()))
	}
	if settings.ChainHook {
		if err := chainPreCommitLocal(c.Args().Slice(), repo); err != nil {
			return err
		}
	}
	return nil
}

// chainPreCommitLocal execs pre-commit.local in the repo's hooks dir if
// it exists and is executable. The chained hook's exit code wins.
func chainPreCommitLocal(args []string, repo *git.Repo) error {
	hooksDir, err := repo.HooksDir()
	if err != nil {
		return err
	}
	chained := filepath.Join(hooksDir, "pre-commit.local")
	info, err := os.Stat(chained)
	if err != nil {
		return nil // no chained hook — pass
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return nil
	}
	cmd := exec.Command(chained, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = repo.Root()
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return cli.Exit("", exitErr.ExitCode())
		}
		return fmt.Errorf("hook: chained %s: %w", chained, err)
	}
	return nil
}
