package proxy

import (
	"strings"
)

// ParsePush extracts the destination ref(s) of a `git push` invocation.
//
// Accepts the standard forms:
//
//	git push                       → currentBranch (if any)
//	git push origin                → currentBranch (if any)
//	git push origin main           → main
//	git push origin main:main      → main
//	git push origin HEAD:main      → main
//	git push origin +main          → main
//	git push origin :main          → main (delete)
//	git push origin main develop   → main, develop
//
// Flags are ignored. The first positional after any flags is treated as the
// remote (and discarded); remaining positionals are refspecs.
//
// Returns the list of destination branches. `currentBranch` is used when no
// refspec is provided and hasBranch is true.
func ParsePush(subArgs []string, currentBranch string, hasBranch bool) []string {
	var positional []string
	i := 0
	for i < len(subArgs) {
		a := subArgs[i]
		switch {
		case a == "--":
			positional = append(positional, subArgs[i+1:]...)
			i = len(subArgs)
		case strings.HasPrefix(a, "--") && hasValue(a):
			// e.g. --repo=X (already inline)
			i++
		case isFlagWithValue(a):
			i += 2
		case strings.HasPrefix(a, "-"):
			i++
		default:
			positional = append(positional, a)
			i++
		}
	}
	if len(positional) == 0 {
		if hasBranch {
			return []string{currentBranch}
		}
		return nil
	}
	// First positional is the remote, rest are refspecs.
	refspecs := positional[1:]
	if len(refspecs) == 0 {
		if hasBranch {
			return []string{currentBranch}
		}
		return nil
	}
	out := make([]string, 0, len(refspecs))
	for _, r := range refspecs {
		out = append(out, destOfRefspec(r, currentBranch))
	}
	return out
}

func destOfRefspec(refspec, currentBranch string) string {
	// strip leading '+' (force) or '!'
	refspec = strings.TrimPrefix(refspec, "+")
	if _, dst, ok := strings.Cut(refspec, ":"); ok {
		return strings.TrimPrefix(dst, "refs/heads/")
	}
	// no colon → push <src> to same name (src may be HEAD)
	if refspec == "HEAD" {
		return currentBranch
	}
	return strings.TrimPrefix(refspec, "refs/heads/")
}

func isFlagWithValue(a string) bool {
	// `git push` flags that take a SEPARATE argument (i.e. not joined with =).
	// `-u`/`--set-upstream` are booleans and intentionally absent.
	switch a {
	case "-o", "--push-option", "--receive-pack", "--repo", "--exec":
		return true
	}
	return false
}

func hasValue(a string) bool {
	return strings.Contains(a, "=")
}
