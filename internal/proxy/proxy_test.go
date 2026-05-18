package proxy

import (
	"reflect"
	"runtime"
	"testing"
)

func TestParseGlobalFlags(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		wantSub     string
		wantSubArgs []string
		wantWorkDir string
		wantGitDir  string
	}{
		{
			name:    "plain commit",
			args:    []string{"commit", "-m", "msg"},
			wantSub: "commit", wantSubArgs: []string{"-m", "msg"},
		},
		{
			name:        "C dir before subcommand",
			args:        []string{"-C", "/some/dir", "commit", "-am", "x"},
			wantSub:     "commit",
			wantSubArgs: []string{"-am", "x"},
			wantWorkDir: "/some/dir",
		},
		{
			name:       "git-dir with =",
			args:       []string{"--git-dir=/x", "status"},
			wantSub:    "status",
			wantGitDir: "/x",
		},
		{
			name:        "c key=value passes through",
			args:        []string{"-c", "core.pager=cat", "log"},
			wantSub:     "log",
			wantSubArgs: nil,
		},
		{
			name:    "no subcommand",
			args:    []string{"--version"},
			wantSub: "",
		},
		{
			name:    "version as long flag",
			args:    []string{"--no-pager", "status"},
			wantSub: "status",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := parseGlobalFlags(tc.args)
			if g.Subcommand != tc.wantSub {
				t.Errorf("Subcommand = %q, want %q", g.Subcommand, tc.wantSub)
			}
			if tc.wantSubArgs != nil && !reflect.DeepEqual(g.SubArgs, tc.wantSubArgs) {
				t.Errorf("SubArgs = %v, want %v", g.SubArgs, tc.wantSubArgs)
			}
			if g.WorkDir != tc.wantWorkDir {
				t.Errorf("WorkDir = %q, want %q", g.WorkDir, tc.wantWorkDir)
			}
			if g.GitDir != tc.wantGitDir {
				t.Errorf("GitDir = %q, want %q", g.GitDir, tc.wantGitDir)
			}
		})
	}
}

func TestParsePush(t *testing.T) {
	cases := []struct {
		name          string
		args          []string
		currentBranch string
		hasBranch     bool
		want          []string
	}{
		{"no args, on main", nil, "main", true, []string{"main"}},
		{"no args, detached", nil, "", false, nil},
		{"remote only", []string{"origin"}, "main", true, []string{"main"}},
		{"branch only", []string{"origin", "main"}, "", false, []string{"main"}},
		{"src:dst", []string{"origin", "HEAD:main"}, "feat", true, []string{"main"}},
		{"force +ref", []string{"origin", "+main"}, "", false, []string{"main"}},
		{"delete :ref", []string{"origin", ":main"}, "", false, []string{"main"}},
		{"refs/heads prefix", []string{"origin", "refs/heads/main"}, "", false, []string{"main"}},
		{"multiple refspecs", []string{"origin", "main", "develop"}, "", false, []string{"main", "develop"}},
		{"with flags", []string{"--force", "origin", "main"}, "", false, []string{"main"}},
		{"with -o flag value", []string{"-o", "ci.skip", "origin", "main"}, "", false, []string{"main"}},
		{"set-upstream", []string{"-u", "origin", "main"}, "", false, []string{"main"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParsePush(tc.args, tc.currentBranch, tc.hasBranch)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParsePush(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestHasTerminalGlobalFlag(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"help before subcommand", []string{"--help", "commit"}, true},
		{"version alone", []string{"--version"}, true},
		{"short help", []string{"-h"}, true},
		{"plain commit", []string{"commit", "-m", "x"}, false},
		{"no-pager status", []string{"--no-pager", "status"}, false},
		{"commit --help is subcmd flag, not global", []string{"commit", "--help"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := parseGlobalFlags(tc.args)
			if got := g.hasTerminalGlobalFlag(); got != tc.want {
				t.Errorf("hasTerminalGlobalFlag(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestStripPath(t *testing.T) {
	sep := string(pathListSepFor(t))
	in := "/usr/local/bin" + sep + "/home/u/.wtguard/bin" + sep + "/usr/bin"
	out := stripPath(in, "/home/u/.wtguard/bin")
	want := "/usr/local/bin" + sep + "/usr/bin"
	if out != want {
		t.Errorf("stripPath = %q, want %q", out, want)
	}
	// repeat dir entries all stripped
	out = stripPath("/x"+sep+"/x"+sep+"/y", "/x")
	want = "/y"
	if out != want {
		t.Errorf("stripPath repeats = %q, want %q", out, want)
	}
}

func pathListSepFor(_ *testing.T) byte {
	if runtime.GOOS == "windows" {
		return ';'
	}
	return ':'
}
