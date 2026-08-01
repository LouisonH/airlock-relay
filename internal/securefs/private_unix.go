//go:build !windows

// Package securefs provides platform-aware filesystem checks for Airlock's
// private local state.
package securefs

import (
	"os"
)

// EnsurePrivateDirectory creates a directory and restricts it to its owner.
func EnsurePrivateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return os.ErrPermission
	}
	return os.Chmod(path, 0o700)
}

// IsPrivateRegularFile accepts only a user-only regular file.
func IsPrivateRegularFile(info os.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm()&0o077 == 0
}

// PreparePrivateFile applies user-only permissions to a newly-created file.
func PreparePrivateFile(file *os.File) error {
	return file.Chmod(0o600)
}

// SyncDirectory persists a rename on filesystems that support directory fsync.
func SyncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
