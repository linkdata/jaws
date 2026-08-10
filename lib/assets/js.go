package assets

import (
	_ "embed" // enable go:embed directives
	"net/url"
	"path"
	"strings"

	"github.com/linkdata/jaws/internal/headresource"
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
// Files with .js and .css extensions are emitted as script and stylesheet tags.
// Image and font resources are emitted as preloads for their respective
// destinations; other resources use the fetch destination. Font and fetch
// preloads use anonymous CORS. Favicon image URLs are returned separately. A
// recognized MIME type is included in the preload's type attribute.
//
// Nil URL arguments are skipped. A resource is returned as faviconURL only when
// its base name begins with "favicon" and its MIME type (resolved from the file
// extension) is image/*. A .js or .css resource is always emitted as a script or
// stylesheet and is never treated as a favicon, regardless of its base name. If
// more than one resource qualifies as a favicon, the last one wins and earlier
// favicon URLs are discarded rather than emitted as preload links.
func PreloadHTML(urls ...*url.URL) (htmlCode, faviconURL string) {
	var jsurls, cssurls []string
	var favicontype string
	var buf []byte
	for _, u := range urls {
		destination, mimetype := headresource.Classify(u)
		if destination == headresource.DestinationNone {
			continue
		}
		var asattr string
		var crossorigin bool
		urlstr := u.String()
		switch destination {
		case headresource.DestinationScript:
			jsurls = append(jsurls, urlstr)
			continue
		case headresource.DestinationStyle:
			cssurls = append(cssurls, urlstr)
			continue
		case headresource.DestinationFetch:
			asattr = "fetch"
			crossorigin = true
		case headresource.DestinationImage:
			asattr = "image"
			if strings.HasPrefix(strings.ToLower(path.Base(u.Path)), "favicon") {
				favicontype = mimetype
				faviconURL = urlstr
				continue
			}
		case headresource.DestinationFont:
			asattr = "font"
			crossorigin = true
		}
		buf = append(buf, `<link rel="preload"`...)
		buf = htmlio.AppendAttr(buf, "href", urlstr)
		if asattr != "" {
			buf = htmlio.AppendAttr(buf, "as", asattr)
		}
		if mimetype != "" {
			buf = htmlio.AppendAttr(buf, "type", mimetype)
		}
		if crossorigin {
			buf = htmlio.AppendAttr(buf, "crossorigin", "anonymous")
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
