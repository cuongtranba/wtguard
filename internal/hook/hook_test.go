package hook

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallOnCleanDir(t *testing.T) {
	dir := t.TempDir()
	res, err := Install(dir, false)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !res.Wrote || res.AlreadyOurs || res.ChainedTo != "" {
		t.Fatalf("unexpected result: %+v", res)
	}
	content, err := os.ReadFile(filepath.Join(dir, "pre-commit"))
	if err != nil {
		t.Fatal(err)
	}
	if !IsOurs(content) {
		t.Fatalf("written shim missing marker: %q", content)
	}
	info, _ := os.Stat(filepath.Join(dir, "pre-commit"))
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("shim not executable: %v", info.Mode())
	}
}

func TestInstallIdempotent(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, false); err != nil {
		t.Fatal(err)
	}
	res, err := Install(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.AlreadyOurs || res.Wrote {
		t.Fatalf("expected AlreadyOurs, got %+v", res)
	}
}

func TestInstallChainsExistingUserHook(t *testing.T) {
	dir := t.TempDir()
	userHook := "#!/bin/sh\necho user hook\n"
	hookPath := filepath.Join(dir, "pre-commit")
	if err := os.WriteFile(hookPath, []byte(userHook), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Install(dir, false)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !res.Wrote || res.ChainedTo == "" {
		t.Fatalf("want chain, got %+v", res)
	}
	chained, err := os.ReadFile(filepath.Join(dir, "pre-commit.local"))
	if err != nil {
		t.Fatal(err)
	}
	if string(chained) != userHook {
		t.Errorf("chained content mismatch: %q", chained)
	}
	// new shim is in place
	newContent, _ := os.ReadFile(hookPath)
	if !IsOurs(newContent) {
		t.Errorf("shim not written")
	}
}

func TestInstallRefusesChainConflict(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pre-commit"), []byte("#!/bin/sh\nuser\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pre-commit.local"), []byte("already there"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Install(dir, false)
	if err == nil {
		t.Fatalf("expected error, got nil; res=%+v", res)
	}
	if !res.ChainConflict {
		t.Errorf("want ChainConflict, got %+v", res)
	}
}

func TestUninstallRestoresChainedHook(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pre-commit"), []byte("#!/bin/sh\nold user\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(dir, false); err != nil {
		t.Fatal(err)
	}
	res, err := Uninstall(dir)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if !res.Removed || res.Restored == "" {
		t.Fatalf("expected Removed+Restored, got %+v", res)
	}
	content, err := os.ReadFile(filepath.Join(dir, "pre-commit"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "#!/bin/sh\nold user\n" {
		t.Errorf("user hook not restored: %q", content)
	}
	if _, err := os.Stat(filepath.Join(dir, "pre-commit.local")); !os.IsNotExist(err) {
		t.Errorf("pre-commit.local still present")
	}
}

func TestUninstallRefusesForeignHook(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pre-commit"), []byte("#!/bin/sh\nnot ours\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Uninstall(dir)
	if err == nil {
		t.Fatal("expected error refusing foreign hook")
	}
	if !res.NotOurs {
		t.Errorf("want NotOurs, got %+v", res)
	}
}

func TestUninstallWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	res, err := Uninstall(dir)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if !res.NotPresent {
		t.Errorf("want NotPresent, got %+v", res)
	}
}

func TestIsInstalled(t *testing.T) {
	dir := t.TempDir()
	ok, _ := IsInstalled(dir)
	if ok {
		t.Error("expected false on empty dir")
	}
	if _, err := Install(dir, false); err != nil {
		t.Fatal(err)
	}
	ok, _ = IsInstalled(dir)
	if !ok {
		t.Error("expected true after Install")
	}
}
