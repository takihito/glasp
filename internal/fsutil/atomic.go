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
// The rename replaces path's content atomically on POSIX systems but does
// not itself follow a symlink at path onto another location — if path is a
// symlink, the rename target is the symlink location, not the link's target
// (os.Rename never follows symlinks).
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
	// On Windows, os.Rename fails if the destination already exists. Back up
	// the original first so it can be restored if the rename fails partway.
	backupPath := path + ".bak"
	hadOriginal := false
	if _, err := os.Stat(path); err == nil {
		hadOriginal = true
		_ = os.Remove(backupPath) // remove a stale backup, if any
		if err := os.Rename(path, backupPath); err != nil {
			return fmt.Errorf("failed to back up existing file %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat existing file %s: %w", path, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		if hadOriginal {
			if restoreErr := os.Rename(backupPath, path); restoreErr != nil {
				return fmt.Errorf("failed to replace %s: %w (also failed to restore backup: %v)", path, err, restoreErr)
			}
		}
		return fmt.Errorf("failed to replace %s: %w", path, err)
	}
	if hadOriginal {
		_ = os.Remove(backupPath)
	}
	return nil
}
