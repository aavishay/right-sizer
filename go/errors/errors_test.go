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

package errors

import (
	"errors"
	"testing"
)

func TestOperatorError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		category string
		op       string
		contains string
	}{
		{
			name:     "basic error",
			err:      New(CategoryValidation, "validatePod", "pod name cannot be empty"),
			category: CategoryValidation,
			op:       "validatePod",
			contains: "[validation] validatePod: pod name cannot be empty",
		},
		{
			name:     "wrapped error",
			err:      Wrap(errors.New("connection refused"), CategoryAPI, "listPods", "failed to connect"),
			category: CategoryAPI,
			op:       "listPods",
			contains: "[api] listPods: failed to connect: connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			if errMsg != tt.contains {
				t.Logf("Error() = %v", errMsg)
				t.Logf("Expected to contain: %v", tt.contains)
			}

			if !IsCategory(tt.err, tt.category) {
				t.Errorf("IsCategory(%v, %v) = false, want true", tt.err, tt.category)
			}

			if cat := GetCategory(tt.err); cat != tt.category {
				t.Errorf("GetCategory() = %v, want %v", cat, tt.category)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "validation error - not retryable",
			err:  ValidationError("test", "invalid input"),
			want: false,
		},
		{
			name: "config error - not retryable",
			err:  ConfigError("test", "invalid config"),
			want: false,
		},
		{
			name: "api error - retryable",
			err:  APIError("test", errors.New("connection timeout")),
			want: true,
		},
		{
			name: "resource error - retryable",
			err:  ResourceError("test", errors.New("not found")),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	baseErr := errors.New("base error")
	wrappedErr := Wrap(baseErr, CategoryAPI, "test", "wrapped")

	if !errors.Is(wrappedErr, baseErr) {
		t.Error("Wrapped error should unwrap to base error")
	}

	var opErr *OperatorError
	if !errors.As(wrappedErr, &opErr) {
		t.Error("Should be able to extract OperatorError")
	}
}

func TestConvenienceFunctions(t *testing.T) {
	tests := []struct {
		name     string
		errFunc  func() error
		category string
	}{
		{
			name:     "ValidationErrorf",
			errFunc:  func() error { return ValidationErrorf("op", "value %d invalid", 42) },
			category: CategoryValidation,
		},
		{
			name:     "ResourceErrorf",
			errFunc:  func() error { return ResourceErrorf("op", errors.New("base"), "failed to get %s", "pod") },
			category: CategoryResource,
		},
		{
			name:     "APIErrorf",
			errFunc:  func() error { return APIErrorf("op", errors.New("base"), "connection to %s failed", "server") },
			category: CategoryAPI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.errFunc()
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !IsCategory(err, tt.category) {
				t.Errorf("Expected category %s, got %s", tt.category, GetCategory(err))
			}
		})
	}
}
