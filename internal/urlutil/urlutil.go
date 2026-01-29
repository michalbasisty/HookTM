package urlutil

import "strings"

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
