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
		s = strings.Trim(filepath.Base(s), "/.\\")
		s = strings.TrimSuffix(s, filepath.Ext(s))
		if s != "" {
			DefaultCookieName = s
		}
	}
}
