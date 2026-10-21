package storage

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"strings"
)

func tryLock(f *os.File) (bool, error) {
	err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}
	return err == nil, err
}
func unlock(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &windows.Overlapped{})
}
func longPath(s string) string {
	if strings.HasPrefix(s, `\\?\`) {
		return s
	}
	if strings.HasPrefix(s, `\\`) {
		return `\\?\UNC\` + s[2:]
	}
	return `\\?\` + s
}
func replace(from, to string) error {
	a, e := windows.UTF16PtrFromString(longPath(from))
	if e != nil {
		return e
	}
	b, e := windows.UTF16PtrFromString(longPath(to))
	if e != nil {
		return e
	}
	return windows.MoveFileEx(a, b, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

// MoveFileEx requests write-through; this is not a power-loss guarantee.
func syncDirectory(string) error { return nil }
