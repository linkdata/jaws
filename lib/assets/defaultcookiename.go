package assets

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
//
// It uses the path's base name with its last file extension removed, then keeps
// only ASCII letters and digits. If no usable characters remain (including for an
// empty path), it returns "jaws", the fallback recorded in [DefaultCookieName].
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
