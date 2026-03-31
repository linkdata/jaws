package jawsdata

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultCookieName holds the default JaWS cookie name.
// It will be generated from the executable name, or "jaws" if that fails.
var DefaultCookieName string

func init() {
	exename, _ := os.Executable()
	DefaultCookieName = MakeCookieName(exename)
}

// MakeCookieName creates a cookie-safe name from an executable path.
func MakeCookieName(exename string) (cookie string) {
	cookie = "jaws"
	exename = filepath.Base(exename)
	exename = strings.TrimSuffix(exename, filepath.Ext(exename))
	var b []byte
	for _, ch := range exename {
		if ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') || ('0' <= ch && ch <= '9') {
			b = append(b, byte(ch)) //#nosec G115
		}
	}
	if len(b) > 0 {
		cookie = string(b)
	}
	return
}
