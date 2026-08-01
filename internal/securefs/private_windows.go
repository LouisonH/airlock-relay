//go:build windows

// Package securefs provides platform-aware filesystem checks for Airlock's
// private local state.
package securefs

import "os"

// EnsurePrivateDirectory validates a local state directory. Desktop-mode
// directories are ACL-restricted by the native launcher before airlockd starts.
// Windows does not expose a portable POSIX permission mask through os.FileMode.
func EnsurePrivateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return os.ErrPermission
	}
	return nil
}

// IsPrivateRegularFile relies on the containing directory's Windows ACL. The
// POSIX-style mode bits reported by os.FileMode are not an ACL security signal
// on Windows and would reject files Airlock itself created.
func IsPrivateRegularFile(info os.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0
}

// PreparePrivateFile intentionally leaves inherited ACLs intact.
func PreparePrivateFile(_ *os.File) error {
	return nil
}

// SyncDirectory is not supported by Windows directory handles. Renames are
// issued only after the temporary file has been flushed and closed.
func SyncDirectory(_ string) error {
	return nil
}
