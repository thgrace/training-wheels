package shellcontext

// SafeArgMode describes how a command's arguments should be treated.
type SafeArgMode int

const (
	SafeArgNone  SafeArgMode = iota // No special handling
	SafeArgAll                       // ALL arguments are data (e.g., echo, printf)
	SafeArgFlags                     // Only specific flag values are data
)

// SafeCommandEntry describes a command whose args (or specific flag-args) are data.
type SafeCommandEntry struct {
	Mode      SafeArgMode
	SafeFlags map[string]bool // For SafeArgFlags mode: which flags mark their next token as data
}

// safeCommands is the static registry of commands with known-safe argument handling.
var safeCommands = map[string]*SafeCommandEntry{
	// Commands where ALL arguments are data.
	"echo":   {Mode: SafeArgAll},
	"printf": {Mode: SafeArgAll},
	"print":  {Mode: SafeArgAll}, // zsh
	"test":   {Mode: SafeArgAll}, // conditional, not execution
	"[":      {Mode: SafeArgAll}, // same as test
	"jq":     {Mode: SafeArgAll}, // jq expressions, not shell commands

	// Commands where specific flags mark their next token as data.
	"git": {Mode: SafeArgFlags, SafeFlags: map[string]bool{
		"-m": true, "--message": true,
		"--grep": true, "--author": true,
		"-C": true,
	}},
	"grep": {Mode: SafeArgAll}, // All args are patterns or filenames — never executed
	"rg":  {Mode: SafeArgAll}, // ripgrep: patterns and filenames
	"ag":  {Mode: SafeArgAll}, // silver searcher: patterns and filenames
	"ack": {Mode: SafeArgAll}, // ack: patterns and filenames
	"curl": {Mode: SafeArgFlags, SafeFlags: map[string]bool{
		"-d": true, "--data": true,
		"--data-raw": true, "--data-binary": true, "--data-urlencode": true,
		"-H": true, "--header": true,
		"-o": true, "--output": true,
		"-u": true, "--user": true,
	}},
	"wget": {Mode: SafeArgFlags, SafeFlags: map[string]bool{
		"-O": true, "--output-document": true,
		"--header": true,
	}},
	"ssh": {Mode: SafeArgFlags, SafeFlags: map[string]bool{
		"-o": true,
	}},
	"find": {Mode: SafeArgFlags, SafeFlags: map[string]bool{
		"-name": true, "-iname": true,
		"-path": true, "-ipath": true,
		"-regex": true, "-iregex": true,
	}},
	"docker": {Mode: SafeArgFlags, SafeFlags: map[string]bool{
		"--name": true, "--label": true,
		"-e": true, "--env": true,
		"--network": true,
	}},
	"kubectl": {Mode: SafeArgFlags, SafeFlags: map[string]bool{
		"-n": true, "--namespace": true,
		"-l": true, "--selector": true,
		"--field-selector": true,
	}},
	"tar": {Mode: SafeArgFlags, SafeFlags: map[string]bool{
		"-f": true, "--file": true,
		"-C": true, "--directory": true,
	}},

	// Windows-native commands where ALL arguments are data.
	"findstr": {Mode: SafeArgAll}, // Windows grep equivalent
	"where":   {Mode: SafeArgAll}, // Windows which equivalent
	"type":    {Mode: SafeArgAll}, // Windows cat equivalent
	"more":    {Mode: SafeArgAll}, // Windows pager
	"sort":    {Mode: SafeArgAll}, // Sort input
	"fc":      {Mode: SafeArgAll}, // File compare
}

// LookupSafeCommand returns the SafeCommandEntry for a command name, or nil if not found.
func LookupSafeCommand(cmdName string) *SafeCommandEntry {
	return safeCommands[cmdName]
}

// IsSafeFlag checks if, for a given command, a specific flag means "next arg is data".
func IsSafeFlag(cmdName string, flag string) bool {
	entry := safeCommands[cmdName]
	if entry == nil || entry.Mode != SafeArgFlags {
		return false
	}
	return entry.SafeFlags[flag]
}
