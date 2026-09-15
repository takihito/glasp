package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteFileAtomicCreatesFileWithPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := WriteFileAtomic(path, []byte("hello"), 0600); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected 'hello', got %q", data)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("expected perm 0600, got %o", info.Mode().Perm())
		}
	}
}

func TestWriteFileAtomicReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatalf("initial WriteFile failed: %v", err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0644); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("expected 'new', got %q", data)
	}
	// No leftover temp or backup files.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 file in dir, got %d: %v", len(entries), entries)
	}
}

func TestWriteFileAtomicFailsForMissingDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist", "file.txt")
	if err := WriteFileAtomic(path, []byte("data"), 0644); err == nil {
		t.Fatal("expected error for missing parent directory")
	}
}
