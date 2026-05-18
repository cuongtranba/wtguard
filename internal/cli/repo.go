package cli

import (
	"github.com/cuongtranba/wtguard/internal/git"
	"github.com/urfave/cli/v2"
)

// openRepo opens the git repo pointed at by the global --repo flag (or cwd).
func openRepo(c *cli.Context) (*git.Repo, error) {
	repo, err := git.Open(c.String("repo"))
	if err != nil {
		return nil, err
	}
	repo.SetVerbose(c.Bool("verbose"))
	return repo, nil
}
