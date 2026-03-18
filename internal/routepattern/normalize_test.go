package routepattern

import "testing"

func TestHasMethodPrefix(t *testing.T) {
	for _, tc := range []struct {
		pattern string
		want    bool
	}{
		{pattern: "GET /x", want: true},
		{pattern: "POST\t/x", want: true},
		{pattern: "M-SEARCH\t/x", want: true},
		{pattern: "P0ST /x", want: true},
		{pattern: "GE/T /x", want: false},
		{pattern: "get /x", want: false},
		{pattern: "/x", want: false},
		{pattern: "", want: false},
		{pattern: " ", want: false},
	} {
		if got := hasMethodPrefix(tc.pattern); got != tc.want {
			t.Errorf("hasMethodPrefix(%q): want %v, got %v", tc.pattern, tc.want, got)
		}
	}
}

func TestIsMethodChar(t *testing.T) {
	for _, tc := range []struct {
		c    byte
		want bool
	}{
		{c: 'A', want: true},
		{c: '7', want: true},
		{c: '-', want: true},
		{c: '~', want: true},
		{c: '/', want: false},
		{c: 'a', want: false},
	} {
		if got := isMethodChar(tc.c); got != tc.want {
			t.Errorf("isMethodChar(%q): want %v, got %v", tc.c, tc.want, got)
		}
	}
}

func TestNormalizeGET(t *testing.T) {
	for _, tc := range []struct {
		pattern string
		want    string
	}{
		{pattern: "/file.js", want: "GET /file.js"},
		{pattern: "file.js", want: "GET /file.js"},
		{pattern: "", want: "GET /"},
		{pattern: "  file.js\t", want: "GET /file.js"},
		{pattern: "POST /file.js", want: "POST /file.js"},
		{pattern: "POST\t/file.js", want: "POST\t/file.js"},
	} {
		if got := NormalizeGET(tc.pattern); got != tc.want {
			t.Errorf("NormalizeGET(%q): want %q, got %q", tc.pattern, tc.want, got)
		}
	}
}
