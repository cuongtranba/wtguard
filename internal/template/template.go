// Package template writes a hook shim into ~/.wtguard/template/hooks so
// every future `git init`/`git clone` ships with the wtguard hook (when
// init.templateDir is pointed at us).
package template

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cuongtranba/wtguard/internal/git"
	"github.com/cuongtranba/wtguard/internal/hook"
)

// InitTemplateKey is the git-config key we manage.
const InitTemplateKey = "init.templateDir"

// Install writes the hook shim into <root>/hooks/ and sets
// init.templateDir=<root> globally. Idempotent.
//
// `root` is typically ~/.wtguard/template.
type InstallResult struct {
	Root           string
	HookPath       string
	WroteHook      bool
	SetGlobal      bool   // we set init.templateDir
	LeftExisting   string // existing init.templateDir we refused to overwrite
}

func Install(root string) (InstallResult, error) {
	res := InstallResult{Root: root}
	hooks := filepath.Join(root, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		return res, fmt.Errorf("template: mkdir %s: %w", hooks, err)
	}
	hookPath := filepath.Join(hooks, "pre-commit")
	res.HookPath = hookPath
	// Always rewrite — the shim is tiny and pinning means newer wtguard
	// versions can update their own marker / contents.
	if err := os.WriteFile(hookPath, []byte(hook.Shim), 0o755); err != nil {
		return res, fmt.Errorf("template: write %s: %w", hookPath, err)
	}
	_ = os.Chmod(hookPath, 0o755)
	res.WroteHook = true

	cur, _, err := git.GlobalConfigGet(InitTemplateKey)
	if err != nil {
		return res, err
	}
	if cur == "" || cur == root {
		if err := git.GlobalConfigSet(InitTemplateKey, root); err != nil {
			return res, err
		}
		res.SetGlobal = true
	} else {
		res.LeftExisting = cur
	}
	return res, nil
}

// Uninstall removes the global init.templateDir setting if it points at root.
// The template directory itself is left in place (the parent uninstaller will
// delete ~/.wtguard wholesale).
type UninstallResult struct {
	UnsetGlobal     bool
	PointedElsewhere string
}

func Uninstall(root string) (UninstallResult, error) {
	res := UninstallResult{}
	cur, _, err := git.GlobalConfigGet(InitTemplateKey)
	if err != nil {
		return res, err
	}
	if cur == "" {
		return res, nil
	}
	if cur != root {
		res.PointedElsewhere = cur
		return res, nil
	}
	if err := git.GlobalConfigUnset(InitTemplateKey); err != nil {
		return res, err
	}
	res.UnsetGlobal = true
	return res, nil
}

// DefaultRoot returns ~/.wtguard/template (the conventional location).
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".wtguard", "template"), nil
}
