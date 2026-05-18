package remote

import "testing"

func TestOwnerRepo(t *testing.T) {
	cases := []struct {
		in    string
		owner string
		repo  string
		ok    bool
	}{
		{"git@github.com:cuongtranba/wtguard.git", "cuongtranba", "wtguard", true},
		{"git@github.com:cuongtranba/wtguard", "cuongtranba", "wtguard", true},
		{"https://github.com/cuongtranba/wtguard.git", "cuongtranba", "wtguard", true},
		{"https://github.com/cuongtranba/wtguard", "cuongtranba", "wtguard", true},
		{"ssh://git@github.com/cuongtranba/wtguard.git", "cuongtranba", "wtguard", true},
		{"", "", "", false},
		{"https://gitlab.com/x/y.git", "", "", false},
	}
	for _, tc := range cases {
		owner, repo, ok := OwnerRepo(tc.in)
		if ok != tc.ok || owner != tc.owner || repo != tc.repo {
			t.Errorf("OwnerRepo(%q) = (%q,%q,%v); want (%q,%q,%v)",
				tc.in, owner, repo, ok, tc.owner, tc.repo, tc.ok)
		}
	}
}

func TestUnprotectCommand(t *testing.T) {
	got := UnprotectCommand("o", "r", "main")
	want := "gh api -X DELETE repos/o/r/branches/main/protection"
	if got != want {
		t.Errorf("UnprotectCommand = %q, want %q", got, want)
	}
}
