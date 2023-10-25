package jaws

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultCookieName holds the default JaWS cookie name.
// It will be generated from the executable name, or "jaws" if that fails.
var DefaultCookieName string

func init() {
	DefaultCookieName = "jaws"
	if s, err := os.Executable(); err == nil {
		s = filepath.Base(s)
		s = strings.TrimSuffix(s, filepath.Ext(s))
		var b []byte
		for _, ch := range s {
			if ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') || ('0' <= ch && ch <= '9') {
				b = append(b, byte(ch))
			}
		}
		if len(b) > 0 {
			DefaultCookieName = s
		}
	}
}
