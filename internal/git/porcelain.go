package git

import (
	"bufio"
	"strings"
)

// parseWorktrees parses `git worktree list --porcelain` output.
//
// Format (each record separated by blank line):
//
//	worktree /path/to/wt
//	HEAD <sha>
//	branch refs/heads/<name>      (or `detached`, or `bare`)
//
// A `bare` entry has no HEAD/branch.
func parseWorktrees(out string) []Worktree {
	var (
		result []Worktree
		cur    Worktree
		seen   bool
	)
	flush := func() {
		if seen {
			result = append(result, cur)
		}
		cur = Worktree{}
		seen = false
	}
	s := bufio.NewScanner(strings.NewReader(out))
	for s.Scan() {
		line := s.Text()
		if line == "" {
			flush()
			continue
		}
		seen = true
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			cur.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		case line == "bare":
			cur.Bare = true
		case line == "detached":
			cur.Detached = true
		}
	}
	flush()
	return result
}
