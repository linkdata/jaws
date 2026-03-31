package jaws

import "github.com/linkdata/jaws/core/assets"

// DefaultCookieName holds the default JaWS cookie name.
// It will be generated from the executable name, or "jaws" if that fails.
var DefaultCookieName = assets.DefaultCookieName

func makeCookieName(exename string) (cookie string) {
	return assets.MakeCookieName(exename)
}
