package assets

import (
	_ "embed" // enable go:embed directives
	"mime"
	"net/url"
	"path"
	"strings"

	"github.com/linkdata/jaws/lib/htmlio"
)

// JavascriptText is the source code for the client-side JaWS JavaScript library.
//
// It holds the embedded asset as an immutable string shared process-wide.
//
//go:embed jaws.js
var JavascriptText string

// JawsCSS is the source code for the client-side JaWS stylesheet.
//
// It holds the embedded asset as an immutable string shared process-wide.
//
//go:embed jaws.css
var JawsCSS string

// PreloadHTML returns HTML code to load the given resources efficiently.
//
// JavaScript and CSS files are emitted as script and stylesheet tags. Other
// recognized resources are emitted as preload tags. Favicon image URLs are
// returned separately.
//
// Nil URL arguments are skipped. A resource is returned as faviconURL only when
// its base name begins with "favicon" and its MIME type (resolved from the file
// extension) is image/*; a favicon whose extension has no image MIME mapping is
// emitted as an ordinary preload link instead. If more than one resource
// qualifies as a favicon, the last one wins and earlier favicon URLs are
// discarded rather than emitted as preload links.
func PreloadHTML(urls ...*url.URL) (htmlCode, faviconURL string) {
	var jsurls, cssurls []string
	var favicontype string
	var buf []byte
	for _, u := range urls {
		if u == nil {
			continue
		}
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
