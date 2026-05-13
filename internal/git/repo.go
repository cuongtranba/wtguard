// Package git is a typed wrapper over `git` invoked via os/exec.
//
// All wtguard interactions with git go through this package so the rest of
// the code never has to think about argv quoting, working-directory, or
// porcelain parsing.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Repo struct {
	root    string
	gitDir  string
	verbose bool
}

type Worktree struct {
	Path     string
	Branch   string
	Head     string
	Bare     bool
	Detached bool
}

// Open resolves path to a git repository root and returns a Repo handle.
// Path can be anywhere inside the working tree.
func Open(path string) (*Repo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("git: resolve path: %w", err)
	}
	root, err := runOut(abs, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("git: not a git repository: %s", abs)
	}
	gitDir, err := runOut(abs, "rev-parse", "--git-dir")
	if err != nil {
		return nil, fmt.Errorf("git: resolve git-dir: %w", err)
	}
	rootTrim := strings.TrimSpace(root)
	gd := strings.TrimSpace(gitDir)
	if !filepath.IsAbs(gd) {
		gd = filepath.Join(rootTrim, gd)
	}
	return &Repo{root: rootTrim, gitDir: gd}, nil
}

// OpenCwd opens the current working directory as a git repo.
func OpenCwd() (*Repo, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return Open(cwd)
}

// SetVerbose toggles command-tracing to stderr (used by the --verbose flag).
func (r *Repo) SetVerbose(v bool) { r.verbose = v }

// Root returns the absolute path to the working tree root.
func (r *Repo) Root() string { return r.root }

// GitDir returns the absolute path to the .git directory (or git-common-dir
// in a worktree).
func (r *Repo) GitDir() string { return r.gitDir }

// HooksDir returns the absolute path to the hooks directory for this repo,
// honoring core.hooksPath and git-common-dir for worktrees.
func (r *Repo) HooksDir() (string, error) {
	out, err := r.runOut("rev-parse", "--git-path", "hooks")
	if err != nil {
		return "", fmt.Errorf("git: resolve hooks path: %w", err)
	}
	p := strings.TrimSpace(out)
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.root, p)
	}
	return p, nil
}

// CurrentBranch returns the current branch name. The second return value is
// false when HEAD is detached.
func (r *Repo) CurrentBranch() (string, bool, error) {
	out, err := r.runOut("symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		// symbolic-ref returns exit 1 on detached HEAD with no output.
		return "", false, nil
	}
	br := strings.TrimSpace(out)
	if br == "" {
		return "", false, nil
	}
	return br, true, nil
}

// Worktrees returns one entry per worktree as reported by
// `git worktree list --porcelain`. The main worktree is always included.
func (r *Repo) Worktrees() ([]Worktree, error) {
	out, err := r.runOut("worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git: worktree list: %w", err)
	}
	return parseWorktrees(out), nil
}

// HasLocalBranch reports whether refs/heads/<branch> exists.
func (r *Repo) HasLocalBranch(branch string) bool {
	_, err := r.runOut("show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// AddWorktree runs `git worktree add`. If create is true the branch is
// created from HEAD.
func (r *Repo) AddWorktree(path, branch string, create bool) error {
	args := []string{"worktree", "add"}
	if create {
		args = append(args, "-b", branch, path)
	} else {
		args = append(args, path, branch)
	}
	if err := r.run(args...); err != nil {
		return fmt.Errorf("git: worktree add: %w", err)
	}
	return nil
}

// RemoveWorktree runs `git worktree remove`.
func (r *Repo) RemoveWorktree(path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	if err := r.run(args...); err != nil {
		return fmt.Errorf("git: worktree remove: %w", err)
	}
	return nil
}

// ConfigGet reads --local config. Returns ("", false, nil) when unset.
func (r *Repo) ConfigGet(key string) (string, bool, error) {
	cmd := exec.Command("git", "config", "--local", "--get", key)
	cmd.Dir = r.root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			// `git config --get` exits 1 when key is not set.
			return "", false, nil
		}
		return "", false, fmt.Errorf("git: config get %s: %w: %s", key, err, stderr.String())
	}
	return strings.TrimRight(stdout.String(), "\n"), true, nil
}

// ConfigSet writes a --local config value.
func (r *Repo) ConfigSet(key, val string) error {
	if err := r.run("config", "--local", key, val); err != nil {
		return fmt.Errorf("git: config set %s: %w", key, err)
	}
	return nil
}

// ConfigUnset removes a --local config key (idempotent: missing key is ok).
func (r *Repo) ConfigUnset(key string) error {
	cmd := exec.Command("git", "config", "--local", "--unset", key)
	cmd.Dir = r.root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		// exit 5 = "you try to unset an option which does not exist"
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 5 {
			return nil
		}
		return fmt.Errorf("git: config unset %s: %w: %s", key, err, stderr.String())
	}
	return nil
}

// IsInsideWorkTree returns true if path is inside a git working tree.
func IsInsideWorkTree(path string) bool {
	out, err := runOut(path, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "true"
}

// GlobalConfigGet reads --global config.
func GlobalConfigGet(key string) (string, bool, error) {
	cmd := exec.Command("git", "config", "--global", "--get", key)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", false, nil
		}
		return "", false, fmt.Errorf("git: global config get %s: %w: %s", key, err, stderr.String())
	}
	return strings.TrimRight(stdout.String(), "\n"), true, nil
}

// GlobalConfigSet writes a --global config value.
func GlobalConfigSet(key, val string) error {
	cmd := exec.Command("git", "config", "--global", key, val)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git: global config set %s: %w: %s", key, err, stderr.String())
	}
	return nil
}

// GlobalConfigUnset removes a --global config key (idempotent).
func GlobalConfigUnset(key string) error {
	cmd := exec.Command("git", "config", "--global", "--unset", key)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 5 {
			return nil
		}
		return fmt.Errorf("git: global config unset %s: %w: %s", key, err, stderr.String())
	}
	return nil
}

// OriginURL returns the URL of the `origin` remote, or "" if unset.
func (r *Repo) OriginURL() (string, error) {
	out, err := r.runOut("remote", "get-url", "origin")
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

func (r *Repo) run(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if r.verbose {
		fmt.Fprintf(os.Stderr, "+ git %s\n", strings.Join(args, " "))
	}
	return cmd.Run()
}

func (r *Repo) runOut(args ...string) (string, error) {
	return runOut(r.root, args...)
}

func runOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
