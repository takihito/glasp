//go:build windows

package history

import (
	"fmt"
	"os"
	"syscall"
)

func acquireFileLock(lockPath string) (*os.File, error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}
	var ol syscall.Overlapped
	if err := syscall.LockFileEx(syscall.Handle(f.Fd()), syscall.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &ol); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to lock file: %w", err)
	}
	return f, nil
}

func releaseFileLock(f *os.File) {
	if f == nil {
		return
	}
	var ol syscall.Overlapped
	_ = syscall.UnlockFileEx(syscall.Handle(f.Fd()), 0, 1, 0, &ol)
	_ = f.Close()
}
