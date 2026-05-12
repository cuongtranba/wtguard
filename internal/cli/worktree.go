package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuongtranba/wtguard/internal/config"
	"github.com/cuongtranba/wtguard/internal/git"
	"github.com/cuongtranba/wtguard/internal/hook"
	"github.com/urfave/cli/v2"
)

func cmdCreate() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "create a worktree (auto-installs hook if missing)",
		ArgsUsage: "<branch>",
		Description: "Runs `git worktree add` for the given branch. If the " +
			"hook is not yet installed, prompts to install it (or pass --yes). " +
			"Default path: '<wtguard.worktreeDir><repoName>-<branch>'.",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "path", Usage: "explicit worktree path"},
		},
		Action: actionCreate,
	}
}

func actionCreate(c *cli.Context) error {
	branch := c.Args().First()
	if branch == "" {
		return fmt.Errorf("create: branch name required")
	}
	repo, err := openRepo(c)
	if err != nil {
		return err
	}
	settings, err := config.Load(repo)
	if err != nil {
		return err
	}
	wtPath := c.String("path")
	if wtPath == "" {
		base := filepath.Base(repo.Root())
		wtPath = filepath.Join(repo.Root(), settings.WorktreeDir, fmt.Sprintf("%s-%s", base, branch))
		wtPath = filepath.Clean(wtPath)
	}
	// auto-install hook if missing
	hooksDir, err := repo.HooksDir()
	if err != nil {
		return err
	}
	if installed, _ := hook.IsInstalled(hooksDir); !installed {
		if c.Bool("yes") || promptYN(fmt.Sprintf("hook not installed in %s; install now?", repo.Root())) {
			res, err := hook.Install(hooksDir, false)
			if err != nil {
				return err
			}
			if res.Wrote {
				fmt.Printf("hook:     %s installed\n", res.HookPath)
			}
		}
	}
	// Decide whether to create the branch or check it out.
	createBranch := !branchExists(repo, branch)
	if err := repo.AddWorktree(wtPath, branch, createBranch); err != nil {
		return err
	}
	fmt.Printf("worktree: %s @ %s\n", wtPath, branch)
	return nil
}

func cmdRemove() *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Usage:     "remove a worktree by path or branch",
		ArgsUsage: "<path|branch>",
		Description: "Runs `git worktree remove`. Argument may be a path or a " +
			"branch name (resolved via `git worktree list`). Pass --force to " +
			"remove worktrees with uncommitted changes.",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "force", Usage: "force removal"},
		},
		Action: actionRemove,
	}
}

func actionRemove(c *cli.Context) error {
	target := c.Args().First()
	if target == "" {
		return fmt.Errorf("remove: path or branch required")
	}
	repo, err := openRepo(c)
	if err != nil {
		return err
	}
	wts, err := repo.Worktrees()
	if err != nil {
		return err
	}
	path := target
	if _, statErr := os.Stat(target); statErr != nil {
		// Not a path — treat as branch.
		path = ""
		for _, w := range wts {
			if w.Branch == target {
				path = w.Path
				break
			}
		}
		if path == "" {
			return fmt.Errorf("remove: no worktree matching path or branch %q", target)
		}
	}
	if err := repo.RemoveWorktree(path, c.Bool("force")); err != nil {
		return err
	}
	fmt.Printf("removed:  %s\n", path)
	return nil
}

func cmdList() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "list worktrees, mark protected branches",
		Description: "One worktree per line with branch, HEAD, and a " +
			"`(protected)` marker for branches in wtguard.protected.",
		Action: actionList,
	}
}

func actionList(c *cli.Context) error {
	repo, err := openRepo(c)
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
	for _, w := range wts {
		marker := ""
		for _, p := range settings.Protected {
			if p == w.Branch {
				marker = " (protected)"
			}
		}
		switch {
		case w.Detached:
			fmt.Printf("%s  (detached @ %s)\n", w.Path, w.Head)
		case w.Bare:
			fmt.Printf("%s  (bare)\n", w.Path)
		default:
			fmt.Printf("%s  [%s] %s%s\n", w.Path, w.Branch, w.Head, marker)
		}
	}
	return nil
}

// branchExists checks whether `branch` exists locally.
func branchExists(repo *git.Repo, branch string) bool {
	wts, _ := repo.Worktrees()
	for _, w := range wts {
		if w.Branch == branch {
			return true
		}
	}
	// We deliberately don't shell to `git rev-parse` here — `git worktree add`
	// already handles both create and existing-branch cases; this hint just
	// picks the right flag for the common case.
	return false
}

func promptYN(msg string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N] ", msg)
	var resp string
	if _, err := fmt.Fscanln(os.Stdin, &resp); err != nil {
		return false
	}
	resp = strings.ToLower(strings.TrimSpace(resp))
	return resp == "y" || resp == "yes"
}
