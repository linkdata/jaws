package core

import (
	_ "embed"
	"mime"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

// JavascriptText is the source code for the client-side JaWS Javascript library.
//
//go:embed jaws.js
var JavascriptText []byte

//go:embed jaws.css
var JawsCSS []byte

// JawsKeyAppend appends the JaWS key as a string to the buffer.
func JawsKeyAppend(b []byte, jawsKey uint64) []byte {
	if jawsKey != 0 {
		b = strconv.AppendUint(b, jawsKey, 32)
	}
	return b
}

// JawsKeyString returns the string to be used for the given JaWS key.
func JawsKeyString(jawsKey uint64) string {
	return string(JawsKeyAppend(nil, jawsKey))
}

// JawsKeyValue parses a key string (as returned JawsKeyString) into a uint64.
func JawsKeyValue(jawsKey string) uint64 {
	slashIdx := strings.IndexByte(jawsKey, '/')
	if slashIdx < 0 {
		slashIdx = len(jawsKey)
	}
	if val, err := strconv.ParseUint(jawsKey[:slashIdx], 32, 64); err == nil {
		return val
	}
	return 0
}

// PreloadHTML returns HTML code to load the given resources efficiently.
func PreloadHTML(urls ...*url.URL) (htmlcode, faviconurl string) {
	var jsurls, cssurls []string
	var favicontype string
	var buf []byte
	for _, u := range urls {
		if u != nil {
			var asattr string
			ext := strings.ToLower(filepath.Ext(u.Path))
			mimetype := mime.TypeByExtension(ext)
			if semi := strings.IndexByte(mimetype, ';'); semi > 0 {
				mimetype = mimetype[:semi]
			}
			urlstr := u.String()
			switch ext {
			case ".js":
				jsurls = append(jsurls, urlstr)
				continue
			case ".css":
				cssurls = append(cssurls, urlstr)
				continue
			default:
				if strings.HasPrefix(mimetype, "image") {
					asattr = "image"
					if strings.HasPrefix(filepath.Base(u.Path), "favicon") {
						favicontype = mimetype
						faviconurl = urlstr
						continue
					}
				} else if strings.HasPrefix(mimetype, "font") {
					asattr = "font"
				}
			}
			buf = append(buf, `<link rel="preload" href="`...)
			buf = append(buf, urlstr...)
			buf = append(buf, '"')
			if asattr != "" {
				buf = append(buf, ` as="`...)
				buf = append(buf, asattr...)
				buf = append(buf, '"')
			}
			if mimetype != "" {
				buf = append(buf, ` type="`...)
				buf = append(buf, mimetype...)
				buf = append(buf, '"')
			}
			buf = append(buf, ">\n"...)
		}
	}
	for _, urlstr := range cssurls {
		buf = append(buf, `<link rel="stylesheet" href="`...)
		buf = append(buf, urlstr...)
		buf = append(buf, "\">\n"...)
	}
	if faviconurl != "" {
		buf = append(buf, `<link rel="icon" type="`...)
		buf = append(buf, favicontype...)
		buf = append(buf, `" href="`...)
		buf = append(buf, faviconurl...)
		buf = append(buf, "\">\n"...)
	}
	for _, urlstr := range jsurls {
		buf = append(buf, `<script defer src="`...)
		buf = append(buf, []byte(urlstr)...)
		buf = append(buf, "\"></script>\n"...)
	}
	htmlcode = string(buf)
	return
}
