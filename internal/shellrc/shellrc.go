// Package shellrc patches a user's shell rc file with a marker-guarded
// block that prepends wtguard's bin dir to PATH.
package shellrc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	BeginMarker = "# >>> wtguard >>>"
	EndMarker   = "# <<< wtguard <<<"
)

// Targets returns the rc files we patch for the given $SHELL.
// Unknown shells default to ~/.profile.
func Targets(shellEnv, home string) []string {
	switch {
	case strings.Contains(shellEnv, "zsh"):
		return []string{filepath.Join(home, ".zshrc")}
	case strings.Contains(shellEnv, "bash"):
		return []string{
			filepath.Join(home, ".bashrc"),
			filepath.Join(home, ".bash_profile"),
		}
	default:
		return []string{filepath.Join(home, ".profile")}
	}
}

// AllKnownTargets returns every rc file we may have touched. Useful for
// uninstall, where we strip markers regardless of current $SHELL.
func AllKnownTargets(home string) []string {
	return []string{
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".profile"),
	}
}

// PatchResult reports what Patch did.
type PatchResult struct {
	Path      string
	Patched   bool // we appended the block now
	AlreadyIn bool // marker block already present
}

// Patch appends the marker-guarded PATH export to path. Creates the file if
// missing. No-op if marker is already present.
func Patch(path, binDir string) (PatchResult, error) {
	res := PatchResult{Path: path}
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return res, fmt.Errorf("shellrc: read %s: %w", path, err)
	}
	if bytes.Contains(content, []byte(BeginMarker)) {
		res.AlreadyIn = true
		return res, nil
	}
	block := buildBlock(binDir)
	// ensure a leading newline so we don't glue onto the previous line
	if len(content) > 0 && !bytes.HasSuffix(content, []byte("\n")) {
		block = "\n" + block
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return res, fmt.Errorf("shellrc: open %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(block); err != nil {
		return res, fmt.Errorf("shellrc: write %s: %w", path, err)
	}
	res.Patched = true
	return res, nil
}

// Unpatch removes the marker block from path. No-op if absent.
type UnpatchResult struct {
	Path    string
	Cleaned bool
	Absent  bool
}

func Unpatch(path string) (UnpatchResult, error) {
	res := UnpatchResult{Path: path}
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			res.Absent = true
			return res, nil
		}
		return res, fmt.Errorf("shellrc: read %s: %w", path, err)
	}
	if !bytes.Contains(content, []byte(BeginMarker)) {
		res.Absent = true
		return res, nil
	}
	out := stripBlock(content)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return res, fmt.Errorf("shellrc: write %s: %w", path, err)
	}
	res.Cleaned = true
	return res, nil
}

func buildBlock(binDir string) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(BeginMarker)
	b.WriteString("\n")
	// Self-idempotent: bash sources both ~/.bash_profile (login) and
	// ~/.bashrc (interactive), so without this guard PATH would be
	// prepended twice.
	fmt.Fprintf(&b, "case \":$PATH:\" in *:%q:*) ;; *) export PATH=%q ;; esac\n",
		binDir, binDir+":$PATH")
	b.WriteString(EndMarker)
	b.WriteString("\n")
	return b.String()
}

func stripBlock(content []byte) []byte {
	var out bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	skip := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimRight(line, " \t")
		if strings.HasPrefix(trimmed, BeginMarker) {
			skip = true
			continue
		}
		if skip {
			if strings.HasPrefix(trimmed, EndMarker) {
				skip = false
			}
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	// Drop a single trailing blank line we may have introduced by writing
	// "\n" before the marker block.
	b := out.Bytes()
	for len(b) >= 2 && b[len(b)-1] == '\n' && b[len(b)-2] == '\n' {
		b = b[:len(b)-1]
	}
	return b
}
