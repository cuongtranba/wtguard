//go:build integration

package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestEndToEndBlocksCommitOnProtectedBranch builds the binary, initializes a
// real git repo, adds a worktree (so the count is > 1), points the hook at
// the built binary via WTGUARD_BIN, and asserts that `git commit` is
// rejected with exit 1 and a wtguard-prefixed stderr message.
//
// Run with: go test -tags=integration ./internal/cli -run EndToEnd
func TestEndToEndBlocksCommitOnProtectedBranch(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "wtguard")

	// Build the binary into a known location.
	root := repoRoot(t)
	build := exec.Command("go", "build", "-o", binPath, "./cmd/wtguard")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	// Init a real git repo and configure identity for the test.
	repo := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	must(t, runCmd(repo, "git", "init", "-q", "-b", "main"))
	must(t, runCmd(repo, "git", "config", "user.email", "test@example.com"))
	must(t, runCmd(repo, "git", "config", "user.name", "test"))
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	must(t, runCmd(repo, "git", "add", "README"))
	must(t, runCmd(repo, "git", "commit", "-q", "-m", "init"))

	// Install hook via the built binary (point HOME at tmp so we don't touch
	// the real user dir).
	env := append(os.Environ(),
		"WTGUARD_BIN="+binPath,
		"WTGUARD_DIR="+filepath.Join(tmp, ".wtguard"),
		"HOME="+tmp,
	)
	installCmd := exec.Command(binPath, "--repo", repo, "install")
	installCmd.Env = env
	if out, err := installCmd.CombinedOutput(); err != nil {
		t.Fatalf("install failed: %v\n%s", err, out)
	}

	// Add a worktree so the count crosses the threshold.
	feat := filepath.Join(tmp, "feat-x")
	must(t, runCmd(repo, "git", "worktree", "add", "-b", "feat-x", feat))

	// Try to commit on main — must be blocked.
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	must(t, runCmd(repo, "git", "add", "README"))
	commit := exec.Command("git", "commit", "-m", "bad")
	commit.Dir = repo
	commit.Env = env
	var stderr bytes.Buffer
	commit.Stderr = &stderr
	err := commit.Run()
	if err == nil {
		t.Fatalf("expected commit to be blocked, got exit 0\nstderr: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "wtguard:") {
		t.Errorf("stderr missing wtguard prefix:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "main") {
		t.Errorf("stderr missing branch name:\n%s", stderr.String())
	}

	// Commit in the worktree should work fine.
	if err := os.WriteFile(filepath.Join(feat, "FEAT"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	must(t, runCmd(feat, "git", "add", "FEAT"))
	out, err := runCmdOut(feat, env, "git", "commit", "-q", "-m", "good")
	if err != nil {
		t.Fatalf("commit on feat-x should succeed: %v\n%s", err, out)
	}

	// Bypass — main commit should now succeed.
	bypassEnv := append(env, "WTGUARD_BYPASS=1")
	out, err = runCmdOut(repo, bypassEnv, "git", "commit", "-m", "bypass")
	if err != nil {
		t.Fatalf("bypass commit failed: %v\n%s", err, out)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func runCmd(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &cmdErr{cmd: name + " " + strings.Join(args, " "), out: string(out), err: err}
	}
	return nil
}

func runCmdOut(dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

type cmdErr struct {
	cmd string
	out string
	err error
}

func (c *cmdErr) Error() string { return c.cmd + ": " + c.err.Error() + "\n" + c.out }

// repoRoot returns the absolute path to the wtguard repo root.
func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}
