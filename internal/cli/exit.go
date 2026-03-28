package cli

import (
	"errors"
	"fmt"
)

// ExitError carries a process exit code without forcing command handlers to terminate directly.
type ExitError struct {
	Code   int
	Err    error
	Silent bool
}

func (e *ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

func exitError(code int, err error) error {
	return &ExitError{Code: code, Err: err}
}

func silentExit(code int) error {
	return &ExitError{Code: code, Silent: true}
}

func exitErrorf(code int, format string, args ...any) error {
	return exitError(code, fmt.Errorf(format, args...))
}

// ExitCode returns the process exit code for an error returned by Execute.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}

	return 1
}

// ShouldPrintError reports whether main should print an error returned by Execute.
func ShouldPrintError(err error) bool {
	if err == nil {
		return false
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return !exitErr.Silent && exitErr.Err != nil
	}

	return true
}
