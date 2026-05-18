package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuongtranba/wtguard/internal/config"
	"github.com/cuongtranba/wtguard/internal/git"
	"github.com/cuongtranba/wtguard/internal/hook"
	"github.com/cuongtranba/wtguard/internal/remote"
	"github.com/cuongtranba/wtguard/internal/shellrc"
	"github.com/cuongtranba/wtguard/internal/template"
	"github.com/urfave/cli/v2"
)

func cmdStatus() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "show what's installed where",
		Description: "Reports state of every defense layer for the current " +
			"repo: proxy symlink + PATH, real git path, shell rc marker " +
			"block, init template, repo hook (with chained pre-commit.local), " +
			"protected branches, and worktree count.",
		Action: actionStatus,
	}
}

func actionStatus(c *cli.Context) error {
	binDir, _ := wtguardBinDir()
	symlink := filepath.Join(binDir, "git")
	proxyOK := false
	target, err := os.Readlink(symlink)
	if err == nil {
		proxyOK = true
		fmt.Printf("proxy:         %s -> %s\n", symlink, target)
	} else {
		fmt.Printf("proxy:         not installed (expected %s)\n", symlink)
	}

	pathOK := strings.Contains(os.Getenv("PATH"), binDir)
	fmt.Printf("path:          %s in $PATH: %v\n", binDir, pathOK)

	// Find real git via PATH (proxy excluded)
	if proxyOK {
		realGit := lookupRealGit(binDir)
		fmt.Printf("real git:      %s\n", realGit)
	}

	// shell rc
	home, _ := os.UserHomeDir()
	for _, target := range shellrc.AllKnownTargets(home) {
		if has, _ := rcHasMarker(target); has {
			fmt.Printf("shellrc:       %s: patched\n", target)
		}
	}

	// global init template
	tplRoot, _ := template.DefaultRoot()
	if cur, _, err := git.GlobalConfigGet(template.InitTemplateKey); err == nil && cur != "" {
		fmt.Printf("init template: %s (active: %v)\n", cur, cur == tplRoot)
	} else {
		fmt.Printf("init template: not set\n")
	}

	// hook + repo state (if in a repo)
	if git.IsInsideWorkTree(c.String("repo")) {
		repo, err := openRepo(c)
		if err != nil {
			return err
		}
		hooksDir, err := repo.HooksDir()
		if err != nil {
			return err
		}
		installed, _ := hook.IsInstalled(hooksDir)
		chained, _ := hook.HasChained(hooksDir)
		fmt.Printf("hook (cwd):    installed=%v, chains pre-commit.local=%v\n", installed, chained)

		settings, err := config.Load(repo)
		if err != nil {
			return err
		}
		fmt.Printf("policy:        %s\n", settings.Policy)
		fmt.Printf("protected:     %s\n", strings.Join(settings.Protected, ", "))

		wts, err := repo.Worktrees()
		if err != nil {
			return err
		}
		fmt.Printf("worktrees:     %d active\n", len(wts))
		for _, w := range wts {
			marker := ""
			for _, p := range settings.Protected {
				if p == w.Branch {
					marker = " (protected)"
				}
			}
			fmt.Printf("  %s  [%s]%s\n", w.Path, w.Branch, marker)
		}

		// remote
		originURL, _ := repo.OriginURL()
		if owner, name, ok := remote.OwnerRepo(originURL); ok {
			fmt.Printf("remote:        %s/%s (run `gh api repos/%s/%s/branches/<br>/protection` to check)\n",
				owner, name, owner, name)
		}
	}

	return nil
}

func lookupRealGit(ownDir string) string {
	if p := os.Getenv("WTGUARD_REAL_GIT"); p != "" {
		return p
	}
	for p := range strings.SplitSeq(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if p == "" || p == ownDir {
			continue
		}
		candidate := filepath.Join(p, "git")
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Mode()&0o111 != 0 {
			return candidate
		}
	}
	return "(not found)"
}

func rcHasMarker(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), shellrc.BeginMarker), nil
}
