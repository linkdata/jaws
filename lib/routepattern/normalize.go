package routepattern

import (
	"net/http"
	"strings"
)

func isMethodChar(c byte) bool {
	if c >= 'A' && c <= 'Z' {
		return true
	}
	if c >= '0' && c <= '9' {
		return true
	}
	switch c {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	}
	return false
}

func hasMethodPrefix(pattern string) bool {
	if i := strings.IndexAny(pattern, " \t"); i > 0 {
		method := pattern[:i]
		for _, c := range []byte(method) {
			if !isMethodChar(c) {
				return false
			}
		}
		return true
	}
	return false
}

// EnsurePrefixSlash returns s with a leading slash.
func EnsurePrefixSlash(s string) string {
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	return s
}

// NormalizeGET returns a method-aware ServeMux pattern.
//
// If pattern already has a method prefix, it is returned unchanged.
// Otherwise GET is prepended and the path is made absolute.
func NormalizeGET(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if hasMethodPrefix(pattern) {
		return pattern
	}
	return http.MethodGet + " " + EnsurePrefixSlash(pattern)
}
