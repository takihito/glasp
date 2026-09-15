// Package fsutil provides small filesystem helpers shared across glasp's
// internal packages.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path by writing to a temporary file in the
// same directory and renaming it into place, so a concurrent reader (or a
// process killed mid-write) never observes a partially written file. perm is
// applied to the temporary file before the rename so the final file has the
// requested permissions from the moment it becomes visible at path.
//
// The destination is replaced by a single rename, so it is never observed
// missing: a concurrent reader sees either the old content or the new one.
// The rename does not follow a symlink at path onto another location — if
// path is a symlink, the rename replaces the symlink itself, not the file it
// points at (os.Rename never follows symlinks).
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, ".glasp-tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", dir, err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()
	if err := tempFile.Chmod(perm); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}
	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	// A single rename over the destination is the whole point: the
	// destination is never absent, so a concurrent reader always sees either
	// the old file or the new one, and a crash can't leave the path missing.
	// This holds on Windows too — os.Rename there calls MoveFileEx with
	// MOVEFILE_REPLACE_EXISTING, so it replaces an existing destination
	// rather than failing. Do not reintroduce a "move the original aside
	// first" step: it opens a window where path does not exist, and any
	// fixed backup name (e.g. path+".bak") can clobber a user's own file.
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("failed to replace %s: %w", path, err)
	}
	return nil
}
