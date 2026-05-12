// Package remote applies GitHub branch protection via the `gh` CLI.
//
// This is the opt-in third defense layer. It is best-effort: missing `gh`,
// no auth, or no `origin` remote all yield a clear error without disturbing
// the local install.
package remote

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// OwnerRepo extracts owner/repo from a GitHub URL.
//
// Accepts: git@github.com:owner/repo.git, https://github.com/owner/repo.git,
// https://github.com/owner/repo, ssh://git@github.com/owner/repo.
func OwnerRepo(originURL string) (owner, repo string, ok bool) {
	originURL = strings.TrimSpace(originURL)
	if originURL == "" {
		return "", "", false
	}
	// strip trailing .git
	url := strings.TrimSuffix(originURL, ".git")
	// SSH form: git@github.com:owner/repo
	if m := sshRE.FindStringSubmatch(url); len(m) == 3 {
		return m[1], m[2], true
	}
	// HTTPS or ssh:// form
	if m := webRE.FindStringSubmatch(url); len(m) == 3 {
		return m[1], m[2], true
	}
	return "", "", false
}

var (
	sshRE = regexp.MustCompile(`git@github\.com:([^/]+)/(.+)$`)
	webRE = regexp.MustCompile(`github\.com[/:]([^/]+)/([^/]+)$`)
)

// CheckPrereqs verifies gh is installed and authenticated.
func CheckPrereqs() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("remote: `gh` CLI not found in PATH — install from https://cli.github.com/")
	}
	cmd := exec.Command("gh", "auth", "status")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remote: gh not authenticated — run `gh auth login`")
	}
	return nil
}

// ProtectBranch applies branch protection to one branch via gh api.
//
// Settings: require 1 PR review, require linear history, do not enforce on
// admins (so the user can still administer their own repo if needed).
func ProtectBranch(owner, repo, branch string) error {
	path := fmt.Sprintf("repos/%s/%s/branches/%s/protection", owner, repo, branch)
	args := []string{
		"api", "-X", "PUT", path,
		"-F", "required_pull_request_reviews.required_approving_review_count=1",
		"-F", "required_linear_history=true",
		"-F", "enforce_admins=false",
		"-F", "required_status_checks=",
		"-F", "restrictions=",
	}
	cmd := exec.Command("gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remote: gh api %s: %w: %s", path, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// UnprotectCommand returns the manual command a user should run to remove
// branch protection. We do not run it automatically because it changes
// shared state.
func UnprotectCommand(owner, repo, branch string) string {
	return fmt.Sprintf("gh api -X DELETE repos/%s/%s/branches/%s/protection",
		owner, repo, branch)
}
