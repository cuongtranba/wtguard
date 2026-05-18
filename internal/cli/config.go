package cli

import (
	"fmt"

	"github.com/cuongtranba/wtguard/internal/config"
	"github.com/urfave/cli/v2"
)

func cmdConfig() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "read / write wtguard.* keys in .git/config",
		Description: "All wtguard settings live in .git/config under wtguard.*. " +
			"Known keys: protected, policy, worktreeDir, bypassLog, chainHook.",
		Subcommands: []*cli.Command{
			{
				Name:      "get",
				Usage:     "print value of a wtguard.<key>",
				ArgsUsage: "<key>",
				Action:    actionConfigGet,
			},
			{
				Name:      "set",
				Usage:     "set a wtguard.<key> to a value",
				ArgsUsage: "<key> <value>",
				Action:    actionConfigSet,
			},
			{
				Name:        "protected",
				Usage:       "manage protected branches",
				Description: "Convenience wrappers over wtguard.protected (comma-list).",
				Subcommands: []*cli.Command{
					{
						Name:      "add",
						Usage:     "append a branch to wtguard.protected",
						ArgsUsage: "<branch>",
						Action:    actionProtectedAdd,
					},
					{
						Name:      "rm",
						Usage:     "remove a branch from wtguard.protected",
						ArgsUsage: "<branch>",
						Action:    actionProtectedRm,
					},
				},
			},
		},
	}
}

func actionConfigGet(c *cli.Context) error {
	key := normalizeKey(c.Args().First())
	if !config.IsKnown(key) {
		return fmt.Errorf("config get: unknown key %q (known: %v)", key, config.KnownKeys())
	}
	repo, err := openRepo(c)
	if err != nil {
		return err
	}
	v, ok, err := repo.ConfigGet(key)
	if err != nil {
		return err
	}
	if !ok {
		// fall back to default
		d := config.Defaults()
		switch key {
		case config.KeyProtected:
			fmt.Println("main,master")
		case config.KeyPolicy:
			fmt.Println(d.Policy)
		case config.KeyWorktreeDir:
			fmt.Println(d.WorktreeDir)
		case config.KeyBypassLog:
			fmt.Println(d.BypassLog)
		case config.KeyChainHook:
			fmt.Println(d.ChainHook)
		}
		return nil
	}
	fmt.Println(v)
	return nil
}

func actionConfigSet(c *cli.Context) error {
	if c.Args().Len() < 2 {
		return fmt.Errorf("config set: need <key> <value>")
	}
	key := normalizeKey(c.Args().Get(0))
	if !config.IsKnown(key) {
		return fmt.Errorf("config set: unknown key %q", key)
	}
	val := c.Args().Get(1)
	repo, err := openRepo(c)
	if err != nil {
		return err
	}
	return repo.ConfigSet(key, val)
}

func actionProtectedAdd(c *cli.Context) error {
	branch := c.Args().First()
	if branch == "" {
		return fmt.Errorf("config protected add: branch required")
	}
	repo, err := openRepo(c)
	if err != nil {
		return err
	}
	return config.ProtectedAdd(repo, branch)
}

func actionProtectedRm(c *cli.Context) error {
	branch := c.Args().First()
	if branch == "" {
		return fmt.Errorf("config protected rm: branch required")
	}
	repo, err := openRepo(c)
	if err != nil {
		return err
	}
	return config.ProtectedRm(repo, branch)
}

func normalizeKey(k string) string {
	// allow `wtguard.protected` or just `protected`
	if len(k) > len("wtguard.") && k[:len("wtguard.")] == "wtguard." {
		return k
	}
	return "wtguard." + k
}
