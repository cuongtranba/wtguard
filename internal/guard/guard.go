// Package guard is the single source of truth for "should this commit be
// blocked?". Called by the proxy (`git commit`, `git push`) and by the
// pre-commit hook.
package guard

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cuongtranba/wtguard/internal/config"
)

// Rule is the input to Decide. All fields are precomputed by the caller so
// Decide stays pure and trivially testable.
type Rule struct {
	Branch        string
	Detached      bool
	Worktrees     int
	Policy        config.Policy
	ProtectedList []string
	BypassEnv     bool
}

// Decision is the output of Decide.
type Decision struct {
	Block  bool
	Reason string
}

// Decide returns Block=true with a human-readable reason when the commit
// should be refused.
//
// Rules:
//   - Bypass env set → allow (caller is responsible for auditing).
//   - Detached HEAD → allow.
//   - Branch ∉ protected → allow.
//   - Policy "always" → block.
//   - Policy "worktree-active" (default) → block iff worktree count > 1.
func Decide(r Rule) Decision {
	if r.BypassEnv {
		return Decision{Block: false, Reason: "bypass env set"}
	}
	if r.Detached {
		return Decision{Block: false, Reason: "detached HEAD"}
	}
	if !slices.Contains(r.ProtectedList, r.Branch) {
		return Decision{Block: false, Reason: fmt.Sprintf("branch %q not protected", r.Branch)}
	}
	policy := r.Policy
	if policy == "" {
		policy = config.PolicyWorktreeActive
	}
	switch policy {
	case config.PolicyAlways:
		return Decision{
			Block:  true,
			Reason: fmt.Sprintf("policy=always: branch %q is protected", r.Branch),
		}
	case config.PolicyWorktreeActive:
		if r.Worktrees > 1 {
			return Decision{
				Block: true,
				Reason: fmt.Sprintf("%d active worktree(s) — commit there instead",
					r.Worktrees-1),
			}
		}
		return Decision{Block: false, Reason: "no extra worktree active"}
	}
	return Decision{Block: false, Reason: fmt.Sprintf("unknown policy %q", policy)}
}

// FormatBlock renders the standard block message wtguard prints to stderr.
// `layer` is "proxy" or "hook"; `worktrees` is the porcelain-style list
// (path + branch). `nextHint` is an optional suggestion line.
func FormatBlock(layer, branch string, decision Decision, worktrees []WorktreeBrief, nextHint string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "wtguard: blocked `git commit` on protected branch %q\n", branch)
	fmt.Fprintf(&b, "  reason: %s\n", decision.Reason)
	if len(worktrees) > 0 {
		b.WriteString("  worktrees:\n")
		for _, w := range worktrees {
			fmt.Fprintf(&b, "    %s  [%s]\n", w.Path, w.Branch)
		}
	}
	if nextHint != "" {
		fmt.Fprintf(&b, "  next:\n    %s\n", nextHint)
	}
	b.WriteString("  override (logged): WTGUARD_BYPASS=1 git commit ...\n")
	if layer != "" {
		fmt.Fprintf(&b, "  layer: %s\n", layer)
	}
	return b.String()
}

// WorktreeBrief is the minimal info FormatBlock needs about a worktree.
type WorktreeBrief struct {
	Path   string
	Branch string
}
