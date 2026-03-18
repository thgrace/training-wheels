//go:build windows

package osutil

import (
	"errors"
	"os"
)

// AtomicRename renames src to dst, replacing dst if it exists.
// On Windows, os.Rename cannot replace an existing file, so we use a
// backup-and-restore strategy to avoid data loss if the rename fails:
//  1. Try a direct rename (works when dst does not exist).
//  2. Rename dst to a temporary backup.
//  3. Rename src to dst.
//  4. On failure, restore the backup; on success, remove it (best-effort).
func AtomicRename(src, dst string) error {
	// Fast path: if dst does not exist, a direct rename succeeds.
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// dst likely exists — move it to a backup so we can retry.
	backup := dst + ".twbak"
	if err := os.Rename(dst, backup); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.Rename(src, dst)
		}
		return err
	}

	// Now dst is free; rename src into its place.
	if err := os.Rename(src, dst); err != nil {
		// Restore the backup so the original dst is not lost.
		_ = os.Rename(backup, dst)
		return err
	}

	// Success — clean up the backup (best-effort).
	_ = os.Remove(backup)
	return nil
}
