package jaws

import (
	"net/url"

	"github.com/linkdata/jaws/core/assets"
)

// JavascriptText is the source code for the client-side JaWS Javascript library.
var JavascriptText = assets.JavascriptText

// JawsCSS is the built-in JaWS stylesheet.
var JawsCSS = assets.JawsCSS

// JawsKeyAppend appends the JaWS key as a string to the buffer.
func JawsKeyAppend(b []byte, jawsKey uint64) []byte {
	return assets.JawsKeyAppend(b, jawsKey)
}

// JawsKeyString returns the string to be used for the given JaWS key.
func JawsKeyString(jawsKey uint64) string {
	return assets.JawsKeyString(jawsKey)
}

// JawsKeyValue parses a key string (as returned JawsKeyString) into a uint64.
func JawsKeyValue(jawsKey string) uint64 {
	return assets.JawsKeyValue(jawsKey)
}

// PreloadHTML returns HTML code to load the given resources efficiently.
func PreloadHTML(urls ...*url.URL) (htmlcode, faviconurl string) {
	return assets.PreloadHTML(urls...)
}
