package cli

import (
	"testing"
)

func TestGetExitCode(t *testing.T) {
	tests := []struct {
		name       string
		hasError   bool
		errType    string
		statusCode int
		want       int
	}{
		{
			name:       "success - 200 OK",
			hasError:   false,
			statusCode: 200,
			want:       0,
		},
		{
			name:       "success - 201 Created",
			hasError:   false,
			statusCode: 201,
			want:       0,
		},
		{
			name:       "HTTP error - 400 Bad Request",
			hasError:   false,
			statusCode: 400,
			want:       2,
		},
		{
			name:       "HTTP error - 500 Internal Server Error",
			hasError:   false,
			statusCode: 500,
			want:       2,
		},
		{
			name:       "HTTP error - 404 Not Found",
			hasError:   false,
			statusCode: 404,
			want:       2,
		},
		{
			name:     "connection error",
			hasError: true,
			errType:  "connection",
			want:     1,
		},
		{
			name:     "not found error",
			hasError: true,
			errType:  "notfound",
			want:     3,
		},
		{
			name:     "other error",
			hasError: true,
			errType:  "other",
			want:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got int
			if tt.hasError {
				var err error
				switch tt.errType {
				case "connection":
					err = &connectionError{msg: "connection refused"}
				case "notfound":
					err = &notFoundError{msg: "webhook not found"}
				default:
					err = &genericError{msg: "some error"}
				}
				got = getExitCodeFromError(err)
			} else {
				got = getExitCodeFromStatus(tt.statusCode)
			}
			if got != tt.want {
				t.Errorf("getExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

// Test error types
type connectionError struct {
	msg string
}

func (e *connectionError) Error() string { return e.msg }

type notFoundError struct {
	msg string
}

func (e *notFoundError) Error() string { return e.msg }

type genericError struct {
	msg string
}

func (e *genericError) Error() string { return e.msg }
