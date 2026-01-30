package urlutil

import (
	"testing"
)

func TestSingleJoiningSlash(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want string
	}{
		{"both empty", "", "", "/"},
		{"a empty", "", "b", "/b"},
		{"b empty", "a", "", "a/"},
		{"both slashes", "/a/", "/b", "/a/b"},
		{"a slash", "/a/", "b", "/a/b"},
		{"b slash", "a", "/b", "a/b"},
		{"no slashes", "a", "b", "a/b"},
		{"b starts with slash", "", "/b", "/b"},
		{"both have slashes", "api/", "/v1", "api/v1"},
		{"nested paths", "/api/v1/", "/users", "/api/v1/users"},
		{"root and path", "/", "webhook", "/webhook"},
		{"path and root", "api", "/", "api/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SingleJoiningSlash(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("SingleJoiningSlash(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantURL string
		scheme  string
		host    string
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "host:port shorthand",
			input:   "localhost:3000",
			wantURL: "http://localhost:3000",
			scheme:  "http",
			host:    "localhost:3000",
		},
		{
			name:    "full http URL",
			input:   "http://example.com/path",
			wantURL: "http://example.com/path",
			scheme:  "http",
			host:    "example.com",
		},
		{
			name:    "https URL",
			input:   "https://secure.example.com/webhook",
			wantURL: "https://secure.example.com/webhook",
			scheme:  "https",
			host:    "secure.example.com",
		},
		{
			name:    "with port",
			input:   "http://localhost:8080/api",
			wantURL: "http://localhost:8080/api",
			scheme:  "http",
			host:    "localhost:8080",
		},
		{
			name:    "with query",
			input:   "http://example.com/webhook?token=abc123",
			wantURL: "http://example.com/webhook?token=abc123",
			scheme:  "http",
			host:    "example.com",
		},
		{
			name:    "with whitespace",
			input:   "  http://example.com  ",
			wantURL: "http://example.com",
			scheme:  "http",
			host:    "example.com",
		},
		{
			name:    "just host no scheme",
			input:   "example.com",
			wantErr: true,
		},
		{
			name:    "invalid URL",
			input:   "://invalid",
			wantErr: true,
		},
		{
			name:    "missing host",
			input:   "http://",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseURL(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.String() != tt.wantURL {
				t.Errorf("ParseURL(%q) = %q, want %q", tt.input, got.String(), tt.wantURL)
			}
			if got.Scheme != tt.scheme {
				t.Errorf("ParseURL(%q) Scheme = %q, want %q", tt.input, got.Scheme, tt.scheme)
			}
			if got.Host != tt.host {
				t.Errorf("ParseURL(%q) Host = %q, want %q", tt.input, got.Host, tt.host)
			}
		})
	}
}

func TestParseURL_IPAddresses(t *testing.T) {
	tests := []struct {
		input  string
		scheme string
		host   string
	}{
		{"http://127.0.0.1:3000", "http", "127.0.0.1:3000"},
		{"http://192.168.1.1:8080", "http", "192.168.1.1:8080"},
		{"http://0.0.0.0:3000", "http", "0.0.0.0:3000"},
		{"127.0.0.1:3000", "http", "127.0.0.1:3000"}, // shorthand
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseURL(tt.input)
			if err != nil {
				t.Fatalf("ParseURL(%q) error: %v", tt.input, err)
			}
			if got.Scheme != tt.scheme {
				t.Errorf("Scheme = %q, want %q", got.Scheme, tt.scheme)
			}
			if got.Host != tt.host {
				t.Errorf("Host = %q, want %q", got.Host, tt.host)
			}
		})
	}
}

func TestMustParseURL(t *testing.T) {
	t.Run("valid URL", func(t *testing.T) {
		u := MustParseURL("http://example.com/path")
		if u.Scheme != "http" {
			t.Errorf("Scheme = %q, want http", u.Scheme)
		}
		if u.Host != "example.com" {
			t.Errorf("Host = %q, want example.com", u.Host)
		}
		if u.Path != "/path" {
			t.Errorf("Path = %q, want /path", u.Path)
		}
	})

	t.Run("invalid URL panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid URL")
			}
		}()
		MustParseURL("")
	})
}

func TestParseURL_ErrorMessages(t *testing.T) {
	tests := []struct {
		input       string
		wantErrMsg  string
	}{
		{"", "empty URL"},
		{"   ", "empty URL"},
		{"not-a-url", "invalid URL"},
		{"ftp://", "invalid URL"}, // no host
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := ParseURL(tt.input)
			if err == nil {
				t.Fatalf("Expected error for %q", tt.input)
			}
			if err.Error() != tt.wantErrMsg && !contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("Error message = %q, want containing %q", err.Error(), tt.wantErrMsg)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(substr) <= len(s) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
