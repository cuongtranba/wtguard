package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendCreatesAndAppendsJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "audit.jsonl") // exercises mkdir
	for i, action := range []Action{ActionCommit, ActionPush} {
		err := Append(Entry{
			Repo:     "/r",
			Branch:   "main",
			Action:   action,
			Decision: DecisionBlock,
			Reason:   "test",
			Layer:    LayerProxy,
			PID:      1000 + i,
		}, path)
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), data)
	}
	var e0 Entry
	if err := json.Unmarshal([]byte(lines[0]), &e0); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if e0.Action != ActionCommit || e0.Decision != DecisionBlock || e0.Layer != LayerProxy {
		t.Errorf("bad entry: %+v", e0)
	}
	if e0.Time.IsZero() {
		t.Errorf("ts not set")
	}
}

func TestAppendSkipsEmptyPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	if err := Append(Entry{Action: ActionCommit, Decision: DecisionAllow, Layer: LayerHook}, "", path, ""); err != nil {
		t.Fatalf("Append: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"allow"`) {
		t.Errorf("expected entry, got %q", data)
	}
}
