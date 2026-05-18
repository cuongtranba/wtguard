// Package audit writes one JSON line per wtguard decision to the global and
// per-repo audit logs.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Layer identifies which enforcement layer recorded the entry.
type Layer string

const (
	LayerProxy Layer = "proxy"
	LayerHook  Layer = "hook"
)

// Action is the git verb the user invoked.
type Action string

const (
	ActionCommit Action = "commit"
	ActionPush   Action = "push"
)

// Decision is what wtguard decided about the action.
type Decision string

const (
	DecisionBlock  Decision = "block"
	DecisionAllow  Decision = "allow"
	DecisionBypass Decision = "bypass"
)

// Entry is one JSON line in the audit log.
type Entry struct {
	Time      time.Time `json:"ts"`
	Repo      string    `json:"repo,omitempty"`
	Branch    string    `json:"branch,omitempty"`
	Action    Action    `json:"action"`
	Decision  Decision  `json:"decision"`
	Reason    string    `json:"reason,omitempty"`
	Worktrees int       `json:"worktrees,omitempty"`
	Args      []string  `json:"args,omitempty"`
	PID       int       `json:"pid"`
	Layer     Layer     `json:"layer"`
	Bypass    bool      `json:"bypass,omitempty"`
	Subject   string    `json:"subject,omitempty"`
}

// GlobalPath returns the path to the user-level audit log.
func GlobalPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".wtguard", "audit.jsonl")
}

// RepoPath returns the per-repo audit log path inside the given .git dir.
func RepoPath(gitDir string) string {
	if gitDir == "" {
		return ""
	}
	return filepath.Join(gitDir, "wtguard.log")
}

// Append writes e to every path in dest. Empty paths are skipped. Errors
// are aggregated but non-fatal callers can ignore them.
func Append(e Entry, dest ...string) error {
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	if e.PID == 0 {
		e.PID = os.Getpid()
	}
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("audit: marshal: %w", err)
	}
	line = append(line, '\n')
	var firstErr error
	for _, p := range dest {
		if p == "" {
			continue
		}
		if err := writeLine(p, line); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func writeLine(path string, line []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("audit: mkdir %s: %w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("audit: open %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("audit: write %s: %w", path, err)
	}
	return nil
}
