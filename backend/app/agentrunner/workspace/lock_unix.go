//go:build !windows

package workspace

import (
	"os"
	"syscall"
)

// lockFile takes an exclusive advisory lock, blocking until it is free, so
// two attempts fetching the same mirror serialize instead of corrupting it.
func lockFile(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, err
	}
	return func() {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, nil
}
