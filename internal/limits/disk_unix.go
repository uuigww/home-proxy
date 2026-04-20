//go:build unix

package limits

import "syscall"

// diskFree returns the number of free bytes on the filesystem that contains
// path, via syscall.Statfs. On non-unix targets a stub in disk_other.go
// returns a "not supported" error so the health poller silently skips the
// disk-space warning.
//
// Kept as a package variable so tests can swap it in-place without build
// tags in test code.
var diskFree = func(path string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	// Bavail is the count of blocks available to unprivileged users;
	// multiplied by the filesystem block size it is the free-space
	// figure admins expect from `df`.
	return int64(st.Bavail) * int64(st.Bsize), nil
}
