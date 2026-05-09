package cli

import (
	"errors"
	"fmt"

	"github.com/urfave/cli/v2"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

var errNotImplemented = errors.New("not implemented yet")

const appDescription = `wtguard creates git worktrees and installs a pre-commit hook on the host
repo. The hook BLOCKS commits to protected branches (default main, master)
whenever any worktree is active. This protects repos from agents or humans
accidentally committing straight to main while feature work lives in a
worktree.

Block rule: branch is protected AND 'git worktree list' has > 1 entry.
Detached HEAD or non-protected branches are always allowed.

Run 'wtguard explain' for a full LLM-friendly reference (markdown).`

func NewApp(b BuildInfo) *cli.App {
	return &cli.App{
		Name:        "wtguard",
		Usage:       "create git worktrees and block direct commits to protected branches",
		Description: appDescription,
		Version:     fmt.Sprintf("%s (commit %s, built %s)", b.Version, b.Commit, b.Date),
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "repo", Usage: "path to git repo", Value: "."},
			&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "skip prompts"},
			&cli.BoolFlag{Name: "verbose", Usage: "verbose output (prints git commands)"},
		},
		Commands: []*cli.Command{
			cmdInstall(),
			cmdUninstall(),
			cmdStatus(),
			cmdCreate(),
			cmdRemove(),
			cmdList(),
			cmdConfig(),
			cmdExplain(),
			cmdHook(),
		},
	}
}

func cmdInstall() *cli.Command {
	return &cli.Command{
		Name:  "install",
		Usage: "install pre-commit hook into the repo",
		Description: "Writes a shim to .git/hooks/pre-commit that execs " +
			"'wtguard hook pre-commit'. If a pre-commit hook already exists, it " +
			"is renamed to pre-commit.local and chained after the guard check.",
		Action: func(c *cli.Context) error { return errNotImplemented },
	}
}

func cmdUninstall() *cli.Command {
	return &cli.Command{
		Name:  "uninstall",
		Usage: "remove the pre-commit hook (restore chained hook if any)",
		Description: "Removes the wtguard shim. If pre-commit.local exists it is " +
			"renamed back to pre-commit. Refuses to delete a hook that is not " +
			"managed by wtguard (no marker comment).",
		Action: func(c *cli.Context) error { return errNotImplemented },
	}
}

func cmdStatus() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "print hook + protected branches + worktree count",
		Description: "Shows: repo root, hook installed yes/no, presence of a " +
			"chained pre-commit.local, current protected list, and number of " +
			"active worktrees.",
		Action: func(c *cli.Context) error { return errNotImplemented },
	}
}

func cmdCreate() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "create a worktree (auto-installs hook if missing)",
		ArgsUsage: "<branch>",
		Description: "Runs 'git worktree add' for the given branch. If the hook " +
			"is not installed yet, prompts to install it (or use --yes / -y to " +
			"skip the prompt). Default path is " +
			"'<wtguard.worktreeDir><repo>-<branch>' which is configurable.",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "path", Usage: "explicit worktree path"},
		},
		Action: func(c *cli.Context) error { return errNotImplemented },
	}
}

func cmdRemove() *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Usage:     "remove a worktree by path or branch",
		ArgsUsage: "<path|branch>",
		Description: "Runs 'git worktree remove'. The argument can be either a " +
			"worktree path or a branch name (resolved via 'git worktree list'). " +
			"Use --force to remove worktrees with uncommitted changes.",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "force", Usage: "force removal"},
		},
		Action: func(c *cli.Context) error { return errNotImplemented },
	}
}

func cmdList() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "list worktrees and mark protected branches",
		Description: "Lists all worktrees with their branch and HEAD. Branches " +
			"that appear in wtguard.protected are flagged.",
		Action: func(c *cli.Context) error { return errNotImplemented },
	}
}

func cmdConfig() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "read / write wtguard config keys (stored in git config)",
		Description: "All wtguard settings live in .git/config under wtguard.*. " +
			"Keys: protected (comma-list, default main,master), worktreeDir " +
			"(path, default ../), bypassLog (bool, default true), chainHook " +
			"(bool, default true).",
		Subcommands: []*cli.Command{
			{
				Name:        "get",
				Usage:       "print value of a wtguard.<key>",
				ArgsUsage:   "<key>",
				Description: "Reads 'git config --local wtguard.<key>'. Falls back to default if unset.",
				Action:      func(c *cli.Context) error { return errNotImplemented },
			},
			{
				Name:        "set",
				Usage:       "set a wtguard.<key> to a value",
				ArgsUsage:   "<key> <value>",
				Description: "Writes 'git config --local wtguard.<key> <value>'.",
				Action:      func(c *cli.Context) error { return errNotImplemented },
			},
			{
				Name:        "protected",
				Usage:       "manage protected branches list",
				Description: "Convenience wrappers over wtguard.protected (comma-separated list).",
				Subcommands: []*cli.Command{
					{
						Name:      "add",
						Usage:     "append a branch to wtguard.protected",
						ArgsUsage: "<branch>",
						Action:    func(c *cli.Context) error { return errNotImplemented },
					},
					{
						Name:      "rm",
						Usage:     "remove a branch from wtguard.protected",
						ArgsUsage: "<branch>",
						Action:    func(c *cli.Context) error { return errNotImplemented },
					},
				},
			},
		},
	}
}

func cmdHook() *cli.Command {
	return &cli.Command{
		Name:   "hook",
		Usage:  "internal: invoked by git hook shim",
		Hidden: true,
		Description: "Not for direct human use. The shim at .git/hooks/pre-commit " +
			"execs 'wtguard hook pre-commit'. Reads current branch + worktree " +
			"list, exits 1 to block, or chains to pre-commit.local if present.",
		Subcommands: []*cli.Command{
			{
				Name:   "pre-commit",
				Usage:  "internal: pre-commit guard entry point",
				Action: func(c *cli.Context) error { return errNotImplemented },
			},
		},
	}
}
