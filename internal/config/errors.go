package config

import (
	"fmt"
	"strings"
)

// Severity indicates whether a validation issue must be fixed or is advisory.
type Severity int

const (
	// SeverityError is a hard failure; the source cannot be used as-is.
	SeverityError Severity = iota
	// SeverityWarning is an advisory; the source can still be used but may
	// behave unexpectedly (e.g. no rate limiting on an HTML source).
	SeverityWarning
)

// String returns "error" or "warning".
func (s Severity) String() string {
	if s == SeverityWarning {
		return "warning"
	}
	return "error"
}

// ValidationError is a single issue found while validating a sources YAML file.
type ValidationError struct {
	// Severity distinguishes hard errors from warnings.
	Severity Severity

	// File is the path of the YAML file that contains the issue.
	File string

	// Line is the 1-based line number within the file where the issue was found.
	// 0 means the line could not be determined.
	Line int

	// Field is the dot-path to the offending field, e.g. "sources[1].code".
	Field string

	// Message describes what is wrong in plain English.
	Message string
}

// String renders the error in a compiler-style format:
//
//	path/to/file.yml:12: error: sources[0].code: code is required
func (e ValidationError) String() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s: %s: %s", e.File, e.Line, e.Severity, e.Field, e.Message)
	}
	return fmt.Sprintf("%s: %s: %s: %s", e.File, e.Severity, e.Field, e.Message)
}

// ValidationErrors is an ordered list of ValidationError values.
// It implements the error interface so functions can return it directly
// as an error while still allowing callers to inspect individual issues.
type ValidationErrors []ValidationError

// Error implements the error interface. It renders only error-severity issues
// as a newline-separated string. Callers that want warnings should use Warnings().
func (ve ValidationErrors) Error() string {
	var b strings.Builder
	n := 0
	for _, e := range ve {
		if e.Severity == SeverityError {
			if n > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(e.String())
			n++
		}
	}
	return b.String()
}

// HasErrors returns true if at least one error-severity issue is present.
func (ve ValidationErrors) HasErrors() bool {
	for _, e := range ve {
		if e.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Errors returns the subset of error-severity issues.
func (ve ValidationErrors) Errors() []ValidationError {
	out := make([]ValidationError, 0, len(ve))
	for _, e := range ve {
		if e.Severity == SeverityError {
			out = append(out, e)
		}
	}
	return out
}

// Warnings returns the subset of warning-severity issues.
func (ve ValidationErrors) Warnings() []ValidationError {
	out := make([]ValidationError, 0, len(ve))
	for _, e := range ve {
		if e.Severity == SeverityWarning {
			out = append(out, e)
		}
	}
	return out
}
