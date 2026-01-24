// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Package errors provides standardized error handling utilities for the right-sizer operator.
// This package implements v0.3.0 roadmap goal: Standardize error wrapping and reporting.
package errors

import (
	"errors"
	"fmt"
)

// Error categories for structured error handling
const (
	CategoryValidation    = "validation"
	CategoryResource      = "resource"
	CategoryAPI           = "api"
	CategoryConfiguration = "configuration"
	CategoryInternal      = "internal"
	CategoryMetrics       = "metrics"
	CategoryRemediation   = "remediation"
)

// OperatorError represents a structured error with category and context
type OperatorError struct {
	Category string
	Op       string // Operation that failed
	Err      error  // Underlying error
	Message  string // Human-readable message
}

// Error implements the error interface
func (e *OperatorError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("[%s] %s: %s: %v", e.Category, e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s: %v", e.Category, e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *OperatorError) Unwrap() error {
	return e.Err
}

// Is implements error matching for errors.Is
func (e *OperatorError) Is(target error) bool {
	t, ok := target.(*OperatorError)
	if !ok {
		return false
	}
	return e.Category == t.Category && (t.Op == "" || e.Op == t.Op)
}

// Wrap wraps an error with operation context and category
func Wrap(err error, category, op, message string) error {
	if err == nil {
		return nil
	}
	return &OperatorError{
		Category: category,
		Op:       op,
		Err:      err,
		Message:  message,
	}
}

// Wrapf wraps an error with formatted message
func Wrapf(err error, category, op, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return &OperatorError{
		Category: category,
		Op:       op,
		Err:      err,
		Message:  fmt.Sprintf(format, args...),
	}
}

// New creates a new OperatorError without wrapping an existing error
func New(category, op, message string) error {
	return &OperatorError{
		Category: category,
		Op:       op,
		Err:      errors.New(message),
		Message:  message,
	}
}

// Newf creates a new OperatorError with formatted message
func Newf(category, op, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return &OperatorError{
		Category: category,
		Op:       op,
		Err:      errors.New(msg),
		Message:  msg,
	}
}

// IsCategory checks if an error belongs to a specific category
func IsCategory(err error, category string) bool {
	var opErr *OperatorError
	if errors.As(err, &opErr) {
		return opErr.Category == category
	}
	return false
}

// GetCategory extracts the category from an error, returns empty string if not an OperatorError
func GetCategory(err error) string {
	var opErr *OperatorError
	if errors.As(err, &opErr) {
		return opErr.Category
	}
	return ""
}

// IsRetryable determines if an error should be retried
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	
	// Validation errors are not retryable
	if IsCategory(err, CategoryValidation) {
		return false
	}
	
	// Configuration errors are not retryable
	if IsCategory(err, CategoryConfiguration) {
		return false
	}
	
	// API and resource errors are typically retryable
	if IsCategory(err, CategoryAPI) || IsCategory(err, CategoryResource) {
		return true
	}
	
	// Default to non-retryable for safety
	return false
}

// Common error constructors for frequently used patterns

// ValidationError creates a validation error
func ValidationError(op, message string) error {
	return New(CategoryValidation, op, message)
}

// ValidationErrorf creates a validation error with formatting
func ValidationErrorf(op, format string, args ...interface{}) error {
	return Newf(CategoryValidation, op, format, args...)
}

// ResourceError wraps a resource-related error
func ResourceError(op string, err error) error {
	return Wrap(err, CategoryResource, op, "")
}

// ResourceErrorf wraps a resource-related error with message
func ResourceErrorf(op string, err error, format string, args ...interface{}) error {
	return Wrapf(err, CategoryResource, op, format, args...)
}

// APIError wraps an API-related error
func APIError(op string, err error) error {
	return Wrap(err, CategoryAPI, op, "")
}

// APIErrorf wraps an API-related error with message
func APIErrorf(op string, err error, format string, args ...interface{}) error {
	return Wrapf(err, CategoryAPI, op, format, args...)
}

// ConfigError creates a configuration error
func ConfigError(op, message string) error {
	return New(CategoryConfiguration, op, message)
}

// ConfigErrorf creates a configuration error with formatting
func ConfigErrorf(op, format string, args ...interface{}) error {
	return Newf(CategoryConfiguration, op, format, args...)
}

// MetricsError wraps a metrics-related error
func MetricsError(op string, err error) error {
	return Wrap(err, CategoryMetrics, op, "")
}

// MetricsErrorf wraps a metrics-related error with message
func MetricsErrorf(op string, err error, format string, args ...interface{}) error {
	return Wrapf(err, CategoryMetrics, op, format, args...)
}

// RemediationError wraps a remediation-related error
func RemediationError(op string, err error) error {
	return Wrap(err, CategoryRemediation, op, "")
}

// RemediationErrorf wraps a remediation-related error with message
func RemediationErrorf(op string, err error, format string, args ...interface{}) error {
	return Wrapf(err, CategoryRemediation, op, format, args...)
}
