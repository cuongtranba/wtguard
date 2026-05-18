package git

import (
	"reflect"
	"testing"
)

func TestParseWorktrees(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []Worktree
	}{
		{
			name: "single main worktree",
			in: "worktree /a/main\n" +
				"HEAD deadbeef\n" +
				"branch refs/heads/main\n\n",
			want: []Worktree{{Path: "/a/main", Head: "deadbeef", Branch: "main"}},
		},
		{
			name: "main plus feature worktree",
			in: "worktree /a/main\n" +
				"HEAD deadbeef\n" +
				"branch refs/heads/main\n\n" +
				"worktree /a/feat\n" +
				"HEAD cafef00d\n" +
				"branch refs/heads/feat-x\n\n",
			want: []Worktree{
				{Path: "/a/main", Head: "deadbeef", Branch: "main"},
				{Path: "/a/feat", Head: "cafef00d", Branch: "feat-x"},
			},
		},
		{
			name: "detached worktree",
			in: "worktree /a/det\n" +
				"HEAD cafef00d\n" +
				"detached\n\n",
			want: []Worktree{{Path: "/a/det", Head: "cafef00d", Detached: true}},
		},
		{
			name: "bare worktree",
			in:   "worktree /a/bare\nbare\n\n",
			want: []Worktree{{Path: "/a/bare", Bare: true}},
		},
		{
			name: "trailing newline missing",
			in: "worktree /a/main\n" +
				"HEAD deadbeef\n" +
				"branch refs/heads/main",
			want: []Worktree{{Path: "/a/main", Head: "deadbeef", Branch: "main"}},
		},
		{
			name: "empty input",
			in:   "",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseWorktrees(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseWorktrees mismatch\nwant: %#v\ngot:  %#v", tc.want, got)
			}
		})
	}
}
