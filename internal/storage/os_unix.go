//go:build linux || darwin

package storage

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
)

func tryLock(f *os.File) (bool, error) {
	e := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(e, unix.EWOULDBLOCK) || errors.Is(e, unix.EAGAIN) {
		return false, nil
	}
	return e == nil, e
}
func unlock(f *os.File) error       { return unix.Flock(int(f.Fd()), unix.LOCK_UN) }
func replace(from, to string) error { return os.Rename(from, to) }
func sharingViolation(error) bool   { return false }
func syncDirectory(dir string) error {
	f, e := os.Open(dir)
	if e != nil {
		return e
	}
	return errors.Join(f.Sync(), f.Close())
}
