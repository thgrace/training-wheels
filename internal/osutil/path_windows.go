//go:build windows

package osutil

import (
	"os/exec"
	"strings"
)

// AddToPath adds a directory to the user's PATH environment variable using PowerShell.
func AddToPath(dir string) (bool, error) {
	// Escape for PowerShell single-quoted string: the only special character
	// inside single quotes is ' itself, which becomes ''.
	escaped := strings.ReplaceAll(dir, "'", "''")

	// Command to update the user-level PATH via PowerShell.
	// It only adds the directory if it's not already in the PATH.
	// Uses single-quoted string to prevent variable expansion and injection.
	psCmd := `
$dirToAdd = '` + escaped + `'
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -notlike "*$dirToAdd*") {
    $newPath = $currentPath + ";" + $dirToAdd
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    return $true
}
return $false
`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(string(output)) == "True", nil
}
