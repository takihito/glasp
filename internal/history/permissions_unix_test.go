//go:build !windows

package history

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func withPermissiveUmask(t *testing.T) {
	t.Helper()
	old := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(old) })
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s: mode %#o, want %#o", path, got, want)
	}
}

func TestAppendCreatesOwnerOnlyFiles(t *testing.T) {
	withPermissiveUmask(t)
	root := t.TempDir()
	if err := Append(root, Entry{Command: "push", Status: "success"}); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	assertMode(t, filepath.Join(root, glaspDirName, historyFileName), 0o600)
	assertMode(t, filepath.Join(root, glaspDirName, lockFileName), 0o600)
}

func TestAppendTightensExistingHistoryFile(t *testing.T) {
	withPermissiveUmask(t)
	root := t.TempDir()
	dir := filepath.Join(root, glaspDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	historyPath := filepath.Join(dir, historyFileName)
	// Simulate a history file written by an older glasp release (0644).
	if err := os.WriteFile(historyPath, nil, 0o644); err != nil {
		t.Fatalf("seed history: %v", err)
	}
	if err := Append(root, Entry{Command: "pull", Status: "success"}); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	assertMode(t, historyPath, 0o600)
}
