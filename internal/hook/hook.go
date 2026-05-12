// Package hook installs / removes the wtguard pre-commit shim, chaining
// any pre-existing user hook to pre-commit.local.
package hook

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Marker line that identifies a wtguard-managed shim.
const Marker = "# wtguard managed - do not edit"

// Shim is the script wtguard drops into .git/hooks/pre-commit (and into the
// init template). It execs the wtguard binary so the binary's path can move
// independently of the shim.
const Shim = `#!/bin/sh
` + Marker + `
exec "${WTGUARD_BIN:-wtguard}" hook pre-commit "$@"
`

// Result describes what Install actually did, so callers can print useful
// status messages without re-reading the filesystem.
type Result struct {
	HookPath      string
	Wrote         bool   // we wrote a new shim
	AlreadyOurs   bool   // marker already present, nothing to do
	ChainedTo     string // existing hook moved here (empty if none)
	ChainConflict bool   // pre-commit.local already existed — refused
}

// Install drops the shim into hooksDir/pre-commit. If a pre-commit hook
// already exists and is not ours, it is moved to pre-commit.local (chained).
// If pre-commit.local already exists, returns ChainConflict=true and does
// nothing.
//
// `force` rewrites our own shim (useful after a binary upgrade) but is not
// needed for the common case.
func Install(hooksDir string, force bool) (Result, error) {
	res := Result{}
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return res, fmt.Errorf("hook: mkdir %s: %w", hooksDir, err)
	}
	path := filepath.Join(hooksDir, "pre-commit")
	res.HookPath = path
	existing, mode, err := readHook(path)
	if err != nil {
		return res, err
	}
	if existing == nil {
		// no existing hook
		if err := writeShim(path); err != nil {
			return res, err
		}
		res.Wrote = true
		return res, nil
	}
	if IsOurs(existing) {
		if !force {
			res.AlreadyOurs = true
			return res, nil
		}
		if err := writeShim(path); err != nil {
			return res, err
		}
		res.Wrote = true
		return res, nil
	}
	// existing user hook — chain
	chained := filepath.Join(hooksDir, "pre-commit.local")
	if _, err := os.Stat(chained); err == nil {
		res.ChainConflict = true
		return res, fmt.Errorf("hook: %s already exists; refusing to overwrite", chained)
	} else if !errors.Is(err, os.ErrNotExist) {
		return res, fmt.Errorf("hook: stat %s: %w", chained, err)
	}
	if err := os.Rename(path, chained); err != nil {
		return res, fmt.Errorf("hook: rename to %s: %w", chained, err)
	}
	if mode != 0 {
		_ = os.Chmod(chained, mode)
	}
	res.ChainedTo = chained
	if err := writeShim(path); err != nil {
		return res, err
	}
	res.Wrote = true
	return res, nil
}

// Uninstall is the inverse of Install. Refuses to delete a non-wtguard hook.
// If pre-commit.local exists it is restored to pre-commit.
type UninstallResult struct {
	HookPath  string
	Removed   bool
	Restored  string // path moved back to pre-commit (empty if no chained hook)
	NotOurs   bool   // hook exists but isn't ours — refused
	NotPresent bool  // no hook to remove
}

func Uninstall(hooksDir string) (UninstallResult, error) {
	res := UninstallResult{}
	path := filepath.Join(hooksDir, "pre-commit")
	res.HookPath = path
	existing, _, err := readHook(path)
	if err != nil {
		return res, err
	}
	if existing == nil {
		res.NotPresent = true
		// Even with no shim, we may still want to restore a chained hook.
	} else if !IsOurs(existing) {
		res.NotOurs = true
		return res, fmt.Errorf("hook: %s is not managed by wtguard (missing marker)", path)
	} else {
		if err := os.Remove(path); err != nil {
			return res, fmt.Errorf("hook: remove %s: %w", path, err)
		}
		res.Removed = true
	}
	chained := filepath.Join(hooksDir, "pre-commit.local")
	if _, err := os.Stat(chained); err == nil {
		if err := os.Rename(chained, path); err != nil {
			return res, fmt.Errorf("hook: restore %s: %w", path, err)
		}
		res.Restored = path
	} else if !errors.Is(err, os.ErrNotExist) {
		return res, fmt.Errorf("hook: stat %s: %w", chained, err)
	}
	return res, nil
}

// IsOurs reports whether content looks like our shim (by marker).
func IsOurs(content []byte) bool {
	return bytes.Contains(content, []byte(Marker))
}

// IsInstalled reports whether hooksDir/pre-commit is our shim.
func IsInstalled(hooksDir string) (bool, error) {
	content, _, err := readHook(filepath.Join(hooksDir, "pre-commit"))
	if err != nil {
		return false, err
	}
	if content == nil {
		return false, nil
	}
	return IsOurs(content), nil
}

// HasChained reports whether a pre-commit.local exists in hooksDir.
func HasChained(hooksDir string) (bool, error) {
	_, err := os.Stat(filepath.Join(hooksDir, "pre-commit.local"))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func readHook(path string) ([]byte, os.FileMode, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("hook: stat %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("hook: read %s: %w", path, err)
	}
	return data, info.Mode().Perm(), nil
}

func writeShim(path string) error {
	if err := os.WriteFile(path, []byte(Shim), 0o755); err != nil {
		return fmt.Errorf("hook: write %s: %w", path, err)
	}
	// ensure executable bit even on systems with restrictive umask
	if err := os.Chmod(path, 0o755); err != nil {
		return fmt.Errorf("hook: chmod %s: %w", path, err)
	}
	return nil
}
