package diagnostic

import (
	"fmt"
	"strings"
)

// Severity represents the severity level of a diagnostic message
type Severity int

const (
	Error Severity = iota
	Warning
	Info
)

// String returns the string representation of the severity level
func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	case Info:
		return "info"
	default:
		return "unknown"
	}
}

// Diagnostic represents a single compiler error, warning, or info message
type Diagnostic struct {
	Severity Severity
	Message  string
	Line     int
	Column   int
	File     string // optional file path (for multi-file compilation)
	Hint     string // optional suggestion
}

// Diagnostics manages a collection of diagnostic messages
type Diagnostics struct {
	items []Diagnostic
}

// New creates a new empty Diagnostics collection
func New() *Diagnostics {
	return &Diagnostics{
		items: make([]Diagnostic, 0),
	}
}

// Errorf adds an error diagnostic with formatted message
func (d *Diagnostics) Errorf(line, col int, format string, args ...interface{}) {
	d.items = append(d.items, Diagnostic{
		Severity: Error,
		Message:  fmt.Sprintf(format, args...),
		Line:     line,
		Column:   col,
	})
}

// Warningf adds a warning diagnostic with formatted message
func (d *Diagnostics) Warningf(line, col int, format string, args ...interface{}) {
	d.items = append(d.items, Diagnostic{
		Severity: Warning,
		Message:  fmt.Sprintf(format, args...),
		Line:     line,
		Column:   col,
	})
}

// Infof adds an info diagnostic with formatted message
func (d *Diagnostics) Infof(line, col int, format string, args ...interface{}) {
	d.items = append(d.items, Diagnostic{
		Severity: Info,
		Message:  fmt.Sprintf(format, args...),
		Line:     line,
		Column:   col,
	})
}

// ErrorWithHint adds an error diagnostic with an optional hint
func (d *Diagnostics) ErrorWithHint(line, col int, msg, hint string) {
	d.items = append(d.items, Diagnostic{
		Severity: Error,
		Message:  msg,
		Line:     line,
		Column:   col,
		Hint:     hint,
	})
}

// WarningWithHint adds a warning diagnostic with an optional hint
func (d *Diagnostics) WarningWithHint(line, col int, msg, hint string) {
	d.items = append(d.items, Diagnostic{
		Severity: Warning,
		Message:  msg,
		Line:     line,
		Column:   col,
		Hint:     hint,
	})
}

// HasErrors returns true if there are any error-level diagnostics
func (d *Diagnostics) HasErrors() bool {
	for _, item := range d.items {
		if item.Severity == Error {
			return true
		}
	}
	return false
}

// Errors returns only the error-level diagnostics
func (d *Diagnostics) Errors() []Diagnostic {
	errors := make([]Diagnostic, 0)
	for _, item := range d.items {
		if item.Severity == Error {
			errors = append(errors, item)
		}
	}
	return errors
}

// All returns all diagnostics regardless of severity
func (d *Diagnostics) All() []Diagnostic {
	return d.items
}

// Count returns the total number of diagnostics
func (d *Diagnostics) Count() int {
	return len(d.items)
}

// ErrorCount returns the number of error-level diagnostics
func (d *Diagnostics) ErrorCount() int {
	count := 0
	for _, item := range d.items {
		if item.Severity == Error {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warning-level diagnostics
func (d *Diagnostics) WarningCount() int {
	count := 0
	for _, item := range d.items {
		if item.Severity == Warning {
			count++
		}
	}
	return count
}

// ErrorfInFile adds an error diagnostic with file path and formatted message
func (d *Diagnostics) ErrorfInFile(file string, line, col int, format string, args ...interface{}) {
	d.items = append(d.items, Diagnostic{
		Severity: Error,
		Message:  fmt.Sprintf(format, args...),
		Line:     line,
		Column:   col,
		File:     file,
	})
}

// Format returns human-readable error messages
// Output format:
//
//	error[filename:3:10]: undeclared variable 'x'
//	  hint: did you mean 'y'?
//	warning[filename:5:1]: unused variable 'z'
func (d *Diagnostics) Format(filename string) string {
	if len(d.items) == 0 {
		return ""
	}

	var builder strings.Builder
	for i, item := range d.items {
		// Use item.File if set, otherwise use the filename parameter
		fileToUse := filename
		if item.File != "" {
			fileToUse = item.File
		}

		// Format the main diagnostic line
		builder.WriteString(fmt.Sprintf("%s[%s:%d:%d]: %s",
			item.Severity.String(),
			fileToUse,
			item.Line,
			item.Column,
			item.Message,
		))

		// Add hint if present
		if item.Hint != "" {
			builder.WriteString(fmt.Sprintf("\n  hint: %s", item.Hint))
		}

		// Add newline unless it's the last item
		if i < len(d.items)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// Clear removes all diagnostics from the collection
func (d *Diagnostics) Clear() {
	d.items = make([]Diagnostic, 0)
}
