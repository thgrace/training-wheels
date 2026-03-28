package osutil

import "runtime"

// IsWindows returns true if the current operating system is Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}
