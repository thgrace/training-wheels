// Package exitcodes defines the exit code contract for tw.
package exitcodes

const (
	// Allow: command passed evaluation (or fail-open triggered).
	Allow = 0

	// Deny: command was denied by a pack or config override.
	Deny = 1

	// Warn: command matched a medium/low severity pattern (reserved for Phase 2).
	Warn = 2

	// ConfigError: config parse error, invalid env var value, etc.
	ConfigError = 3

	// ParseError: hook stdin JSON is malformed (only emitted by tw test, not hook mode).
	ParseError = 4

	// IOError: file not found, permission denied, settings.json write failure, etc.
	IOError = 5
)
