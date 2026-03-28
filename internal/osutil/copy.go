package osutil

import (
	"io"
	"os"
)

// CopyFile copies a file from src to dst. It overwrites dst if it exists.
// It uses an atomic temp-file-then-rename pattern to avoid corrupting dst
// on partial writes. It sets the destination file mode to 0755 (executable).
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}

	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}

	if err = out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	if err = AtomicRename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}

	return nil
}
