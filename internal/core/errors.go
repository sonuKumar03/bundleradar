package core

import "fmt"

// UsageError indicates invalid flags, CLI arguments, or user configuration.
// It maps to the documented usage exit code (2).
type UsageError struct {
	Err error
}

func (e *UsageError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "usage error"
}

func (e *UsageError) Unwrap() error {
	return e.Err
}

// UsageErrorf creates a UsageError with formatted message.
func UsageErrorf(format string, args ...any) *UsageError {
	return &UsageError{Err: fmt.Errorf(format, args...)}
}

// PolicyViolationError indicates a budget breach or disallowed package policy violation.
// It maps to the documented policy-violation exit code (1).
type PolicyViolationError struct {
	Err error
}

func (e *PolicyViolationError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "bundle policy violation"
}

func (e *PolicyViolationError) Unwrap() error {
	return e.Err
}
