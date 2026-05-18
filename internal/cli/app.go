package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

const appDescription = `wtguard installs three independent layers of defense against agents and
humans accidentally committing/pushing directly to a protected branch:

  1. PATH-injected 'git' proxy (~/.wtguard/bin/git) — catches --no-verify
     and missing hooks.
  2. .git/hooks/pre-commit shim — catches the case where the proxy is
     bypassed (e.g. /usr/bin/git called directly).
  3. (opt-in) GitHub branch protection via 'wtguard install --remote-protect'.

Block rule: branch is in wtguard.protected (policy=always, the default —
blocks every commit/push on a protected branch). Set
wtguard.policy=worktree-active to relax the rule and only block when a
feature worktree already exists.

Run 'wtguard explain' for a full LLM-friendly reference (markdown).`

func NewApp(b BuildInfo) *cli.App {
	return &cli.App{
		Name:        "wtguard",
		Usage:       "block direct commits/pushes to protected branches via a 3-layer defense",
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
