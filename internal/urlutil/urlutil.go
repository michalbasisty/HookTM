package urlutil

import (
	"fmt"
	"net/url"
	"strings"
)

// SingleJoiningSlash joins two path segments with exactly one slash between them.
func SingleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		if a == "" {
			return "/" + b
		}
		return a + "/" + b
	default:
		if a == "" {
			if b == "" {
				return "/"
			}
			if strings.HasPrefix(b, "/") {
				return b
			}
			return "/" + b
		}
		return a + b
	}
}

// ParseURL parses a URL string, handling shorthand formats.
// It supports:
//   - Full URLs: http://example.com/path
//   - Host:port shorthand: localhost:3000 (becomes http://localhost:3000)
//   - Host only with scheme: example.com (becomes http://example.com)
//
// Returns an error if the URL is empty or invalid.
func ParseURL(s string) (*url.URL, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty URL")
	}

	// Allow host:port shorthand
	if !strings.Contains(s, "://") && strings.Contains(s, ":") {
		s = "http://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid URL: %q", s)
	}
	return u, nil
}

// MustParseURL parses a URL string and panics if it's invalid.
// This is useful for testing and for URLs that are known to be valid.
func MustParseURL(s string) *url.URL {
	u, err := ParseURL(s)
	if err != nil {
		panic(fmt.Sprintf("MustParseURL(%q): %v", s, err))
	}
	return u
}
