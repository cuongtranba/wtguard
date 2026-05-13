package guard

import (
	"strings"
	"testing"

	"github.com/cuongtranba/wtguard/internal/config"
)

func TestDecide(t *testing.T) {
	protected := []string{"main", "master"}
	cases := []struct {
		name      string
		rule      Rule
		wantBlock bool
	}{
		{
			name: "bypass overrides everything",
			rule: Rule{
				Branch: "main", Worktrees: 5, Policy: config.PolicyAlways,
				ProtectedList: protected, BypassEnv: true,
			},
			wantBlock: false,
		},
		{
			name:      "detached HEAD allows",
			rule:      Rule{Detached: true, Worktrees: 5, ProtectedList: protected, Policy: config.PolicyAlways},
			wantBlock: false,
		},
		{
			name:      "non-protected branch allows",
			rule:      Rule{Branch: "feat-x", Worktrees: 2, ProtectedList: protected, Policy: config.PolicyWorktreeActive},
			wantBlock: false,
		},
		{
			name:      "policy=always blocks even with single worktree",
			rule:      Rule{Branch: "main", Worktrees: 1, ProtectedList: protected, Policy: config.PolicyAlways},
			wantBlock: true,
		},
		{
			name:      "worktree-active with 1 worktree allows",
			rule:      Rule{Branch: "main", Worktrees: 1, ProtectedList: protected, Policy: config.PolicyWorktreeActive},
			wantBlock: false,
		},
		{
			name:      "worktree-active with 2 worktrees blocks",
			rule:      Rule{Branch: "main", Worktrees: 2, ProtectedList: protected, Policy: config.PolicyWorktreeActive},
			wantBlock: true,
		},
		{
			name:      "worktree-active with many worktrees blocks",
			rule:      Rule{Branch: "master", Worktrees: 7, ProtectedList: protected, Policy: config.PolicyWorktreeActive},
			wantBlock: true,
		},
		{
			name:      "default policy blocks commit on master with single worktree",
			rule:      Rule{Branch: "master", Worktrees: 1, ProtectedList: protected},
			wantBlock: true,
		},
		{
			name:      "default policy blocks commit on main with single worktree",
			rule:      Rule{Branch: "main", Worktrees: 1, ProtectedList: protected},
			wantBlock: true,
		},
		{
			name:      "default policy blocks regardless of worktree count",
			rule:      Rule{Branch: "master", Worktrees: 4, ProtectedList: protected},
			wantBlock: true,
		},
		{
			name:      "empty protected list allows everything",
			rule:      Rule{Branch: "main", Worktrees: 5, ProtectedList: nil, Policy: config.PolicyAlways},
			wantBlock: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.rule)
			if d.Block != tc.wantBlock {
				t.Fatalf("Decide(%+v).Block = %v, want %v (reason=%s)",
					tc.rule, d.Block, tc.wantBlock, d.Reason)
			}
			if d.Reason == "" {
				t.Errorf("Decide returned empty reason")
			}
		})
	}
}

func TestFormatBlock(t *testing.T) {
	out := FormatBlock("proxy", "main",
		Decision{Block: true, Reason: "1 active worktree(s) — commit there instead"},
		[]WorktreeBrief{{Path: "../repo-feat-x", Branch: "feat-x"}},
		"cd ../repo-feat-x && git commit ...",
	)
	want := []string{
		"wtguard: blocked",
		"\"main\"",
		"reason:",
		"worktrees:",
		"../repo-feat-x",
		"feat-x",
		"next:",
		"WTGUARD_BYPASS=1",
		"layer: proxy",
	}
	for _, frag := range want {
		if !strings.Contains(out, frag) {
			t.Errorf("FormatBlock output missing %q\n%s", frag, out)
		}
	}
}
