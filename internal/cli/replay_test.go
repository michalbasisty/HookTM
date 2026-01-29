package cli

import (
	"fmt"
	"net"
	"syscall"
	"testing"
)

func TestGetExitCodeFromStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       int
	}{
		{"success 200", 200, 0},
		{"success 201", 201, 0},
		{"success 204", 204, 0},
		{"redirect 301", 301, 3},
		{"client error 400", 400, 2},
		{"client error 404", 404, 2},
		{"server error 500", 500, 2},
		{"server error 503", 503, 2},
		{"unknown 0", 0, 3},
		{"unknown 600", 600, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getExitCodeFromStatus(tt.statusCode)
			if got != tt.want {
				t.Errorf("getExitCodeFromStatus(%d) = %d, want %d", tt.statusCode, got, tt.want)
			}
		})
	}
}

func TestGetExitCodeFromError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "nil error",
			err:  nil,
			want: 0,
		},
		{
			name: "not found error",
			err:  fmt.Errorf("not found: abc123"),
			want: 3,
		},
		{
			name: "connection refused",
			err:  syscall.ECONNREFUSED,
			want: 1,
		},
		{
			name: "connection reset",
			err:  syscall.ECONNRESET,
			want: 1,
		},
		{
			name: "network unreachable",
			err:  syscall.ENETUNREACH,
			want: 1,
		},
		{
			name: "generic error",
			err:  fmt.Errorf("something went wrong"),
			want: 3,
		},
		{
			name: "invalid input error",
			err:  fmt.Errorf("missing id"),
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getExitCodeFromError(tt.err)
			if got != tt.want {
				t.Errorf("getExitCodeFromError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestGetExitCodeFromError_Network(t *testing.T) {
	// Test with net.Error
	netErr := &testNetError{timeout: true}
	got := getExitCodeFromError(netErr)
	if got != 1 {
		t.Errorf("getExitCodeFromError(net.Error timeout) = %d, want 1", got)
	}

	// Test with DNS error
	dnsErr := &net.DNSError{Err: "no such host", Name: "example.com"}
	got = getExitCodeFromError(dnsErr)
	if got != 1 {
		t.Errorf("getExitCodeFromError(DNSError) = %d, want 1", got)
	}
}

// testNetError implements net.Error for testing
type testNetError struct {
	timeout   bool
	temporary bool
}

func (e *testNetError) Error() string   { return "test network error" }
func (e *testNetError) Timeout() bool   { return e.timeout }
func (e *testNetError) Temporary() bool { return e.temporary }
