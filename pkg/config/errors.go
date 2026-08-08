package config

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidConfig marks configuration parsing or validation failures.
var ErrInvalidConfig = errors.New("invalid configforge configuration")

// Position identifies a location in a configuration file.
type Position struct {
	Line   int
	Column int
}

// FieldError describes one configuration validation error.
type FieldError struct {
	File    string
	Line    int
	Column  int
	Path    string
	Message string
}

// Error returns a human-readable validation error.
func (e FieldError) Error() string {
	var b strings.Builder
	if e.File != "" {
		b.WriteString(e.File)
	}
	if e.Line > 0 {
		if b.Len() > 0 {
			b.WriteByte(':')
		}
		b.WriteString(fmt.Sprintf("%d", e.Line))
		if e.Column > 0 {
			b.WriteString(fmt.Sprintf(":%d", e.Column))
		}
	}
	if b.Len() > 0 {
		b.WriteString(": ")
	}
	if e.Path != "" {
		b.WriteString(e.Path)
		b.WriteByte(' ')
	}
	b.WriteString(e.Message)
	return b.String()
}

// ValidationError contains all validation failures discovered for a configuration file.
type ValidationError struct {
	Errors []FieldError
}

// Error returns all validation errors joined by newlines.
func (e *ValidationError) Error() string {
	if e == nil || len(e.Errors) == 0 {
		return ErrInvalidConfig.Error()
	}

	parts := make([]string, 0, len(e.Errors))
	for _, fieldErr := range e.Errors {
		parts = append(parts, fieldErr.Error())
	}
	return strings.Join(parts, "\n")
}

// Is allows errors.Is(err, ErrInvalidConfig).
func (e *ValidationError) Is(target error) bool {
	return target == ErrInvalidConfig
}

func newFieldError(file string, positions map[string]Position, path, message string) FieldError {
	position := positions[path]
	return FieldError{
		File:    file,
		Line:    position.Line,
		Column:  position.Column,
		Path:    path,
		Message: message,
	}
}

func appendFieldError(errs []FieldError, file string, positions map[string]Position, path, message string) []FieldError {
	return append(errs, newFieldError(file, positions, path, message))
}
