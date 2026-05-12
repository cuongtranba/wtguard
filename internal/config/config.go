// Package config is a typed view over the wtguard.* keys in `git config`.
//
// All settings live in .git/config (or --global where noted). This package
// reads them with sensible defaults and writes them back through git.
package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cuongtranba/wtguard/internal/git"
)

type Policy string

const (
	PolicyWorktreeActive Policy = "worktree-active"
	PolicyAlways         Policy = "always"
)

// Defaults are returned when a key is unset.
type Settings struct {
	Protected    []string
	Policy       Policy
	WorktreeDir  string
	BypassLog    bool
	ChainHook    bool
}

func Defaults() Settings {
	return Settings{
		Protected:   []string{"main", "master"},
		Policy:      PolicyWorktreeActive,
		WorktreeDir: "../",
		BypassLog:   true,
		ChainHook:   true,
	}
}

// Keys understood by wtguard. Listed centrally so `wtguard config get` can
// validate input.
const (
	KeyProtected   = "wtguard.protected"
	KeyPolicy      = "wtguard.policy"
	KeyWorktreeDir = "wtguard.worktreeDir"
	KeyBypassLog   = "wtguard.bypassLog"
	KeyChainHook   = "wtguard.chainHook"
)

func KnownKeys() []string {
	return []string{KeyProtected, KeyPolicy, KeyWorktreeDir, KeyBypassLog, KeyChainHook}
}

// IsKnown reports whether key is a recognized wtguard config key.
func IsKnown(key string) bool {
	return slices.Contains(KnownKeys(), key)
}

// Load reads all wtguard.* keys from the repo's local git config, applying
// defaults where unset.
func Load(r *git.Repo) (Settings, error) {
	s := Defaults()
	if v, ok, err := r.ConfigGet(KeyProtected); err != nil {
		return s, err
	} else if ok {
		s.Protected = parseCSV(v)
	}
	if v, ok, err := r.ConfigGet(KeyPolicy); err != nil {
		return s, err
	} else if ok {
		switch Policy(v) {
		case PolicyWorktreeActive, PolicyAlways:
			s.Policy = Policy(v)
		default:
			return s, fmt.Errorf("config: invalid %s=%q (want %q or %q)",
				KeyPolicy, v, PolicyWorktreeActive, PolicyAlways)
		}
	}
	if v, ok, err := r.ConfigGet(KeyWorktreeDir); err != nil {
		return s, err
	} else if ok {
		s.WorktreeDir = v
	}
	if v, ok, err := r.ConfigGet(KeyBypassLog); err != nil {
		return s, err
	} else if ok {
		b, perr := parseBool(v)
		if perr != nil {
			return s, fmt.Errorf("config: %s: %w", KeyBypassLog, perr)
		}
		s.BypassLog = b
	}
	if v, ok, err := r.ConfigGet(KeyChainHook); err != nil {
		return s, err
	} else if ok {
		b, perr := parseBool(v)
		if perr != nil {
			return s, fmt.Errorf("config: %s: %w", KeyChainHook, perr)
		}
		s.ChainHook = b
	}
	return s, nil
}

// ProtectedAdd appends a branch to wtguard.protected if absent.
func ProtectedAdd(r *git.Repo, branch string) error {
	cur, _, err := r.ConfigGet(KeyProtected)
	if err != nil {
		return err
	}
	list := parseCSV(cur)
	if cur == "" {
		list = Defaults().Protected
	}
	if slices.Contains(list, branch) {
		return nil
	}
	list = append(list, branch)
	return r.ConfigSet(KeyProtected, strings.Join(list, ","))
}

// ProtectedRm removes a branch from wtguard.protected if present.
func ProtectedRm(r *git.Repo, branch string) error {
	cur, ok, err := r.ConfigGet(KeyProtected)
	if err != nil {
		return err
	}
	var list []string
	if ok {
		list = parseCSV(cur)
	} else {
		list = Defaults().Protected
	}
	out := list[:0]
	for _, b := range list {
		if b != branch {
			out = append(out, b)
		}
	}
	if len(out) == 0 {
		return r.ConfigSet(KeyProtected, "")
	}
	return r.ConfigSet(KeyProtected, strings.Join(out, ","))
}

func parseCSV(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseBool(v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off", "":
		return false, nil
	}
	return false, fmt.Errorf("invalid bool %q", v)
}
