//go:build !windows

package osutil

import "os"

// AtomicRename renames src to dst, replacing dst if it exists.
// On Unix this is a simple os.Rename (POSIX rename is atomic).
func AtomicRename(src, dst string) error {
	return os.Rename(src, dst)
}
