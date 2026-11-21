// internal/core/callbacks_test.go
package core

import (
	"context"
	"errors"
	"net"
	"testing"
)

type mockNetError struct {
	timeout bool
}

func (m *mockNetError) Error() string   { return "mock network error" }
func (m *mockNetError) Timeout() bool   { return m.timeout }
func (m *mockNetError) Temporary() bool { return false }

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorCategory
	}{
		{
			name:     "Nil error",
			err:      nil,
			expected: ErrorCategoryUnknown,
		},
		{
			name:     "Context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: ErrorCategoryTimeout,
		},
		{
			name:     "Context canceled",
			err:      context.Canceled,
			expected: ErrorCategoryTimeout,
		},
		{
			name:     "Network timeout error",
			err:      &mockNetError{timeout: true},
			expected: ErrorCategoryTimeout,
		},
		{
			name:     "Network non-timeout error",
			err:      &mockNetError{timeout: false},
			expected: ErrorCategoryNetwork,
		},
		{
			name:     "Real net.OpError",
			err:      &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			expected: ErrorCategoryNetwork,
		},
		{
			name:     "DNS error string",
			err:      errors.New("dial tcp: lookup csirc.cyberpolice.ir: no such host"),
			expected: ErrorCategoryNetwork,
		},
		{
			name:     "Robots error",
			err:      errors.New("disallowed by robots.txt"),
			expected: ErrorCategoryRobots,
		},
		{
			name:     "Parse error",
			err:      errors.New("failed to parse HTML"),
			expected: ErrorCategoryParse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyError(tt.err)
			if got != tt.expected {
				t.Errorf("classifyError(%v) = %v, expected %v", tt.err, got, tt.expected)
			}
		})
	}
}
