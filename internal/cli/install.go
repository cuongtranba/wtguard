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

func cmdInstall() *cli.Command {
	return &cli.Command{
		Name:  "install",
		Usage: "install proxy + hook + (opt) GitHub branch protection",
		Description: "Sets up the three layers of wtguard defense:\n" +
			"  1. ~/.wtguard/bin/git -> wtguard symlink (proxy).\n" +
			"  2. Hook shim in current repo and in ~/.wtguard/template (so " +
			"every future clone gets it).\n" +
			"  3. (opt-in --remote-protect) GitHub branch protection via gh.\n" +
			"\n" +
			"Idempotent: re-running picks up where the previous run left off.",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "remote-protect", Usage: "also apply GitHub branch protection via gh"},
			&cli.BoolFlag{Name: "force", Usage: "rewrite an existing wtguard-managed hook"},
		},
		Action: actionInstall,
	}
}

func actionInstall(c *cli.Context) error {
	// 1. proxy: ~/.wtguard/bin/git -> <wtguardBinary>
	binDir, err := wtguardBinDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("install: mkdir %s: %w", binDir, err)
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("install: locate self: %w", err)
	}
	if err := ensureSymlink(self, filepath.Join(binDir, "git")); err != nil {
		return err
	}
	fmt.Printf("proxy:    %s -> %s\n", filepath.Join(binDir, "git"), self)

	// 2. init template
	tplRoot, err := template.DefaultRoot()
	if err != nil {
		return err
	}
	tres, err := template.Install(tplRoot)
	if err != nil {
		return err
	}
	if tres.SetGlobal {
		fmt.Printf("template: %s (init.templateDir set globally)\n", tres.Root)
	} else if tres.LeftExisting != "" {
		fmt.Printf("template: %s\n  (note: init.templateDir left at %q — set it to %q manually for the auto-install belt)\n",
			tres.Root, tres.LeftExisting, tres.Root)
	}

	// 3. shell rc patch
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return fmt.Errorf("install: locate home directory: %w", err)
	}
	for _, target := range shellrc.Targets(os.Getenv("SHELL"), home) {
		res, err := shellrc.Patch(target, binDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wtguard: %v\n", err)
			continue
		}
		if res.Patched {
			fmt.Printf("shellrc:  patched %s — run `source %s`\n", res.Path, res.Path)
		}
	}

	// 4. hook in current repo (best effort — only if we're in one)
	if git.IsInsideWorkTree(c.String("repo")) {
		repo, err := openRepo(c)
		if err == nil {
			hooksDir, err := repo.HooksDir()
			if err != nil {
				return err
			}
			hres, err := hook.Install(hooksDir, c.Bool("force"))
			if err != nil {
				return err
			}
			switch {
			case hres.AlreadyOurs:
				fmt.Printf("hook:     %s (already installed)\n", hres.HookPath)
			case hres.ChainedTo != "":
				fmt.Printf("hook:     %s installed (chained existing hook to %s)\n",
					hres.HookPath, hres.ChainedTo)
			case hres.Wrote:
				fmt.Printf("hook:     %s installed\n", hres.HookPath)
			}
		}
	}

	// 5. opt-in remote
	if c.Bool("remote-protect") {
		if err := applyRemoteProtect(c); err != nil {
			fmt.Fprintf(os.Stderr, "remote:   %v (skipping — local install ok)\n", err)
		}
	}

	fmt.Println()
	fmt.Println("Open a new shell, or `source` your rc file, to pick up the PATH change.")
	return nil
}

// ensureSymlink (re-)creates dst as a symlink to src. Refuses if dst exists
// and is a non-symlink file (user-owned `git` we don't want to clobber).
func ensureSymlink(src, dst string) error {
	if existing, err := os.Lstat(dst); err == nil {
		if existing.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(dst); err != nil {
				return fmt.Errorf("install: replace symlink %s: %w", dst, err)
			}
		} else {
			return fmt.Errorf("install: %s exists and is not a symlink — refusing to overwrite", dst)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("install: stat %s: %w", dst, err)
	}
	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("install: symlink %s -> %s: %w", dst, src, err)
	}
	return nil
}

// wtguardBinDir is the conventional ~/.wtguard/bin. Overridable for tests
// (and parity with the shell installer) via WTGUARD_DIR.
func wtguardBinDir() (string, error) {
	if d := os.Getenv("WTGUARD_DIR"); d != "" {
		return filepath.Join(d, "bin"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".wtguard", "bin"), nil
}

func applyRemoteProtect(c *cli.Context) error {
	if err := remote.CheckPrereqs(); err != nil {
		return err
	}
	repo, err := openRepo(c)
	if err != nil {
		return err
	}
	originURL, err := repo.OriginURL()
	if err != nil || originURL == "" {
		return fmt.Errorf("no `origin` remote configured")
	}
	owner, name, ok := remote.OwnerRepo(originURL)
	if !ok {
		return fmt.Errorf("origin %q is not a recognized GitHub URL", originURL)
	}
	settings, err := config.Load(repo)
	if err != nil {
		return err
	}
	for _, br := range settings.Protected {
		if err := remote.ProtectBranch(owner, name, br); err != nil {
			// `gh api` returns 422 if the branch doesn't exist remotely;
			// keep going.
			fmt.Fprintf(os.Stderr, "remote: %s/%s: %v\n", owner, name, err)
			continue
		}
		fmt.Printf("remote:   %s/%s @ %s — branch protection applied\n", owner, name, br)
	}
	return nil
}

