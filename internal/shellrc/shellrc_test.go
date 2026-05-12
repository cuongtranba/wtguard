package shellrc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTargets(t *testing.T) {
	home := "/home/u"
	cases := []struct {
		shell string
		want  []string
	}{
		{"/bin/zsh", []string{home + "/.zshrc"}},
		{"/usr/bin/bash", []string{home + "/.bashrc", home + "/.bash_profile"}},
		{"/bin/fish", []string{home + "/.profile"}},
		{"", []string{home + "/.profile"}},
	}
	for _, tc := range cases {
		got := Targets(tc.shell, home)
		if len(got) != len(tc.want) {
			t.Errorf("Targets(%q) = %v, want %v", tc.shell, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("Targets(%q)[%d] = %s, want %s", tc.shell, i, got[i], tc.want[i])
			}
		}
	}
}

func TestPatchOnEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rc")
	res, err := Patch(path, "/home/u/.wtguard/bin")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Patched {
		t.Fatalf("want Patched=true, got %+v", res)
	}
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), BeginMarker) {
		t.Errorf("begin marker missing: %q", content)
	}
	if !strings.Contains(string(content), `/home/u/.wtguard/bin:$PATH`) {
		t.Errorf("PATH export missing: %q", content)
	}
}

func TestPatchOnExistingNonEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rc")
	existing := "export FOO=bar"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Patch(path, "/x/bin"); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(content), "export FOO=bar") {
		t.Errorf("original content lost: %q", content)
	}
	if !strings.Contains(string(content), BeginMarker) {
		t.Errorf("marker missing: %q", content)
	}
}

func TestPatchIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rc")
	if _, err := Patch(path, "/x/bin"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	res, err := Patch(path, "/x/bin")
	if err != nil {
		t.Fatal(err)
	}
	if !res.AlreadyIn || res.Patched {
		t.Errorf("want AlreadyIn, got %+v", res)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Errorf("file changed on second patch")
	}
}

func TestUnpatchRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rc")
	original := "export FOO=bar\nexport BAZ=qux\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Patch(path, "/x/bin"); err != nil {
		t.Fatal(err)
	}
	res, err := Unpatch(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Cleaned {
		t.Errorf("want Cleaned, got %+v", res)
	}
	content, _ := os.ReadFile(path)
	if strings.Contains(string(content), BeginMarker) {
		t.Errorf("marker still present after Unpatch: %q", content)
	}
	if !strings.Contains(string(content), "export FOO=bar") {
		t.Errorf("original content lost: %q", content)
	}
}

func TestUnpatchAbsentFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist")
	res, err := Unpatch(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Absent {
		t.Errorf("want Absent, got %+v", res)
	}
}

func TestUnpatchIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rc")
	if err := os.WriteFile(path, []byte("untouched\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Unpatch(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Absent {
		t.Errorf("want Absent, got %+v", res)
	}
}
