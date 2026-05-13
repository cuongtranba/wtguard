package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cuongtranba/wtguard/internal/config"
	"github.com/cuongtranba/wtguard/internal/git"
	"github.com/cuongtranba/wtguard/internal/hook"
	"github.com/cuongtranba/wtguard/internal/remote"
	"github.com/cuongtranba/wtguard/internal/shellrc"
	"github.com/cuongtranba/wtguard/internal/template"
	"github.com/urfave/cli/v2"
)

func cmdUninstall() *cli.Command {
	return &cli.Command{
		Name:  "uninstall",
		Usage: "remove proxy + hook (branch protection stays — affects shared state)",
		Description: "Reverses what `install` did: per-repo hook (restoring " +
			"pre-commit.local if present), global init.templateDir, shell rc " +
			"marker block, and the ~/.wtguard/bin symlink. GitHub branch " +
			"protection from --remote-protect is NOT auto-removed; the " +
			"command to run is printed.",
		Action: actionUninstall,
	}
}

func actionUninstall(c *cli.Context) error {
	// 1. repo hook
	if git.IsInsideWorkTree(c.String("repo")) {
		repo, err := openRepo(c)
		if err == nil {
			hooksDir, err := repo.HooksDir()
			if err == nil {
				res, err := hook.Uninstall(hooksDir)
				switch {
				case err != nil:
					fmt.Fprintf(os.Stderr, "hook:     %v\n", err)
				case res.Removed && res.Restored != "":
					fmt.Printf("hook:     removed (restored pre-commit.local → %s)\n", res.Restored)
				case res.Removed:
					fmt.Printf("hook:     removed %s\n", res.HookPath)
				case res.NotPresent:
					fmt.Printf("hook:     not installed\n")
				}
			}
		}
	}

	// 2. template + global init.templateDir
	tplRoot, err := template.DefaultRoot()
	if err == nil {
		res, err := template.Uninstall(tplRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "template: %v\n", err)
		}
		switch {
		case res.UnsetGlobal:
			fmt.Printf("template: cleared global init.templateDir\n")
		case res.PointedElsewhere != "":
			fmt.Printf("template: init.templateDir is %q (not ours; leaving alone)\n", res.PointedElsewhere)
		}
	}

	// 3. shell rc unpatch
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return fmt.Errorf("uninstall: locate home directory: %w", err)
	}
	for _, target := range shellrc.AllKnownTargets(home) {
		res, err := shellrc.Unpatch(target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "shellrc:  %v\n", err)
			continue
		}
		if res.Cleaned {
			fmt.Printf("shellrc:  cleaned %s\n", res.Path)
		}
	}

	// 4. proxy symlink + bin dir
	binDir, _ := wtguardBinDir()
	symlink := filepath.Join(binDir, "git")
	if info, err := os.Lstat(symlink); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(symlink)
			fmt.Printf("proxy:    removed %s\n", symlink)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "proxy:    %v\n", err)
	}

	// 5. note about remote
	if git.IsInsideWorkTree(c.String("repo")) {
		repo, err := openRepo(c)
		if err == nil {
			noteRemoteCommands(repo)
		}
	}
	return nil
}

func noteRemoteCommands(repo *git.Repo) {
	originURL, err := repo.OriginURL()
	if err != nil || originURL == "" {
		return
	}
	owner, name, ok := remote.OwnerRepo(originURL)
	if !ok {
		return
	}
	settings, err := config.Load(repo)
	if err != nil {
		return
	}
	fmt.Println()
	fmt.Println("note: GitHub branch protection (if applied via --remote-protect) is NOT removed.")
	fmt.Println("      To remove it, run:")
	for _, br := range settings.Protected {
		fmt.Printf("        %s\n", remote.UnprotectCommand(owner, name, br))
	}
}
