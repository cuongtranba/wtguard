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

func NewApp(b BuildInfo) *cli.App {
	return &cli.App{
		Name:    "wtguard",
		Usage:   "create git worktrees and block direct commits to protected branches",
		Version: fmt.Sprintf("%s (commit %s, built %s)", b.Version, b.Commit, b.Date),
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "repo", Usage: "path to git repo", Value: "."},
			&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "skip prompts"},
			&cli.BoolFlag{Name: "verbose", Usage: "verbose output"},
		},
		Commands: []*cli.Command{
			cmdInstall(),
			cmdUninstall(),
			cmdStatus(),
			cmdCreate(),
			cmdRemove(),
			cmdList(),
			cmdConfig(),
			cmdHook(),
		},
	}
}

func cmdInstall() *cli.Command {
	return &cli.Command{
		Name:   "install",
		Usage:  "install pre-commit hook into the repo",
		Action: func(c *cli.Context) error { return errNotImplemented },
	}
}

func cmdUninstall() *cli.Command {
	return &cli.Command{
		Name:   "uninstall",
		Usage:  "remove the pre-commit hook (restore chained hook if any)",
		Action: func(c *cli.Context) error { return errNotImplemented },
	}
}

func cmdStatus() *cli.Command {
	return &cli.Command{
		Name:   "status",
		Usage:  "print hook + protected branches + worktree count",
		Action: func(c *cli.Context) error { return errNotImplemented },
	}
}

func cmdCreate() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "create a worktree (auto-installs hook if missing)",
		ArgsUsage: "<branch>",
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
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "force", Usage: "force removal"},
		},
		Action: func(c *cli.Context) error { return errNotImplemented },
	}
}

func cmdList() *cli.Command {
	return &cli.Command{
		Name:   "list",
		Usage:  "list worktrees and mark protected branches",
		Action: func(c *cli.Context) error { return errNotImplemented },
	}
}

func cmdConfig() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "read / write wtguard config keys (stored in git config)",
		Subcommands: []*cli.Command{
			{
				Name:      "get",
				ArgsUsage: "<key>",
				Action:    func(c *cli.Context) error { return errNotImplemented },
			},
			{
				Name:      "set",
				ArgsUsage: "<key> <value>",
				Action:    func(c *cli.Context) error { return errNotImplemented },
			},
			{
				Name:  "protected",
				Usage: "manage protected branches list",
				Subcommands: []*cli.Command{
					{
						Name:      "add",
						ArgsUsage: "<branch>",
						Action:    func(c *cli.Context) error { return errNotImplemented },
					},
					{
						Name:      "rm",
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
		Subcommands: []*cli.Command{
			{
				Name:   "pre-commit",
				Action: func(c *cli.Context) error { return errNotImplemented },
			},
		},
	}
}
