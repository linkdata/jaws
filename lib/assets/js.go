package assets

import (
	_ "embed" // enable go:embed directives
	"mime"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/linkdata/jaws/lib/htmlio"
)

// JavascriptText is the source code for the client-side JaWS JavaScript library.
//
//go:embed jaws.js
var JavascriptText []byte

// JawsCSS is the source code for the client-side JaWS stylesheet.
//
//go:embed jaws.css
var JawsCSS []byte

// JawsKeyAppend appends the JaWS key as a base-32 string to the buffer.
//
// A zero key appends nothing. The encoding pairs with [JawsKeyValue].
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

// JawsKeyValue parses a base-32 key string, as returned by [JawsKeyString], into
// a uint64. Any trailing "/..." path suffix is ignored. Returns 0 if the key
// cannot be parsed.
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
//
// JavaScript and CSS files are emitted as script and stylesheet tags. Other
// recognized resources are emitted as preload tags. Favicon image URLs are
// returned separately.
func PreloadHTML(urls ...*url.URL) (htmlCode, faviconURL string) {
	var jsurls, cssurls []string
	var favicontype string
	var buf []byte
	for _, u := range urls {
		if u != nil {
			var asattr string
			ext := strings.ToLower(path.Ext(u.Path))
			mimetype := mime.TypeByExtension(ext)
			mimetype, _, _ = strings.Cut(mimetype, ";")
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
					if strings.HasPrefix(strings.ToLower(path.Base(u.Path)), "favicon") {
						favicontype = mimetype
						faviconURL = urlstr
						continue
					}
				} else if strings.HasPrefix(mimetype, "font") {
					asattr = "font"
				}
			}
			buf = append(buf, `<link rel="preload"`...)
			buf = htmlio.AppendAttr(buf, "href", urlstr)
			if asattr != "" {
				buf = htmlio.AppendAttr(buf, "as", asattr)
			}
			if mimetype != "" {
				buf = htmlio.AppendAttr(buf, "type", mimetype)
			}
			buf = append(buf, ">\n"...)
		}
	}
	for _, urlstr := range cssurls {
		buf = append(buf, `<link rel="stylesheet"`...)
		buf = htmlio.AppendAttr(buf, "href", urlstr)
		buf = append(buf, ">\n"...)
	}
	if faviconURL != "" {
		buf = append(buf, `<link rel="icon"`...)
		buf = htmlio.AppendAttr(buf, "type", favicontype)
		buf = htmlio.AppendAttr(buf, "href", faviconURL)
		buf = append(buf, ">\n"...)
	}
	for _, urlstr := range jsurls {
		buf = append(buf, `<script defer`...)
		buf = htmlio.AppendAttr(buf, "src", urlstr)
		buf = append(buf, "></script>\n"...)
	}
	htmlCode = string(buf)
	return
}
